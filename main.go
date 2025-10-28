package main

import (
	"log"
	"net/http"

	"github.com/pkg/errors"

	"github.com/dselans/blastbeat-api/api"
	"github.com/dselans/blastbeat-api/config"
	"github.com/dselans/blastbeat-api/deps"
)

var (
	version = "v0.0.0"
)

func main() {
	cfg := config.New(version)
	if err := cfg.Validate(); err != nil {
		log.Fatalf("unable to validate config: %s", err)
	}

	d, err := deps.New(cfg)
	if err != nil {
		log.Fatalf("Could not setup dependencies: %s", err)
	}

	// Create API server
	a, err := api.New(cfg, d, version)
	if err != nil {
		log.Fatalf("unable to create API instance: %s", err)
	}

	// Run API server in a goroutine so that we can allow signal listener to
	// block the main thread so it can orchestrate graceful shutdown.
	go func() {
		if err := a.Run(); err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				// Graceful API server shutdown
				return
			}

			log.Fatalf("API server run() failed: %s", err)
		}
	}()
}
