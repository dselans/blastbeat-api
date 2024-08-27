package validate

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/superpowerdotcom/events/build/proto/go/common"
	"github.com/superpowerdotcom/events/build/proto/go/user"
)

func Event(event *common.Event) error {
	if event == nil {
		return errors.New("event cannot be nil")
	}

	if event.Id == "" {
		return errors.New("event id cannot be empty")
	}

	if event.Datacontenttype == "" {
		return errors.New("event data content type cannot be empty")
	}

	if event.Source == "" {
		return errors.New("event source cannot be empty")
	}

	if event.Type == "" {
		return errors.New("event type cannot be empty")
	}

	if event.Time == 0 {
		return errors.New("event time cannot be zero")
	}

	if event.SpecVersion == "" {
		return errors.New("event spec version cannot be empty")
	}

	return nil
}

func User(userEntry *user.User) error {
	if userEntry == nil {
		return fmt.Errorf("user entry cannot be nil")
	}

	if userEntry.Id == "" {
		return fmt.Errorf("user id cannot be empty")
	}

	if userEntry.Email == "" {
		return fmt.Errorf("user email cannot be empty")
	}

	if userEntry.FirstName == "" {
		return fmt.Errorf("user first name cannot be empty")
	}

	if userEntry.LastName == "" {
		return fmt.Errorf("user last name cannot be empty")
	}

	if userEntry.Gender == user.Gender_GENDER_UNSPECIFIED {
		return fmt.Errorf("user gender cannot be unspecified")
	}

	if err := Address(userEntry.Address); err != nil {
		return errors.Wrap(err, "unable to validate user address")
	}

	return nil
}

func Address(address *user.Address) error {
	if address == nil {
		return fmt.Errorf("address cannot be nil")
	}

	if len(address.Line) < 1 {
		return fmt.Errorf("address line must have at least one entry")
	}

	if address.City == "" {
		return fmt.Errorf("address city cannot be empty")
	}

	if address.Country == "" {
		return fmt.Errorf("address country cannot be empty")
	}

	if address.State == user.AddressState_ADDRESS_STATE_UNSPECIFIED {
		return fmt.Errorf("address state cannot be unspecified")
	}

	if address.PostalCode == "" {
		return fmt.Errorf("address postal code cannot be empty")
	}

	return nil
}

func UserCreatedEvent(event *user.Created) error {
	if event == nil {
		return errors.New("user created event cannot be nil")
	}

	if err := User(event.User); err != nil {
		return errors.Wrap(err, "failed to validate user")
	}

	return nil
}

func UserUpdatedEvent(event *user.Updated) error {
	if event == nil {
		return errors.New("user updated event cannot be nil")
	}

	if err := User(event.User); err != nil {
		return errors.Wrap(err, "failed to validate user")
	}

	return nil
}

func UserDeletedEvent(event *user.Deleted) error {
	if event == nil {
		return errors.New("user deleted event cannot be nil")
	}

	// Because this is a delete, we only care about the user.id and can skip
	// more in-depth validation.
	if event.User == nil {
		return errors.New("user deleted event user id cannot be nil")
	}

	if event.User.Id == "" {
		return errors.New("user deleted event user id cannot be empty")
	}

	return nil
}
