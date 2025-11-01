package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

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

	go func() {
		if err := a.Run(); err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				return
			}
			log.Fatalf("API server run() failed: %s", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down server...")
}
