package validate

import (
	"time"

	"github.com/pkg/errors"

	"github.com/superpowerdotcom/events/build/proto/go/common"
	"github.com/superpowerdotcom/events/build/proto/go/merch"
	"github.com/superpowerdotcom/events/build/proto/go/user"
)

var (
	// User Errors
	ErrNilUser                = errors.New("user entry cannot be nil")
	ErrEmptyUserID            = errors.New("user id cannot be empty")
	ErrEmptyUserEmail         = errors.New("user email cannot be empty")
	ErrEmptyUserFirstName     = errors.New("user first name cannot be empty")
	ErrEmptyUserLastName      = errors.New("user last name cannot be empty")
	ErrUnspecifiedUserGender  = errors.New("user gender cannot be unspecified")
	ErrEmptyUserPhone         = errors.New("user phone cannot be empty")
	ErrEmptyUserDateOfBirth   = errors.New("user date of birth cannot be empty")
	ErrInvalidUserDateOfBirth = errors.New("user date of birth must be in the format of YYYY-MM-DD")
	ErrInvalidUserAddress     = errors.New("unable to validate user address")

	// Address Errors
	ErrNilAddress              = errors.New("address cannot be nil")
	ErrEmptyAddressLine        = errors.New("address line must have at least one entry")
	ErrEmptyAddressCity        = errors.New("address city cannot be empty")
	ErrEmptyAddressCountry     = errors.New("address country cannot be empty")
	ErrUnspecifiedAddressState = errors.New("address state cannot be unspecified")
	ErrEmptyAddressPostalCode  = errors.New("address postal code cannot be empty")

	// Event Errors
	ErrNilUserCreatedEvent = errors.New("event.GetUserCreated() cannot be nil")
	ErrNilUserUpdatedEvent = errors.New("event.GetUserUpdated() cannot be nil")
	ErrNilUserDeletedEvent = errors.New("event.GetUserDeleted() cannot be nil")

	ErrUserValidationFailed = errors.New("failed to validate user")

	// Card Errors
	ErrNilCard = errors.New("card entry cannot be nil")
)

func User(u *user.User) error {
	if u == nil {
		return ErrNilUser
	}

	if u.Id == "" {
		return ErrEmptyUserID
	}

	if u.Email == "" {
		return ErrEmptyUserEmail
	}

	if u.FirstName == "" {
		return ErrEmptyUserFirstName
	}

	if u.LastName == "" {
		return ErrEmptyUserLastName
	}

	if u.Gender == user.Gender_GENDER_UNSPECIFIED {
		return ErrUnspecifiedUserGender
	}

	if u.Phone == "" {
		return ErrEmptyUserPhone
	}

	if u.DateOfBirth == "" {
		return ErrEmptyUserDateOfBirth
	}

	if _, err := time.Parse("2006-01-02", u.DateOfBirth); err != nil {
		return ErrInvalidUserDateOfBirth
	}

	return nil
}

func Address(a *user.Address) error {
	if a == nil {
		return ErrNilAddress
	}

	if len(a.Line) < 1 {
		return ErrEmptyAddressLine
	}

	if a.City == "" {
		return ErrEmptyAddressCity
	}

	if a.Country == "" {
		return ErrEmptyAddressCountry
	}

	if a.State == user.AddressState_ADDRESS_STATE_UNSPECIFIED {
		return ErrUnspecifiedAddressState
	}

	if a.PostalCode == "" {
		return ErrEmptyAddressPostalCode
	}

	return nil
}

func UserCreatedEvent(event *common.Event) error {
	if event == nil {
		return ErrNilEvent
	}

	if event.GetUserCreated() == nil {
		return ErrNilUserCreatedEvent
	}

	if err := User(event.GetUserCreated().GetUser()); err != nil {
		return errors.Wrap(err, ErrUserValidationFailed.Error())
	}

	return nil
}

func UserUpdatedEvent(event *common.Event) error {
	if event == nil {
		return ErrNilEvent
	}

	if event.GetUserUpdated() == nil {
		return ErrNilUserUpdatedEvent
	}

	u := event.GetUserUpdated().User

	if err := User(u); err != nil {
		return errors.Wrap(err, ErrUserValidationFailed.Error())
	}

	if err := Address(u.Address); err != nil {
		return errors.Wrap(err, ErrInvalidUserAddress.Error())
	}

	return nil
}

func UserDeletedEvent(event *common.Event) error {
	if event == nil {
		return ErrNilEvent
	}

	if event.GetUserDeleted() == nil {
		return ErrNilUserDeletedEvent
	}

	if err := User(event.GetUserDeleted().User); err != nil {
		return errors.Wrap(err, ErrUserValidationFailed.Error())
	}

	return nil
}

func Card(card *merch.Card) error {
	if card == nil {
		return ErrNilCard
	}

	if err := User(card.User); err != nil {
		return ErrUserValidationFailed
	}

	return nil
}
