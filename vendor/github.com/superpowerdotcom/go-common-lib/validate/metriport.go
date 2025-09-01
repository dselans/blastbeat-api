package validate

import (
	"github.com/pkg/errors"

	"github.com/superpowerdotcom/events/build/proto/go/common"
	"github.com/superpowerdotcom/events/build/proto/go/metriport"
)

var (
	ErrNilMetriportPatient     = errors.New("patient cannot be nil")
	ErrEmptyMetriportPatientID = errors.New("patient id cannot be empty")
	ErrEmptyMetriportFirstName = errors.New("patient first name cannot be empty")
	ErrEmptyMetriportLastName  = errors.New("patient last name cannot be empty")

	ErrNilMetriportPatientCreatedEvent      = errors.New("event.GetMetriportPatientCreated() cannot be nil")
	ErrNilCommandRefreshPatientRecordsEvent = errors.New("event.GetCommandRefreshPatientRecords() cannot be nil")
	ErrEmptyDatabaseUserID                  = errors.New("database user id cannot be empty")
	ErrEmptyMedplumPatientID                = errors.New("medplum patient id cannot be empty")
)

func MetriportPatientCreatedEvent(event *common.Event) error {
	if event == nil {
		return ErrNilEvent
	}

	if event.GetMetriportPatientCreated() == nil {
		return ErrNilMetriportPatientCreatedEvent
	}

	return MetriportPatient(event.GetMetriportPatientCreated().Patient)
}

func MetriportPatient(patient *metriport.Patient) error {
	if patient == nil {
		return ErrNilMetriportPatient
	}

	if patient.Id == "" {
		return ErrEmptyMetriportPatientID
	}

	if patient.FirstName == "" {
		return ErrEmptyMetriportFirstName
	}

	if patient.LastName == "" {
		return ErrEmptyMetriportLastName
	}

	// Should check other fields but this is a good start

	return nil
}

func CommandRefreshPatientRecords(event *common.Event) error {
	if event == nil {
		return ErrNilEvent
	}

	if event.GetCommandRefreshPatientRecords() == nil {
		return ErrNilCommandRefreshPatientRecordsEvent
	}

	cmd := event.GetCommandRefreshPatientRecords()

	if cmd.MetriportPatientId == "" {
		return ErrEmptyMetriportPatientID
	}

	if cmd.DatabaseUserId == "" {
		return ErrEmptyDatabaseUserID
	}

	if cmd.MedplumPatientId == "" {
		return ErrEmptyMedplumPatientID
	}

	return nil
}
