package api

import (
	"encoding/json"
	"io"
	"net/http"

	"go.uber.org/zap"
)

const (
	RoutingKey = "events.webhook"
)

func (a *API) webhookHandler(rw http.ResponseWriter, r *http.Request) {
	llog := a.log.With(zap.String("method", "webhookHandler"))
	llog.Debug("handling POST request", zap.String("remoteAddr", r.RemoteAddr))

	// Read body
	data, err := io.ReadAll(r.Body)
	if err != nil {
		llog.Warn("failed to read body", zap.Error(err))

		WriteJSON(rw, ResponseJSON{
			Status:  http.StatusBadRequest,
			Message: "failed to read request body",
		}, http.StatusBadRequest)

		return
	}

	defer r.Body.Close()

	// Valid json? Add whatever other validations you need.
	if !json.Valid(data) {
		// Return error if given invalid JSON
		llog.Warn("invalid json", zap.String("data", string(data)))

		WriteJSON(rw, &ResponseJSON{
			Status:  http.StatusBadRequest,
			Message: "invalid json",
		}, http.StatusBadRequest)

		return
	}

	// Publish message to rabbit
	if err := a.deps.PublisherService.Publish(r.Context(), data, RoutingKey); err != nil {
		llog.Error("failed to publish message", zap.Error(err))

		WriteJSON(rw, ResponseJSON{
			Status:  http.StatusInternalServerError,
			Message: "failed to publish message",
		}, http.StatusInternalServerError)

		return
	}
}
