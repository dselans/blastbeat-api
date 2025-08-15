package util

import (
	"github.com/pkg/errors"
	dt_gp "github.com/superpowerdotcom/fhir/go/proto/google/fhir/proto/r4/core/datatypes_go_proto"
)

const (
	CodeSystemRangeType                       = "https://superpower.com/fhir/CodeSystem/range-type"
	CodeSystemObservationDefinitionStatus     = "https://superpower.com/fhir/CodeSystem/observation-definition-status"
	CodeSystemVitalLabcorpTestCode            = "https://superpower.com/fhir/CodeSystem/vital-labcorp-test-code"
	CodeSystemVitalBiorefTestCode             = "https://superpower.com/fhir/CodeSystem/vital-bioref-test-code"
	CodeSystemUnknownTestCode                 = "https://superpower.com/fhir/CodeSystem/unknown-test-code"
	CodeSystemObservationInterpretationStatus = "https://superpower.com/fhir/CodeSystem/observation-interpretation-status"
	CodeSystemDiagnosticReportStatus          = "https://superpower.com/fhir/CodeSystem/diagnostic-report-status"
	CodeSystemOrganizationType                = "https://superpower.com/fhir/CodeSystem/organization-type"

	IdentifierObservationDatabaseID      = "https://superpower.com/fhir/Identifier/observation-database-id"
	IdentifierServiceRequestDatabaseID   = "https://superpower.com/fhir/Identifier/service-request-database-id"
	IdentifierDiagnosticReportDatabaseID = "https://superpower.com/fhir/Identifier/diagnostic-report-database-id"

	ExtensionObservationMigrationMetadata = "https://superpower.com/fhir/Extension/observation-migration-metadata"
	ExtensionObservationStatus            = "https://superpower.com/fhir/Extension/observation-status"

	FHIRActivityDefinitionSystemURL                        = "https://superpower.com/fhir/ActivityDefinition"
	FHIRIdentifierSlugSystemURL                            = "https://superpower.com/fhir/Identifier/slug"
	FHIRExtensionObservationMetadataSystemURL              = "https://superpower.com/fhir/Extension/observation-metadata"
	FHIRCodeSystemObservationInterpretationStatusSystemURL = "https://superpower.com/fhir/CodeSystem/observation-interpretation-status"
	FHIRCodeSystemObservationDefinitionStatusSystemURL     = "https://superpower.com/fhir/CodeSystem/observation-definition-status"
)

func GetIdentifier(systemValue string, ids []*dt_gp.Identifier) string {
	for _, id := range ids {
		if id.System == nil || id.System.Value != systemValue || id.Value == nil {
			continue
		}

		return id.GetValue().GetValue()
	}

	return ""
}

/**
 * Sets a resource identifier for the given system.
 *
 * Note that this method is only available on resources that have an "identifier" property,
 * and that property must be an array of Identifier objects,
 * which is not true for all FHIR resources.
 *
 * If the identifier already exists, then the value is updated.
 *
 * Otherwise a new identifier is added.
 *
 * @param resource - The resource to add the identifier to.
 * @param system - The identifier system.
 * @param value - The identifier value.
 */
func SetIdentifier(identifiers []*dt_gp.Identifier, system string, value string) []*dt_gp.Identifier {
	if identifiers == nil {
		return nil
	}
	for _, identifier := range identifiers {
		if identifier.System != nil && identifier.System.Value == system {
			if identifier.Value == nil {
				identifier.Value = &dt_gp.String{Value: value}
			} else {
				identifier.Value.Value = value
			}
			return identifiers
		}
	}
	updatedIdentifiers := append(identifiers, &dt_gp.Identifier{
		System: &dt_gp.Uri{Value: system},
		Value:  &dt_gp.String{Value: value},
	})

	return updatedIdentifiers
}

/**
* Returns true if the input value is a CodeableConcept object.
* This is a heuristic check based on the presence of the "coding" property.
* @param value - The candidate value.
* @returns True if the input value is a CodeableConcept.
 */
func IsCodeableConcept(value interface{}) bool {
	obj, ok := value.(map[string]interface{})
	if !ok {
		return false
	}

	coding, exists := obj["coding"]
	if !exists {
		return false
	}

	codingSlice, ok := coding.([]interface{})
	if !ok {
		return false
	}

	for _, item := range codingSlice {
		if !IsCoding(item) {
			return false
		}
	}

	return true
}

/**
 * Sets a code for a given system within a given codeable concept.
 * @param concept - The codeable concept.
 * @param system - The system string.
 * @param code - The code value.
 */
func SetCodeBySystem(concept *dt_gp.CodeableConcept, system string, code string) error {
	if concept == nil {
		return errors.New("CodeableConcept cannot be nil")
	}
	if system == "" {
		return errors.New("system cannot be empty")
	}
	if code == "" {
		return errors.New("code cannot be empty")
	}

	if concept.Coding == nil {
		concept.Coding = []*dt_gp.Coding{}
	}

	for _, coding := range concept.Coding {
		if coding.System != nil && coding.System.Value == system {
			if coding.Code == nil {
				coding.Code = &dt_gp.Code{Value: code}
			} else {
				coding.Code.Value = code
			}
			return nil
		}
	}

	concept.Coding = append(concept.Coding, &dt_gp.Coding{
		System: &dt_gp.Uri{Value: system},
		Code:   &dt_gp.Code{Value: code},
	})

	return nil
}

/**
* Tries to find a code string for a given system within a given codeable concept.
* @param concept - The codeable concept.
  * @param system - The system string.
  * @returns The code if found; otherwise nil.
*/
func GetCodeBySystem(concept *dt_gp.CodeableConcept, system string) *string {
	if concept == nil || concept.Coding == nil {
		return nil
	}
	if system == "" {
		return nil
	}

	for _, coding := range concept.Coding {
		if coding.System != nil && coding.System.Value == system {
			if coding.Code != nil {
				return &coding.Code.Value
			}
		}
	}

	return nil
}

// IsCoding checks if the input value is a valid Coding object.
// This function should be implemented based on the structure of Coding.
func IsCoding(value interface{}) bool {
	// Check if the value is a map
	obj, ok := value.(map[string]interface{})
	if !ok {
		return false
	}

	// Check if the "code" key exists and its value is a string
	code, exists := obj["code"]
	if !exists {
		return false
	}

	_, isString := code.(string)
	return isString
}
