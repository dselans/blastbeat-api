package util

import (
	"github.com/pkg/errors"
	"github.com/superpowerdotcom/fhir/go/proto/google/fhir/proto/r4/core/datatypes_go_proto"
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
)

func GetIdentifier(systemValue string, ids []*datatypes_go_proto.Identifier) (string, error) {
	for _, id := range ids {
		if id.System == nil || id.System.Value != systemValue || id.Value == nil {
			continue
		}

		return id.Value.Value, nil
	}

	return "", errors.New("no database ID found")
}
