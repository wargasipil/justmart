//go:build wireinject

// The build tag makes sure this stub is not built into the real binary — it is
// the input Wire reads to generate wire_gen.go. Regenerate with `make wire`
// (or `go tool wire ./cmd/server`) after changing providers.go.
package main

import (
	"github.com/google/wire"
)

// initApp builds the whole server object graph: config -> DB (migrated) ->
// auth primitives -> every ConnectRPC service -> the HTTP stack. The returned
// cleanup closes the DB pool.
func initApp(path configPath, version buildVersion) (*App, func(), error) {
	wire.Build(appSet)
	return nil, nil, nil
}
