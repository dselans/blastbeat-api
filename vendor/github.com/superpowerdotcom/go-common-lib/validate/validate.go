package validate

import (
	"strings"

	"github.com/pkg/errors"

	"github.com/superpowerdotcom/events/build/proto/go/common"
)

var (
	ErrNilEvent             = errors.New("event cannot be nil")
	ErrEmptyEventID         = errors.New("event id cannot be empty")
	ErrEmptyDataContentType = errors.New("event data content type cannot be empty")
	ErrEmptySource          = errors.New("event source cannot be empty")
	ErrEmptyType            = errors.New("event type cannot be empty")
	ErrZeroTime             = errors.New("event time cannot be zero")
	ErrEmptySpecVersion     = errors.New("event spec version cannot be empty")
)

func Event(event *common.Event) error {
	if event == nil {
		return ErrNilEvent
	}

	if event.Id == "" {
		return ErrEmptyEventID
	}

	if event.Datacontenttype == "" {
		return ErrEmptyDataContentType
	}

	if event.Source == "" {
		return ErrEmptySource
	}

	if event.Type == "" {
		return ErrEmptyType
	}

	if event.Time == 0 {
		return ErrZeroTime
	}

	if event.SpecVersion == "" {
		return ErrEmptySpecVersion
	}

	return nil
}

func IsProbablyEmail(email string) bool {
	at := strings.Index(email, "@")
	dot := strings.LastIndex(email, ".")

	// Check if "@" is present, not at the start or end, and there's a "." after it
	return at > 0 && dot > at+1 && dot < len(email)-1
}
