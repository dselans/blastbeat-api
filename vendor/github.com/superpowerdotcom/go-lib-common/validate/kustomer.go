package validate

import (
	"github.com/pkg/errors"

	"github.com/superpowerdotcom/events/build/proto/go/action_plan"
	"github.com/superpowerdotcom/events/build/proto/go/common"
)

var (
	ErrEmptyActionPlanID                = errors.New("action plan id cannot be empty")
	ErrZeroActionPlanPublishedTimestamp = errors.New("action plan published at unix ts utc cannot be zero")
	ErrFailedUserValidation             = errors.New("failed to validate user")
	ErrFailedEventValidation            = errors.New("failed to validate event")
	ErrNilActionPlanPublishedEvent      = errors.New("event action plan published cannot be nil")
	ErrFailedActionPlanPublishedEvent   = errors.New("failed to validate action plan published event")
)

func ActionPlan(event *action_plan.ActionPlan) error {
	if event == nil {
		return ErrNilEvent
	}

	if event.Id == "" {
		return ErrEmptyActionPlanID
	}

	if event.PublishedAtUnixTsUtc == 0 {
		return ErrZeroActionPlanPublishedTimestamp
	}

	if err := User(event.User); err != nil {
		return errors.Wrap(err, ErrFailedUserValidation.Error())
	}

	return nil
}

func ActionPlanPublishedEvent(event *common.Event) error {
	if event == nil {
		return ErrNilEvent
	}

	if err := Event(event); err != nil {
		return errors.Wrap(err, ErrFailedEventValidation.Error())
	}

	if event.GetActionPlanPublished() == nil {
		return ErrNilActionPlanPublishedEvent
	}

	if err := ActionPlan(event.GetActionPlanPublished().ActionPlan); err != nil {
		return errors.Wrap(err, ErrFailedActionPlanPublishedEvent.Error())
	}

	return nil
}
