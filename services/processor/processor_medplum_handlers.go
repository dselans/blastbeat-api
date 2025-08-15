package processor

import (
	"context"

	"github.com/superpowerdotcom/events/build/proto/go/common"
	"github.com/superpowerdotcom/go-common-lib/util"
	rvalidate "github.com/superpowerdotcom/go-common-lib/validate"
	"go.uber.org/zap"
)

/*
	THIS IS AN EXAMPLE HANDLER - COPY IT AS BASE FOR YOUR OWN, CUSTOM HANDLERS

	TIPS:

	1) Create temporary logger to include attributes across all log messages:

		logger = logger.With(
			zap.String("foo", "bar"),
			zap.String("baz", "qux"),
		)

	2) For medplum-related helpers - use github.com/superpowerdotcom/go-medplum-lib

	3) For common helper funcs, validation - use github.com/superpowerdotcom/go-common-lib

	4) Ensure you use `jsonformat` library for marshalling/unmarshalling FHIR.
	   If you use another proto json lib, you will generate invalid FHIR JSON!

	5) Use `github.com/superpowerdotcom/fhir` instead of `github.com/google/fhir`!
	   We run a custom fork of the FHIR lib which has our own custom types in it
       and various modifications necessary for working with Medplum.

	6) Use New Relic! The passed context includes NR transaction and logger - it
       is your responsibility to add additional Start/End segment calls, add
       attributes, etc.

    7) Do insightful logging - attach attributes that will help debug issues,
       add contextual information, do not return blank `err`.

	8) And if unsure - consult #go in Slack!
*/

// handleMedplumWebhook processes the MedplumWebhook event and determines the appropriate action
func (p *Processor) handleMedplumWebhook(ctx context.Context, event *common.Event) error {
	txn, logger := util.MethodSetup(ctx, p.log, zap.String("method", "handleMedplumWebhook"))
	segment := txn.StartSegment("ProcessorService.handleMedplumWebhook")
	defer segment.End()

	logger.Info("Handling medplum.Webhook event", zap.Any("event", event))

	if err := rvalidate.MedplumWebhookEvent(event); err != nil {
		return util.Error(txn, logger, "failed to validate medplum webhook event", err)
	}

	// We only care about DiagnosticReport events
	if event.GetMedplumWebhook().GetContainedResource().GetDiagnosticReport() == nil {
		logger.Debug("Ignoring non-diagnostic report event")
		return nil
	}

	logger.Debug("Event validation succeeded")

	// Add additional business logic here

	return nil
}
