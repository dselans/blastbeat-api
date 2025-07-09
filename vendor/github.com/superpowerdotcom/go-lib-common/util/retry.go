package util

import (
	"time"

	"github.com/pkg/errors"

	"go.uber.org/zap"

	"github.com/superpowerdotcom/go-lib-common/clog"
)

const (
	DefaultRetryDelay = 500 * time.Millisecond
)

// NonRetryableError is a custom error type to indicate non-retryable errors.
type NonRetryableError struct {
	Err error
}

func (e *NonRetryableError) Error() string {
	if e.Err == nil {
		return ""
	}

	return e.Err.Error()
}

func (e *NonRetryableError) Unwrap() error {
	return e.Err
}

func (e *NonRetryableError) Is(target error) bool {
	_, ok := target.(*NonRetryableError)
	return ok
}

func NewNonRetryableError(err error) error {
	return &NonRetryableError{Err: err}
}

// RetryOptions holds optional parameters for RetryFunc.
type RetryOptions struct {
	Logger    clog.ICustomLog
	BaseDelay time.Duration
}

// RetryOption is a function that modifies RetryOptions.
type RetryOption func(*RetryOptions)

// WithLogger sets a custom logger for RetryFunc.
func WithLogger(logger clog.ICustomLog) RetryOption {
	return func(opts *RetryOptions) {
		opts.Logger = logger
	}
}

func WithDelay(delay time.Duration) RetryOption {
	return func(opts *RetryOptions) {
		opts.BaseDelay = delay
	}
}

func RetryFunc(fn func() error, maxRetries int, opts ...RetryOption) error {
	options := &RetryOptions{
		BaseDelay: DefaultRetryDelay,
	}

	for _, opt := range opts {
		opt(options)
	}

	var llog clog.ICustomLog
	llog = &clog.CustomLogNoop{}

	if options.Logger != nil {
		llog = options.Logger.With(
			zap.String("method", "RetryFunc"),
			zap.Int("maxRetries", maxRetries),
		)
	}

	var err error

	for i := 0; i < maxRetries; i++ {
		llog.Debug("Exec", zap.Int("attempt", i+1))

		if err = fn(); err == nil {
			return nil
		}

		var nonRetryableErr *NonRetryableError

		if errors.As(err, &nonRetryableErr) {
			llog.Warn("Non-retryable error encountered, stopping retries", zap.Int("attempt", i+1), zap.Error(err))
			return nonRetryableErr
		}

		llog.Warn("Retry failed", zap.Int("attempt", i+1), zap.Error(err))

		time.Sleep(options.BaseDelay * (1 << i))
	}

	llog.Error("All retry attempts failed", zap.Error(err))

	return errors.Wrap(err, "all retry attempts failed")
}
