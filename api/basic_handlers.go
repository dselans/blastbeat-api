package api

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"
)

func (a *API) healthCheckHandler(wr http.ResponseWriter, r *http.Request) {
	status := http.StatusOK
	body := "ok"

	if a.deps.Health.Failed() {
		status = http.StatusInternalServerError
		body = "failed"
	}

	wr.WriteHeader(status)
	wr.Write([]byte(body))
}

func (a *API) versionHandler(rw http.ResponseWriter, r *http.Request) {
	logger := a.log.With(zap.String("method", "versionHandler"))
	logger.Info("handling /version request", zap.String("remoteAddr", r.RemoteAddr))

	rw.Header().Set("Content-Type", "application/json; charset=UTF-8")
	rw.WriteHeader(http.StatusOK)

	response := &ResponseJSON{Status: http.StatusOK, Message: "your_org/go-svc-template " + a.version}

	if err := json.NewEncoder(rw).Encode(response); err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
	}
}
