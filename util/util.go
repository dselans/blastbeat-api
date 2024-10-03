package util

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/your_org/go-svc-template/clog"
)

// Error is a helper log func that will log an error to NewRelic and to a custom
// logger. All fields can be nil.
//
// Examples:
//
// Log(nil, nil, "foo", nil) -- will return errors.New("missing message")
// Log(txn, nil, "foo", nil) -- will notice Error
// Log(txn, logger, "foo", nil) -- will return errors.New("missing message") and log to logger
// Log(txn, logger, "foo", errors.New("bar")) -- will log "Foo: bar" to logger and NR + return errors.New("foo: bar")
// Log(nil, nil, nil, nil) -- will return nil
func Error(txn *newrelic.Transaction, log clog.ICustomLog, msg string, err error, fields ...zap.Field) error {
	if err == nil && msg == "" {
		// Nothing to do if neither error or msg is present
		return nil
	} else if err != nil && msg != "" {
		// If both err and msg are present, wrap err with msg
		err = errors.Wrap(err, msg)
	} else if err == nil && msg != "" {
		// If only msg is present, use msg for err
		err = errors.New(msg)
	} else if err != nil && msg == "" {
		// If err is provided but no msg, leave err as-is
	}

	if txn != nil {
		txn.NoticeError(err)
	}

	if log != nil {
		log.Error(CapitalizeFirstChar(err.Error()), fields...)
	}

	return err
}

func CapitalizeFirstChar(s string) string {
	if len(s) == 0 {
		return s
	}

	return strings.ToUpper(string(s[0])) + s[1:]
}

// MethodSetup is a helper function that will attempt to extract a NewRelic txn
// and a logger from a provided context. It is used partly to reduce boilerplate
// setup code in all methods but most importantly, it ensures that every method
// has access to a logger that contains common log fields like cloudEventID,
// cloudEventType and cloudEventSource.
//
// If the context doesn't contain a txn, NewRelic lib will continue to be able
// to handle calls o nil transactions.
//
// If the context does not contain a logger, it will try to use a fallback
// logger. If no fallback logger is provided, a Basic logger will be created and
// a noisy error will be printed.
func MethodSetup(ctx context.Context, fallbackLogger clog.ICustomLog, fields ...zap.Field) (*newrelic.Transaction, clog.ICustomLog) {
	// If ctx is nil, returned txn will be nil of *Transaction type and NewRelic
	// lib is able to handle calls on nil transactions.
	txn := newrelic.FromContext(ctx)

	// If there is no context, we should use the fallback logger
	if ctx == nil {
		// But if there is no fallback logger, we should print a noisy message + use Basic logger
		if fallbackLogger == nil {
			fmt.Println("WARNING: CTX IS NIL AND NO FALLBACK LOGGER PROVIDED, RETURNING BASIC LOGGER")
			return txn, clog.NewBasic(fields...)
		}

		fmt.Println("WARNING: CTX IS NIL, USING FALLBACK LOGGER")
		return txn, fallbackLogger.With(fields...)
	}

	// Context is non-nil, check if it has a logger
	logger, ok := ctx.Value("logger").(clog.ICustomLog)
	if !ok {
		if fallbackLogger != nil {
			logger = fallbackLogger
		} else {
			fmt.Println("WARNING: NO LOGGER FOUND IN CTX AND NO FALLBACK LOGGER PROVIDED")
			logger = clog.NewBasic()
		}
	}

	// Attach fields to logger
	for _, f := range fields {
		logger = logger.With(f)
	}

	return txn, logger
}

func DoHTTP(
	ctx context.Context,
	endpoint,
	method string,
	requestBody []byte,
	target any,
	header ...http.Header,
) (*http.Response, error) {
	txn, logger := MethodSetup(ctx, nil, zap.String("method", "DoHTTP"))
	segment := txn.StartSegment("util.DoHTTP")
	defer segment.End()

	if target != nil {
		if reflect.ValueOf(target).Kind() != reflect.Ptr {
			return nil, errors.New("target must be a pointer")
		}
	}

	logger = logger.With(
		zap.String("method", "DoHTTP"),
		zap.String("httpEndpoint", endpoint),
		zap.String("httpMethod", method),
		zap.String("httpBody", string(requestBody)),
	)

	logger.Debug("Performing HTTP request")

	txn.AddAttribute("httpEndpoint", endpoint)
	txn.AddAttribute("httpMethod", method)

	// Automatically handles nil requestBody
	bodyBuffer := bytes.NewBuffer(requestBody)

	// Generate request
	request, err := http.NewRequest(method, endpoint, bodyBuffer)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create http request")
	}

	// Set headers
	for _, h := range header {
		request.Header = h
	}

	// Create HTTP client
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Perform the request
	resp, err := client.Do(request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to perform http request")
	}

	defer resp.Body.Close()

	body, err := GetResponseBody(resp)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get response body")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("received non-200 status code: %d; resp body: %s", resp.StatusCode, string(body))
	}

	// If there is no target, we are done
	if target == nil {
		return resp, nil
	}

	// Target is non-nil, let's determine if we need to unmarshal using protojson
	// or encoding/json.
	switch target.(type) {
	case proto.Message:
		logger.Debug("Unmarshalling response body using protojson")
		// We can safely assert as we already checked the type
		err = ProtoJSONUnmarshal(body, target.(proto.Message), true)
	default:
		logger.Debug("Unmarshalling response body using encoding/json")
		err = json.Unmarshal(body, &target)
	}

	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response body")
	}

	return resp, nil
}

func GetResponseBody(resp *http.Response) ([]byte, error) {
	if resp == nil {
		return nil, errors.New("response cannot be nil")
	}

	if resp.Body == nil {
		return nil, errors.New("response body cannot be nil")
	}

	defer resp.Body.Close()

	// Read the response body into a buffer so we can re-add the body back into
	// the response for others to use
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "unable to read response body")
	}

	// Re-create body so it can be read again
	resp.Body = io.NopCloser(bytes.NewReader(buf.Bytes()))

	return buf.Bytes(), nil
}

func ProtoJSONUnmarshal(data []byte, message proto.Message, discardUnknown bool) error {
	unmarshaller := &protojson.UnmarshalOptions{
		DiscardUnknown: discardUnknown,
	}

	return unmarshaller.Unmarshal(data, message)
}
