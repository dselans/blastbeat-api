package api

import (
	"fmt"
	"net/http"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
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

// This is an example handler that will consume once from the queue that is
// bound to the exchange with the routing key "example.publish".
func (a *API) rabbitConsumeHandler(wr http.ResponseWriter, r *http.Request) {
	msg, err := a.consumeOnce(ExampleRoutingKey, 5*time.Second)
	if err != nil {
		a.log.Error("failed to consume from rabbitmq", zap.Error(err))

		WriteJSON(wr, ResponseJSON{
			Status:  http.StatusInternalServerError,
			Message: "failed to consume from rabbitmq: " + err.Error(),
		}, http.StatusInternalServerError)

		return
	}

	// Unmarshal/deserialize/decode the message from binary -> protobuf
	event := &common.Event{}

	if err := proto.Unmarshal(msg.Body, event); err != nil {
		a.log.Error("failed to unmarshal event", zap.Error(err))

		WriteJSON(wr, ResponseJSON{
			Status:  http.StatusInternalServerError,
			Message: "failed to unmarshal event",
		}, http.StatusInternalServerError)

		return
	}

	// Return success to user
	WriteJSON(wr, map[string]interface{}{
		"status":  http.StatusOK,
		"message": "consumed example message from rabbitmq",
		"event":   event,
	}, http.StatusOK)
}

func (a *API) consumeOnce(routingKey string, timeout time.Duration) (*amqp.Delivery, error) {
	conn, err := amqp.Dial(a.deps.Config.ProcessorRabbitURL[0])
	if err != nil {
		a.log.Error("failed to connect to rabbitmq", zap.Error(err))
		return nil, fmt.Errorf("failed to connect to rabbitmq: %w", err)
	}

	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		a.log.Error("failed to create channel", zap.Error(err))
		return nil, fmt.Errorf("failed to create channel: %w", err)
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"",    // name
		false, // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		a.log.Error("failed to declare queue", zap.Error(err))
		return nil, fmt.Errorf("failed to declare queue: %w", err)
	}

	if err := ch.QueueBind(q.Name, routingKey, a.deps.Config.ProcessorRabbitExchangeName, false, nil); err != nil {
		a.log.Error("failed to bind queue", zap.Error(err))
		return nil, fmt.Errorf("failed to bind queue: %w", err)
	}

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)

	if err != nil {
		a.log.Error("failed to consume from queue", zap.Error(err))
		return nil, fmt.Errorf("failed to consume from queue: %w", err)
	}

	var m amqp.Delivery

	// Try to consume a message for given timeout
	select {
	case m = <-msgs:
	case <-time.After(timeout):
		return nil, fmt.Errorf("timed out after %s", timeout)
	}

	// Manually acknowledge the message
	if err := m.Ack(false); err != nil {
		a.log.Error("failed to acknowledge message", zap.Error(err))
		return nil, fmt.Errorf("failed to acknowledge message: %w", err)
	}

	return &m, nil
}
