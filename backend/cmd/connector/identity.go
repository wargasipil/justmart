package main

import (
	"encoding/json"
	"os"

	"github.com/google/uuid"
)

// identity is the connector's stable device id + display name, persisted to
// connector-identity.json (in the working dir) so the server recognizes the
// same device across restarts. Do not delete the file.
type identity struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

const identityFile = "connector-identity.json"

// loadOrCreateIdentity reads connector-identity.json, or mints a new identity
// (uuid + hostname) and writes it on first run.
func loadOrCreateIdentity() (identity, error) {
	if data, err := os.ReadFile(identityFile); err == nil {
		var id identity
		if json.Unmarshal(data, &id) == nil && id.ID != "" {
			return id, nil
		}
	}
	host, _ := os.Hostname()
	if host == "" {
		host = "connector"
	}
	id := identity{ID: uuid.NewString(), Name: host}
	data, err := json.MarshalIndent(id, "", "  ")
	if err != nil {
		return identity{}, err
	}
	if err := os.WriteFile(identityFile, data, 0o600); err != nil {
		return identity{}, err
	}
	return id, nil
}
