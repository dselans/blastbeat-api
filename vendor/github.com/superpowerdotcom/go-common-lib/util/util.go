package util

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/goforj/godump"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/superpowerdotcom/events/build/proto/go/user"

	"github.com/superpowerdotcom/go-common-lib/clog"
)

const (
	beginGreen = "\033[32m"
	endGreen   = "\033[0m"
)

// Ptr returns a pointer to the provided value.
// This exists so we can avoid having to declare a variable to get a pointer to a value
func Ptr[T any](v T) *T {
	return &v
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

func GetStateCode(state user.AddressState) string {
	var defaultState string

	stateString := state.String()

	if stateString == "" {
		return defaultState
	}

	// Example state: "ADDRESS_STATE_NY"
	split := strings.Split(stateString, "_")

	if len(split) == 3 {
		return split[2]
	}

	return strings.ToUpper(defaultState)
}

func NormalizePhone(phone string) string {
	if phone == "" {
		return ""
	}

	// Remove all non-digit characters
	normalized := ""
	for _, c := range phone {
		if c >= '0' && c <= '9' {
			normalized += string(c)
		}
	}

	// Remove leading 1
	if normalized[0] == '1' {
		normalized = normalized[1:]
	}

	return normalized
}

func ReadBody(body io.ReadCloser) string {
	defer body.Close()

	b, err := io.ReadAll(body)
	if err != nil {
		return fmt.Sprintf("error reading body: %v", err)
	}

	return string(b)
}

func Dump(input ...any) {
	d := godump.NewDumper(
		godump.WithMaxDepth(8),
		godump.WithMaxItems(20),
		godump.WithMaxStringLen(1000),
		godump.WithWriter(os.Stdout),
	)

	fmt.Println(beginGreen, d.DumpJSONStr(input...), endGreen)
}

func StringInSlice(s string, list []string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}

	return false
}
