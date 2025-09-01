package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/superpowerdotcom/go-svc-template/api"
	"github.com/superpowerdotcom/go-svc-template/config"
	"github.com/superpowerdotcom/go-svc-template/deps"
)

const (
	GracefulShutdownTimeout = 10 * time.Second
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

	handleShutdown(d)
}

func handleShutdown(d *deps.Dependencies) {
	llog := d.Log.With(zap.String("method", "handleShutdown"), zap.String("pkg", "main"))
	llog.Debug("Listening for shutdown")

	componentWaitGroup := &sync.WaitGroup{}
	componentDoneCh := make(chan struct{})

	// Launch a goroutine for each component that has a graceful shutdown. Once
	// component has shutdown, it will send signal on componentDoneCh. This will
	// decrease wait group counter. Once all components have shutdown,
	// componentWaitGroup.Wait() will unblock, allowing 2nd goroutine to write
	// to componentDoneCh which will be caught by the select().
	go func() {
		componentWaitGroup.Add(1) // Add(1) for every component
		<-d.PublisherShutdownDoneCh
		componentWaitGroup.Done()
	}()

	// ^ If you have additional components that have a graceful shutdown, you
	// will want to launch a separate goroutine here, same as for publisher.

	// Goroutine listening for all components to have completed shutdown
	go func() {
		componentWaitGroup.Wait()
		componentDoneCh <- struct{}{}
	}()

	// Detect ctrl-c and gracefully shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, os.Kill)

	// Block waiting for signal
	sig := <-c
	llog.Debug("Received system call", zap.Any("signal", sig))
	llog.Debug("Shutting down all components...")

	// Send termination signal to all components
	d.ShutdownCancel()

	select {
	case <-componentDoneCh:
		llog.Debug("Graceful shutdown complete")
		os.Exit(0)
	case <-time.After(GracefulShutdownTimeout):
		llog.Debug("Graceful shutdown timed out", zap.String("timeout", GracefulShutdownTimeout.String()))
		os.Exit(1)
	}
}
