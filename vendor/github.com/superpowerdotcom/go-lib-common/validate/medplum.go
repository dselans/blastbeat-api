package validate

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/superpowerdotcom/events/build/proto/go/common"
	c_gp "github.com/superpowerdotcom/fhir/go/proto/google/fhir/proto/r4/core/codes_go_proto"
	dt_gp "github.com/superpowerdotcom/fhir/go/proto/google/fhir/proto/r4/core/datatypes_go_proto"
	bcr_gp "github.com/superpowerdotcom/fhir/go/proto/google/fhir/proto/r4/core/resources/bundle_and_contained_resource_go_proto"
	p_gp "github.com/superpowerdotcom/fhir/go/proto/google/fhir/proto/r4/core/resources/patient_go_proto"
)

var (
	ErrEmptyNameList              = errors.New("name must have at least one entry")
	ErrNilNameEntry               = errors.New("name entry cannot be nil")
	ErrEmptyGivenNameList         = errors.New("given name must have at least one entry")
	ErrEmptyGivenNameValue        = errors.New("given name value cannot be empty")
	ErrNilFamilyName              = errors.New("family name cannot be nil")
	ErrEmptyFamilyNameValue       = errors.New("family name value cannot be empty")
	ErrNilContainedResource       = errors.New("contained resource cannot be nil")
	ErrNilContainedResourceBundle = errors.New("contained resource bundle cannot be nil")
	ErrEmptyBundleEntry           = errors.New("bundle entry must have at least one entry")
	ErrNilPatient                 = errors.New("patient cannot be nil")
	ErrNilPatientID               = errors.New("patient id cannot be nil")
	ErrEmptyPatientIDValue        = errors.New("patient id value cannot be empty")
	ErrPatientNameValidation      = errors.New("failed to validate patient name")
	ErrNilPatientBirthDate        = errors.New("patient birthdate cannot be nil")
	ErrZeroPatientBirthDateValue  = errors.New("patient birthdate values cannot be zero")
	ErrPatientTelecomValidation   = errors.New("failed to validate patient telecom")
	ErrPatientAddressValidation   = errors.New("failed to validate patient address")
	ErrNilPatientGender           = errors.New("patient gender cannot be nil")
	ErrUnsetPatientGenderEnum     = errors.New("patient gender must be set to a non-0 enum")
	ErrPatientValidationFailed    = errors.New("failed to validate patient")
	ErrNilMedplumWebhookEvent     = errors.New("medplum webhook event cannot be nil")
	ErrMedplumCRValidationFailed  = errors.New("failed to validate contained resource")

	ErrEmptyAddressList            = errors.New("addresses must have at least one entry")
	ErrNilAddressEntry             = errors.New("address entry cannot be nil")
	ErrEmptyAddressLineList        = errors.New("address line must have at least one entry")
	ErrEmptyAddressLineValue       = errors.New("address line value cannot be empty")
	ErrNilAddressCity              = errors.New("address city cannot be nil")
	ErrEmptyAddressCityValue       = errors.New("address city value cannot be empty")
	ErrNilAddressCountry           = errors.New("address country cannot be nil")
	ErrEmptyAddressCountryValue    = errors.New("address country value cannot be empty")
	ErrNilAddressState             = errors.New("address state cannot be nil")
	ErrEmptyAddressStateValue      = errors.New("address state value cannot be empty")
	ErrNilAddressPostalCode        = errors.New("address postal code cannot be nil")
	ErrEmptyAddressPostalCodeValue = errors.New("address postal code value cannot be empty")

	ErrNilContactPoint                = errors.New("contact point cannot be nil")
	ErrNilContactPointSystem          = errors.New("contact point system cannot be nil")
	ErrInvalidContactPointSystem      = errors.New("contact point system must be EMAIL")
	ErrInvalidContactPointSystemEmail = errors.New("contact point system must be PHONE")
	ErrNilContactPointValue           = errors.New("contact point value cannot be nil")
	ErrEmptyContactPointValueValue    = errors.New("contact point value cannot be empty")
	ErrInvalidEmailAddress            = errors.New("provided email field does not appear to be an email address")

	ErrEmptyContactPointList        = errors.New("contact points must have at least one entry")
	ErrPatientEmailNotFound         = errors.New("patient email not found in telecom entries")
	ErrFailedContactPointValidation = errors.New("failed to validate contact point")
)

func MedplumName(names []*dt_gp.HumanName) error {
	if len(names) < 1 {
		return ErrEmptyNameList
	}

	// Name checks
	for _, n := range names {
		if n == nil {
			return ErrNilNameEntry
		}

		if len(n.Given) < 1 {
			return ErrEmptyGivenNameList
		}

		if n.Given[0].Value == "" {
			return ErrEmptyGivenNameValue
		}

		if n.Family == nil {
			return ErrNilFamilyName
		}

		if n.Family.Value == "" {
			return ErrEmptyFamilyNameValue
		}
	}

	return nil
}

func MedplumSearchResponse(pcr *bcr_gp.ContainedResource) error {
	if pcr == nil {
		return ErrNilContainedResource
	}

	if pcr.GetBundle() == nil {
		return ErrNilContainedResourceBundle
	}

	if len(pcr.GetBundle().Entry) < 1 {
		return ErrEmptyBundleEntry
	}

	return nil
}

