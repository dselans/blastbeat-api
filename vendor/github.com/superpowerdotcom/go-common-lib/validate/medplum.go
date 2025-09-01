package validate

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/superpowerdotcom/events/build/proto/go/common"
	c_gp "github.com/superpowerdotcom/fhir/go/proto/google/fhir/proto/r4/core/codes_go_proto"
	dt_gp "github.com/superpowerdotcom/fhir/go/proto/google/fhir/proto/r4/core/datatypes_go_proto"
	ad_gp "github.com/superpowerdotcom/fhir/go/proto/google/fhir/proto/r4/core/resources/activity_definition_go_proto"
	bcr_gp "github.com/superpowerdotcom/fhir/go/proto/google/fhir/proto/r4/core/resources/bundle_and_contained_resource_go_proto"
	dr_gp "github.com/superpowerdotcom/fhir/go/proto/google/fhir/proto/r4/core/resources/diagnostic_report_go_proto"
	od_gp "github.com/superpowerdotcom/fhir/go/proto/google/fhir/proto/r4/core/resources/observation_definition_go_proto"
	o_gp "github.com/superpowerdotcom/fhir/go/proto/google/fhir/proto/r4/core/resources/observation_go_proto"
	p_gp "github.com/superpowerdotcom/fhir/go/proto/google/fhir/proto/r4/core/resources/patient_go_proto"
	pd_gp "github.com/superpowerdotcom/fhir/go/proto/google/fhir/proto/r4/core/resources/plan_definition_go_proto"
	sr_gp "github.com/superpowerdotcom/fhir/go/proto/google/fhir/proto/r4/core/resources/service_request_go_proto"

	"github.com/superpowerdotcom/go-common-lib/util"
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
	if len(cp.Value.Value) < 10 {
		return fmt.Errorf("contact phone value must have at least have 10 chars (got: '%s')", cp.Value.Value)
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

	switch cr.OneofResource.(type) {
	case *bcr_gp.ContainedResource_ObservationDefinition:
		return MedplumObservationDefinition(cr.GetObservationDefinition())
	case *bcr_gp.ContainedResource_ServiceRequest:
		return MedplumServiceRequest(cr.GetServiceRequest())
	case *bcr_gp.ContainedResource_Patient:
		return MedplumPatient(cr.GetPatient(), true)
	default:
		return nil
	}
}

func MedplumDiagnosticReport(resource *dr_gp.DiagnosticReport) error {
	if resource == nil {
		return errors.New("diagnostic report cannot be nil")
	}

	if resource.GetId() == nil || resource.GetId().GetValue() == "" {
		return errors.New("diagnostic report ID cannot be nil or empty")
	}

	if resource.GetStatus() == nil {
		return errors.New("diagnostic report status cannot be nil")
	}

	if resource.GetCode() == nil {
		return errors.New("diagnostic report code cannot be nil")
	}

	// ref to patient
	if resource.GetSubject() == nil || resource.GetSubject().GetReference() == nil {
		return errors.New("diagnostic report subject reference cannot be nil")
	}

	if resource.GetEffective() == nil {
		return errors.New("diagnostic report effective (datetime) cannot be nil")
	}

	if resource.GetIssued() == nil {
		return errors.New("diagnostic report issued cannot be nil")
	}

	// ref to service request
	if resource.GetBasedOn() == nil || len(resource.GetBasedOn()) == 0 {
		return errors.New("diagnostic report basedOn cannot be nil or empty")
	}

	if resource.GetResult() == nil || len(resource.GetResult()) == 0 {
		return errors.New("diagnostic report result cannot be nil or empty")
	}

	return nil
}

func MedplumServiceRequest(resource *sr_gp.ServiceRequest) error {
	if resource == nil {
		return errors.New("service request cannot be nil")
	}

	if resource.GetId() == nil || resource.GetId().GetValue() == "" {
		return errors.New("service request ID cannot be nil or empty")
	}

	if resource.GetStatus() == nil {
		return errors.New("service request status cannot be nil")
	}

	if resource.GetIntent() == nil {
		return errors.New("service request intent cannot be nil")
	}

	if resource.GetCode() == nil {
		return errors.New("service request code cannot be nil")
	}

	if resource.GetSubject() == nil || resource.GetSubject().GetReference() == nil {
		return errors.New("service request subject reference cannot be nil")
	}

	if resource.GetOccurrence() == nil {
		return errors.New("service request occurrence (datetime) cannot be nil")
	}

	if resource.GetOrderDetail() == nil || len(resource.GetOrderDetail()) == 0 {
		return errors.New("service request orderDetail cannot be nil or empty")
	}

	return nil
}

