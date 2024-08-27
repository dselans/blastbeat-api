package publisher

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/superpowerdotcom/events/build/proto/go/common"
	"github.com/superpowerdotcom/events/build/proto/go/user"
	"google.golang.org/protobuf/proto"
)

func (p *Publisher) PublishUserCreatedEvent(ctx context.Context, u *user.User) error {
	if ctx == nil {
		return errors.New("context cannot be nil")
	}

	if u == nil {
		return errors.New("user cannot be nil")
	}

	userCreatedEvent := &common.Event{
		Id:              uuid.New().String(),
		Source:          CloudEventsSource,
		Type:            "user.created",
		SpecVersion:     CloudEventsSpecVersion,
		Datacontenttype: CloudEventsDataContentType,
		Subject:         u.Id,
		Time:            time.Now().UTC().UnixNano(),
		Data: &common.Event_UserCreated{
			UserCreated: &user.Created{
				User: u,
			},
		},
	}

	data, err := proto.Marshal(userCreatedEvent)
	if err != nil {
		return errors.Wrap(err, "failed to marshal user.created event")
	}

	if err := p.Publish(ctx, data, "user.created"); err != nil {
		return errors.Wrap(err, "failed to publish user.created event")
	}

	return nil
}