func MedplumPatientCR(pcr *bcr_gp.ContainedResource, checkID bool) error {
	if pcr == nil {
		return ErrNilContainedResource
	}

	if err := MedplumPatient(pcr.GetPatient(), checkID); err != nil {
		return errors.Wrap(err, ErrPatientValidationFailed.Error())
	}

	return nil
}

// MedplumPatient validates that a patient resource contains required fields
func MedplumPatient(patient *p_gp.Patient, checkID bool) error {
	if patient == nil {
		return ErrNilPatient
	}

	if checkID {
		if patient.Id == nil {
			return ErrNilPatientID
		}

		if patient.Id.Value == "" {
			return ErrEmptyPatientIDValue
		}
	}

	if err := MedplumName(patient.Name); err != nil {
		return errors.Wrap(err, ErrPatientNameValidation.Error())
	}

	if patient.BirthDate == nil {
		return ErrNilPatientBirthDate
	}

	if patient.BirthDate.ValueUs == 0 {
		return ErrZeroPatientBirthDateValue
	}

	if err := MedplumContactPoint(patient.Telecom); err != nil {
		return errors.Wrap(err, ErrPatientTelecomValidation.Error())
	}

	if err := MedplumAddress(patient.Address); err != nil {
		return errors.Wrap(err, ErrPatientAddressValidation.Error())
	}

	if patient.Gender == nil {
		return ErrNilPatientGender
	}

	if patient.Gender.Value == 0 {
		return ErrUnsetPatientGenderEnum
	}

	// TODO: Should also validate Extension and Metadata

	return nil
}

func MedplumAddress(addresses []*dt_gp.Address) error {
	if len(addresses) < 1 {
		return ErrEmptyAddressList
	}

	for _, address := range addresses {
		if address == nil {
			return ErrNilAddressEntry
		}

		if len(address.Line) < 1 {
			return ErrEmptyAddressLineList
		}

		if address.Line[0].Value == "" {
			return ErrEmptyAddressLineValue
		}

		if address.City == nil {
			return ErrNilAddressCity
		}

		if address.City.Value == "" {
			return ErrEmptyAddressCityValue
		}

		if address.Country == nil {
			return ErrNilAddressCountry
		}

		if address.Country.Value == "" {
			return ErrEmptyAddressCountryValue
		}

		if address.State == nil {
			return ErrNilAddressState
		}

		if address.State.Value == "" {
			return ErrEmptyAddressStateValue
		}

		if address.PostalCode == nil {
			return ErrNilAddressPostalCode
		}

		if address.PostalCode.Value == "" {
			return ErrEmptyAddressPostalCodeValue
		}
	}

	return nil
}

func MedplumEmail(cp *dt_gp.ContactPoint) error {
	if cp == nil {
		return ErrNilContactPoint
	}

	if cp.System == nil {
		return ErrNilContactPointSystem
	}

	if cp.System.Value != c_gp.ContactPointSystemCode_EMAIL {
		return ErrInvalidContactPointSystem
	}

	if cp.Value == nil {
		return ErrNilContactPointValue
	}

	if cp.Value.Value == "" {
		return ErrEmptyContactPointValueValue
	}

	if !IsProbablyEmail(cp.Value.Value) {
		return fmt.Errorf("%w: (got: '%s')", ErrInvalidEmailAddress, cp.Value.Value)
	}

	return nil
}

func MedplumPhone(cp *dt_gp.ContactPoint) error {
	if cp == nil {
		return ErrNilContactPoint
	}

	if cp.System == nil {
		return ErrNilContactPointSystem
	}

	if cp.System.Value != c_gp.ContactPointSystemCode_PHONE {
		return ErrInvalidContactPointSystemEmail
	}

	if cp.Value == nil {
		return ErrNilContactPointValue
	}

	if cp.Value.Value == "" {
		return ErrEmptyContactPointValueValue
	}

	// Metriport expects exactly `[0-9]{10}`
	if len(cp.Value.Value) != 10 {
		return fmt.Errorf("contact phone value must be a 10 digit number (got: '%s')", cp.Value.Value)
	}

	return nil
}

func MedplumContactPoint(cps []*dt_gp.ContactPoint) error {
	if len(cps) < 1 {
		return ErrEmptyContactPointList
	}

	// Must have email
	var emailFound bool

	for _, cp := range cps {
		if cp == nil {
			return ErrNilContactPoint
		}

		if cp.System == nil {
			return ErrNilContactPointSystem
		}

		var err error

		switch cp.System.Value {
		case c_gp.ContactPointSystemCode_EMAIL:
			err = MedplumEmail(cp)
			emailFound = true
		case c_gp.ContactPointSystemCode_PHONE:
			err = MedplumPhone(cp)
		default:
			// Unrecognized contact point system - store it and🙏
			continue
		}

		if err != nil {
			return errors.Wrap(err, ErrFailedContactPointValidation.Error())
		}
	}

	if !emailFound {
		return ErrPatientEmailNotFound
	}

	return nil
}

func MedplumWebhookEvent(event *common.Event) error {
	if event == nil {
		return ErrNilEvent
	}

	if event.GetMedplumWebhook() == nil {
		return ErrNilMedplumWebhookEvent
	}

	cr := event.GetMedplumWebhook().GetContainedResource()

	if err := MedplumContainedResource(cr); err != nil {
		return errors.Wrap(err, ErrMedplumCRValidationFailed.Error())
	}

	// TODO: Validate what's contained within the contained resource

	return nil
}

func MedplumContainedResource(cr *bcr_gp.ContainedResource) error {
	if cr == nil {
		return ErrNilContainedResource
	}

	return nil
}
