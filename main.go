package main

import (
	"log"

	"github.com/your_org/go-svc-template/api"
	"github.com/your_org/go-svc-template/config"
	"github.com/your_org/go-svc-template/deps"
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

	// Start rabbit consumer
	if err := d.ProcessorService.StartConsumers(); err != nil {
		log.Fatalf("Unable to start proc consumers")
	}

	// Create and run the API server (will block until shutdown)
	a, err := api.New(cfg, d, version)
	if err != nil {
		log.Fatalf("unable to create API instance: %s", err)
	}

	if err := a.Run(); err != nil {
		log.Fatalf("Run() failed: %s", err)
	}
}
