package main

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/net/http2"

	connectorifacev1 "github.com/justmart/backend/gen/connector_iface/v1"
	"github.com/justmart/backend/gen/connector_iface/v1/connectorifacev1connect"
)

// h2cClient returns an http.Client that speaks HTTP/2 over cleartext (h2c) so
// server-streaming works against Justmart's plain-HTTP LAN endpoint (no TLS).
func h2cClient() *http.Client {
	return &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, network, addr)
			},
		},
	}
}

// run dials the server and pumps print jobs forever, reconnecting with capped
// exponential backoff. It only returns on a non-recoverable condition (it
// doesn't, in practice — the loop is infinite).
func run(cfg config, id identity) error {
	client := connectorifacev1connect.NewConnectorServiceClient(h2cClient(), cfg.ServerURL)
	backoff := time.Second
	for {
		start := time.Now()
		if err := connectOnce(client, cfg, id); err != nil {
			slog.Warn("connector stream ended", "error", err, "retry_in", backoff.String())
		}
		// A connection that stayed up a while → reset backoff (it was healthy).
		if time.Since(start) > 10*time.Second {
			backoff = time.Second
		}
		time.Sleep(backoff)
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

// connectOnce opens one stream, registers, and prints jobs until it closes.
func connectOnce(client connectorifacev1connect.ConnectorServiceClient, cfg config, id identity) error {
	printers, err := readPrinterNames()
	if err != nil {
		slog.Warn("could not list printers", "error", err)
	}
	stream, err := client.Connect(context.Background(), connect.NewRequest(&connectorifacev1.ConnectRequest{
		DeviceId:     id.ID,
		DeviceName:   id.Name,
		PrinterNames: printers,
	}))
	if err != nil {
		return err
	}
	for stream.Receive() {
		switch e := stream.Msg().Event.(type) {
		case *connectorifacev1.ServerEvent_Registered:
			slog.Info("registered with server", "device_id", e.Registered.DeviceId, "printers", printers)
		case *connectorifacev1.ServerEvent_PrintJob:
			job := e.PrintJob
			name := job.PrinterName
			if name == "" {
				name = cfg.DefaultPrinter
			}
			if perr := printToSpooler(name, job.Payload, job.JobId); perr != nil {
				slog.Error("print failed", "job_id", job.JobId, "printer", name, "error", perr)
			} else {
				slog.Info("printed", "job_id", job.JobId, "printer", name, "bytes", len(job.Payload))
			}
		}
	}
	return stream.Err()
}
