package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

// config is the connector's own configuration (NOT the server's config.yaml).
// server_url is the Justmart server's /api base; token must match the server's
// connector.token; default_printer is used when a job names no printer.
type config struct {
	ServerURL      string `yaml:"server_url"`
	Token          string `yaml:"token"`
	DefaultPrinter string `yaml:"default_printer"`
}

// loadConfig reads the connector config file (default ./config.yaml, override
// with JUSTMART_CONNECTOR_CONFIG) and applies env overrides. A missing file is
// fine — env + defaults fill in.
func loadConfig() (config, error) {
	path := os.Getenv("JUSTMART_CONNECTOR_CONFIG")
	if path == "" {
		path = "config.yaml"
	}
	var c config
	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, &c); err != nil {
			return config{}, err
		}
	}
	if v := os.Getenv("JUSTMART_CONNECTOR_SERVER_URL"); v != "" {
		c.ServerURL = v
	}
	if v := os.Getenv("JUSTMART_CONNECTOR_TOKEN"); v != "" {
		c.Token = v
	}
	if v := os.Getenv("JUSTMART_CONNECTOR_DEFAULT_PRINTER"); v != "" {
		c.DefaultPrinter = v
	}
	if c.ServerURL == "" {
		c.ServerURL = "http://localhost:8080/api"
	}
	return c, nil
}
