package api

import (
	"encoding/json"
	"log"
	"net/http"
	_ "net/http/pprof"

	"github.com/newrelic/go-agent/v3/integrations/nrhttprouter"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/your_org/go-svc-template/clog"
	"github.com/your_org/go-svc-template/config"
	"github.com/your_org/go-svc-template/deps"
)

type API struct {
	config  *config.Config
	deps    *deps.Dependencies
	log     clog.ICustomLog
	version string
}

type ResponseJSON struct {
	Status  int               `json:"status"`
	Message string            `json:"message"`
	Values  map[string]string `json:"values,omitempty"`
	Errors  string            `json:"errors,omitempty"`
}

func New(cfg *config.Config, d *deps.Dependencies, version string) (*API, error) {
	if cfg == nil {
		return nil, errors.New("cfg cannot be nil")
	}

	if d == nil {
		return nil, errors.New("deps cannot be nil")
	}

	return &API{
		config:  cfg,
		deps:    d,
		version: version,
		log:     d.Log.With(zap.String("pkg", "api")),
	}, nil
}

func (a *API) Run() error {
	logger := a.log.With(zap.String("method", "Run"))

	router := nrhttprouter.New(a.deps.NewRelicApp)

	router.HandlerFunc("GET", "/health-check", a.healthCheckHandler)
	router.HandlerFunc("GET", "/version", a.versionHandler)

	// Maybe enable profiling
	if a.config.EnablePprof {
		router.Handler(http.MethodGet, "/debug/pprof/*item", http.DefaultServeMux)
	}

	logger.Info("API server running", zap.String("listenAddress", a.config.APIListenAddress))

	return http.ListenAndServe(a.config.APIListenAddress, router)
}

// WriteJSON is a helper function for writing JSON responses
func WriteJSON(rw http.ResponseWriter, payload interface{}, status int) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("ERROR: unable to marshal JSON during WriteJSON "+
			"(payload: '%s'; status: '%d'): %s\n", payload, status, err)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)

	if _, err := rw.Write(data); err != nil {
		log.Printf("ERROR: unable to write resp in WriteJSON: %s\n", err)
		return
	}
}
