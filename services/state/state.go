package state

import (
	"context"

	"github.com/pkg/errors"
	"github.com/superpowerdotcom/go-common-lib/clog"
	"go.uber.org/zap"

	sb "github.com/dselans/blastbeat-api/backends/state"
)

type IState interface{}

type State struct {
	opts *Options
	log  clog.ICustomLog
}

type Options struct {
	Backend     sb.IState
	Log         clog.ICustomLog
	ShutdownCtx context.Context
}

func New(opts *Options) (*State, error) {
	if err := validateOptions(opts); err != nil {
		return nil, errors.Wrap(err, "failed to validate options")
	}

	return &State{
		opts: opts,
		log:  opts.Log.With(zap.String("pkg", "state")),
	}, nil
}

func validateOptions(opts *Options) error {
	if opts == nil {
		return errors.New("options cannot be nil")
	}

	if opts.Backend == nil {
		return errors.New("backend cannot be nil")
	}

	if opts.Log == nil {
		return errors.New("log cannot be nil")
	}

	if opts.ShutdownCtx == nil {
		return errors.New("shutdown context cannot be nil")
	}

	return nil
}