func MedplumObservationDefinition(resource *od_gp.ObservationDefinition) error {
	if resource == nil {
		return errors.New("observation definition cannot be nil")
	}

	if resource.GetId() == nil || resource.GetId().GetValue() == "" {
		return errors.New("observation definition ID cannot be nil or empty")
	}

	if resource.GetCode() == nil {
		return errors.New("observation definition code cannot be nil")
	}

	if resource.GetIdentifier() == nil || len(resource.GetIdentifier()) == 0 {
		return errors.New("observation definition identifier cannot be nil or empty")
	}

	if resource.GetPreferredReportName() == nil || resource.GetPreferredReportName().GetValue() == "" {
		return errors.New("observation definition preferredReportName cannot be nil or empty")
	}

	if resource.GetPermittedDataType() == nil || len(resource.GetPermittedDataType()) == 0 {
		return errors.New("observation definition permittedDataType cannot be nil or empty")
	}

	if resource.GetQualifiedInterval() == nil || len(resource.GetQualifiedInterval()) == 0 {
		return errors.New("observation definition qualifiedInterval cannot be nil or empty")
	}

	if resource.GetQuantitativeDetails() == nil {
		return errors.New("observation definition quantitativeDetails cannot be nil")
	}

	return nil
}

func MedplumObservation(resource *o_gp.Observation) error {
	if resource == nil {
		return errors.New("observation cannot be nil")
	}

	return nil
}

// TODO: Implement ~DS 07.14.2025
func MedplumActivityDefinition(resource *ad_gp.ActivityDefinition) error {
	if resource == nil {
		return errors.New("activity definition cannot be nil")
	}

	if resource.GetId() == nil || resource.GetId().GetValue() == "" {
		return errors.New("activity definition ID cannot be nil or empty")
	}

	if resource.GetStatus() == nil {
		return errors.New("activity definition status cannot be nil")
	}

	if resource.GetCode() == nil {
		return errors.New("activity definition code cannot be nil")
	}

	if resource.GetKind() == nil {
		return errors.New("activity definition kind cannot be nil")
	}

	if resource.GetName() == nil || resource.GetName().GetValue() == "" {
		return errors.New("activity definition name cannot be nil or empty")
	}

	if resource.GetTitle() == nil || resource.GetTitle().GetValue() == "" {
		return errors.New("activity definition title cannot be nil or empty")
	}

	if resource.GetUrl() == nil || resource.GetUrl().GetValue() == "" {
		return errors.New("activity definition url cannot be nil or empty")
	}

	if resource.GetIdentifier() == nil || len(resource.GetIdentifier()) == 0 {
		return errors.New("activity definition identifier cannot be nil or empty")
	}

	if util.GetIdentifier(util.FHIRIdentifierSlugSystemURL, resource.GetIdentifier()) == "" {
		return errors.New("activity definition must have slug identifier")
	}

	if resource.GetDescription() == nil || resource.GetDescription().GetValue() == "" {
		return errors.New("activity definition description cannot be nil or empty")
	}

	if resource.GetRelatedArtifact() != nil {
		if err := MedplumRelatedArtifact(resource.GetRelatedArtifact()...); err != nil {
			return fmt.Errorf("activity definition relatedArtifact validation failed: %w", err)
		}
	}

	if resource.GetObservationRequirement() != nil {
		if err := MedplumObservationResultRequirement(resource.GetObservationRequirement()...); err != nil {
			return fmt.Errorf("activity definition observationRequirement validation failed: %w", err)
		}
	}

	if resource.GetSpecimenRequirement() != nil {
		if err := MedplumSpecimenRequirement(resource.GetSpecimenRequirement()); err != nil {
			return fmt.Errorf("activity definition specimenRequirement validation failed: %w", err)
		}
	}

	return nil
}

func MedplumPlanDefinition(resource *pd_gp.PlanDefinition) error {
	if resource == nil {
		return errors.New("plan definition cannot be nil")
	}

	if resource.GetId() == nil || resource.GetId().GetValue() == "" {
		return errors.New("plan definition ID cannot be nil or empty")
	}

	if resource.GetStatus() == nil {
		return errors.New("plan definition status cannot be nil")
	}

	if resource.GetName() == nil || resource.GetName().GetValue() == "" {
		return errors.New("plan definition name cannot be nil or empty")
	}

	if resource.GetTitle() == nil || resource.GetTitle().GetValue() == "" {
		return errors.New("plan definition title cannot be nil or empty")
	}

	if resource.GetType() == nil {
		return errors.New("plan definition type cannot be nil")
	}

	if resource.GetUsage() == nil || resource.GetUsage().GetValue() == "" {
		return errors.New("plan definition usage cannot be nil or empty")
	}

	if resource.GetAction() != nil {
		if err := MedplumPlanDefinitionAction(resource.GetAction()...); err != nil {
			return fmt.Errorf("plan definition action validation failed: %w", err)
		}
	}

	return nil
}

