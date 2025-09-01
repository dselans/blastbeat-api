package validate

import (
	"github.com/pkg/errors"

	"github.com/superpowerdotcom/events/build/proto/go/superpower"
)

var (
	// SuperpowerDocumentUploadRequest Errors
	ErrNilDocumentUploadRequest = errors.New("request cannot be nil")
	ErrEmptyMessageID           = errors.New("message id cannot be empty")
	ErrFailedDocumentValidation = errors.New("failed to validate document entry")

	// SuperpowerDocumentUploadEntries Errors
	ErrEmptyDocumentEntries          = errors.New("entries must have at least one entry")
	ErrFailedDocumentEntryValidation = errors.New("failed to validate document entry")

	// SuperpowerDocumentUploadEntry Errors
	ErrNilDocumentUploadEntry = errors.New("entry cannot be nil")
	ErrEmptyMimeType          = errors.New("mime type cannot be empty")
	ErrEmptyURL               = errors.New("url cannot be empty")
)

func SuperpowerDocumentUploadRequest(req *superpower.WebhookDocumentUploadRequest) error {
	if req == nil {
		return ErrNilDocumentUploadRequest
	}

	if req.MessageId == "" {
		return ErrEmptyMessageID
	}

	if req.DatabaseUserId == "" {
		return ErrEmptyDatabaseUserID
	}

	if req.MetriportPatientId == "" {
		return ErrEmptyMetriportPatientID
	}

	if err := SuperpowerDocumentUploadEntries(req.Documents); err != nil {
		return errors.Wrap(err, ErrFailedDocumentValidation.Error())
	}

	return nil
}

func SuperpowerDocumentUploadEntries(entries []*superpower.WebhookDocumentUploadEntry) error {
	if len(entries) < 1 {
		return ErrEmptyDocumentEntries
	}

	for _, entry := range entries {
		if err := SuperpowerDocumentUploadEntry(entry); err != nil {
			return errors.Wrap(err, ErrFailedDocumentEntryValidation.Error())
		}
	}

	return nil
}

func SuperpowerDocumentUploadEntry(entry *superpower.WebhookDocumentUploadEntry) error {
	if entry == nil {
		return ErrNilDocumentUploadEntry
	}

	if entry.MimeType == "" {
		return ErrEmptyMimeType
	}

	if entry.Url == "" {
		return ErrEmptyURL
	}

	return nil
}
