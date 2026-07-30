package cloudflare_tunnel

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"sync"
)

func downloadBinary(filepath string) error {
	var err error
	uri := "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-windows-amd64.exe"

	resp, err := http.Get(uri)
	if err != nil {
		slog.Error("failed download cloudflared",
			"err", err.Error(),
		)
		return err
	}

	defer resp.Body.Close()

	// Check server response
	if resp.StatusCode != http.StatusOK {
		slog.Error("failed download cloudflared",
			"status", resp.StatusCode,
		)
		return err
	}

	// Create the output file
	out, err := os.Create(filepath)
	if err != nil {
		slog.Error("failed create ./cloudflared.exe",
			"err", err.Error(),
		)
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		slog.Error("failed writing ./cloudflared.exe",
			"err", err.Error(),
		)
		return err
	}

	return nil

}

func RunCloudflareTunnel(ctx context.Context, token string) error {
	var err error
	binpath := "./cloudflared.exe"

	// Verify the binary exists before trying to run it
	var cloudflaredPath string
	cloudflaredPath, err = exec.LookPath(binpath)
	if err != nil {
		slog.Warn("cloudflared.exe not found, try to downloading")
		err = downloadBinary(binpath)
		cloudflaredPath, err = exec.LookPath(binpath)
		if err != nil {
			slog.Error("path still error after downloading",
				"err", err.Error(),
			)
			return err
		}
	}

	slog.Info("using cloudflared at", "path", cloudflaredPath)
	cmd := exec.CommandContext(ctx, cloudflaredPath,
		"tunnel",
		"--no-autoupdate",
		"run",
		"--token", token,
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to attach stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to attach stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start cloudflared: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Stream stdout
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			fmt.Println("[cloudflared]", scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			slog.Error("cloudflared stdout stream ended",
				"err", err.Error(),
			)
		}
	}()

	// Stream stderr (cloudflared logs most of its output here)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			fmt.Println("[cloudflared]", scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			slog.Error("cloudflared stderr stream ended",
				"err", err.Error(),
			)
		}
	}()

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		// If we cancelled the context ourselves, treat it as a clean exit
		if ctx.Err() == context.Canceled {
			return nil
		}
		return fmt.Errorf("cloudflared exited with error: %w", err)
	}

	return nil

}
