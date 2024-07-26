package api

import (
	"net/http"
	"time"

	uuid "github.com/satori/go.uuid"
	"github.com/superpowerdotcom/events/codegen/protos/go/metriport"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/superpowerdotcom/events/codegen/protos/go/common"
)

const (
	ExampleRoutingKey = "example.go-svc-template"
)

func (a *API) rabbitPublishHandler(rw http.ResponseWriter, r *http.Request) {
	// Create an event
	event := &common.Event{
		Id:              uuid.NewV4().String(),
		Source:          "go-svc-template",
		Type:            "metriport.PatientRecordRequest",
		SpecVersion:     "1.0",
		Datacontenttype: "application/protobuf",
		Subject:         "n/a", // optional
		Time:            time.Now().UTC().UnixNano(),
		Data: &common.Event_PatientRecordRequest{
			PatientRecordRequest: &metriport.PatientRecordRequest{
				PatientId: "abc123",
			},
		},
	}

	// Marshal/serialize/encode event from protobuf -> binary
	data, err := proto.Marshal(event)
	if err != nil {
		a.log.Error("failed to marshal event", zap.Error(err))

		WriteJSON(rw, ResponseJSON{
			Status:  http.StatusInternalServerError,
			Message: "failed to marshal event",
		}, http.StatusInternalServerError)

		return
	}

	// Publish the marshalled/serialized/encoded event to rabbit
	if err := a.deps.PublisherService.Publish(r.Context(), data, ExampleRoutingKey); err != nil {
		// Log error
		a.log.Error("failed to publish message message to rabbitmq", zap.Error(err))

		// Return error to user
		WriteJSON(rw, ResponseJSON{
			Status:  http.StatusInternalServerError,
			Message: "failed to publish example message to rabbitmq",
		}, http.StatusInternalServerError)

		return
	}

	// Return success to user
	WriteJSON(rw, ResponseJSON{
		Status:  http.StatusOK,
		Message: "published example message to rabbitmq",
	}, http.StatusOK)
}
