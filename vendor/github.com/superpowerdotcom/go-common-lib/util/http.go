package util

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"reflect"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

func DoHTTP(
	ctx context.Context,
	endpoint,
	method string,
	requestBody []byte,
	target any,
	headers []http.Header,
	client ...*http.Client,
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
	)

	// Only log request body for JSON content
	if len(headers) > 0 {
		for _, h := range headers {
			if contentType := h.Get("Content-Type"); contentType == "application/json" {
				logger = logger.With(zap.String("httpBody", string(requestBody)))
				break
			}
		}
	}

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
	for _, h := range headers {
		for key, values := range h {
			for _, value := range values {
				request.Header.Add(key, value)
			}
		}
	}

	// Create HTTP client
	var httpClient *http.Client
	if len(client) == 0 {
		httpClient = &http.Client{
			Timeout: 5 * time.Second,
		}
	} else {
		httpClient = client[0]
	}
	// Perform the request
	resp, err := httpClient.Do(request)
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

func WriteJSON(rw http.ResponseWriter, payload interface{}, status int) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("ERROR: unable to marshal JSON during WriteJSON "+
			"(payload: '%s'; status: '%d'): %s\n", payload, status, err)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)

	if _, err := rw.Write(data); err != nil {
		log.Printf("ERROR: unable to write resp in WriteJSON: %s\n", err)
		return
	}
}