func MedplumPlanDefinitionForCalculatedObservation(resource *pd_gp.PlanDefinition) error {
	if err := MedplumPlanDefinition(resource); err != nil {
		return errors.Wrap(err, "calculated observation plan definition validation failed")
	}

	if resource.GetAction() == nil || len(resource.GetAction()) == 0 {
		return errors.New("action in calculated observation plan definition cannot be nil or empty")
	}

	if err := MedplumPlanDefinitionAction(resource.GetAction()...); err != nil {
		return errors.Wrap(err, "calculated observation action validation failed")
	}

	return nil
}

func MedplumPlanDefinitionAction(actions ...*pd_gp.PlanDefinition_Action) error {
	for _, action := range actions {
		if action == nil {
			return errors.New("action entry cannot be nil")
		}

		if action.GetId() == nil || action.GetId().GetValue() == "" {
			return errors.New("action id cannot be nil or empty")
		}

		if action.GetTitle() == nil || action.GetTitle().GetValue() == "" {
			return errors.New("action title cannot be nil or empty")
		}

		if action.GetDescription() == nil || action.GetDescription().GetValue() == "" {
			return errors.New("action description cannot be nil or empty")
		}

		if action.GetRelatedAction() != nil {
			if err := MedplumPlanDefinitionRelatedAction(action.GetRelatedAction()); err != nil {
				return fmt.Errorf("action relatedAction validation failed: %w", err)
			}
		}

		if action.GetDynamicValue() != nil {
			if err := MedplumPlanDefinitionDynamicValue(action.GetDynamicValue()); err != nil {
				return fmt.Errorf("action dynamicValue validation failed: %w", err)
			}
		}
	}

	return nil
}

func MedplumPlanDefinitionRelatedAction(relatedActions []*pd_gp.PlanDefinition_Action_RelatedAction) error {
	for _, ra := range relatedActions {
		if ra == nil {
			return errors.New("relatedAction entry cannot be nil")
		}

		if ra.GetActionId() == nil || ra.GetActionId().GetValue() == "" {
			return errors.New("relatedAction actionId cannot be nil or empty")
		}

		if ra.GetRelationship() == nil {
			return errors.New("relatedAction relationship cannot be nil")
		}
	}

	return nil
}

func MedplumPlanDefinitionDynamicValue(dynamicValues []*pd_gp.PlanDefinition_Action_DynamicValue) error {
	for _, dv := range dynamicValues {
		if dv == nil {
			return errors.New("dynamicValue entry cannot be nil")
		}

		if dv.GetPath() == nil || dv.GetPath().GetValue() == "" {
			return errors.New("dynamicValue path cannot be nil or empty")
		}

		if dv.GetExpression() == nil {
			return errors.New("dynamicValue expression cannot be nil")
		}
	}

	return nil
}

func MedplumRelatedArtifact(artifacts ...*dt_gp.RelatedArtifact) error {
	for _, artifact := range artifacts {
		if artifact == nil {
			return errors.New("relatedArtifact entry cannot be nil")
		}

		if artifact.GetType() == nil {
			return errors.New("relatedArtifact type cannot be nil")
		}

		if artifact.GetResource() == nil || artifact.GetResource().GetValue() == "" {
			return errors.New("relatedArtifact resource cannot be nil or empty")
		}
	}

	return nil
}

func MedplumObservationResultRequirement(requirements ...*dt_gp.Reference) error {
	for _, req := range requirements {
		if req == nil {
			return errors.New("observationRequirement entry cannot be nil")
		}

		if req.GetReference() == nil {
			return errors.New("observationRequirement reference cannot be nil or empty")
		}
	}

	return nil
}

func MedplumSpecimenRequirement(requirements []*dt_gp.Reference) error {
	for _, req := range requirements {
		if req == nil {
			return errors.New("specimenRequirement entry cannot be nil")
		}

		if req.GetReference() == nil {
			return errors.New("specimenRequirement reference cannot be nil or empty")
		}
	}

	return nil
}
