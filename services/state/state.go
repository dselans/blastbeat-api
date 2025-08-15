package state

import (
	"context"

	"github.com/pkg/errors"
	"github.com/superpowerdotcom/events/build/proto/go/user"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/superpowerdotcom/go-common-lib/clog"
	sb "github.com/superpowerdotcom/go-svc-template/backends/state"
)

var (
	UserPrefix = "user"
)

type IState interface {
	GetUser(ctx context.Context, id string) (*user.User, error)
	AddUser(ctx context.Context, user *user.User) error
	SetUser(ctx context.Context, user *user.User) error
	DeleteUser(ctx context.Context, id string) error
}

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

func (s State) GetUser(ctx context.Context, id string) (*user.User, error) {
	if id == "" {
		return nil, errors.New("id cannot be empty")
	}

	if ctx == nil {
		ctx = s.opts.ShutdownCtx
	}

	userData, err := s.opts.Backend.Get(ctx, id, "user")
	if err != nil {
		if errors.Is(err, sb.ErrDoesNotExist) {
			return nil, sb.ErrDoesNotExist
		}

		return nil, errors.Wrap(err, "failed to get user")
	}

	s.log.Debug("found user in state", zap.String("id", id))

	userEntry := &user.User{}

	if err := proto.Unmarshal([]byte(userData), userEntry); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal existing user")
	}

	return userEntry, nil
}

func (s State) AddUser(ctx context.Context, user *user.User) error {
	if user == nil {
		return errors.New("user cannot be nil")
	}

	if ctx == nil {
		ctx = s.opts.ShutdownCtx
	}

	userData, err := proto.Marshal(user)
	if err != nil {
		return errors.Wrap(err, "failed to marshal user")
	}

	if err := s.opts.Backend.Add(ctx, user.Id, string(userData), UserPrefix); err != nil {
		return errors.Wrap(err, "failed to add user")
	}

	return nil
}

func (s State) SetUser(ctx context.Context, user *user.User) error {
	if user == nil {
		return errors.New("user cannot be nil")
	}

	if ctx == nil {
		ctx = s.opts.ShutdownCtx
	}

	userData, err := proto.Marshal(user)
	if err != nil {
		return errors.Wrap(err, "failed to marshal user")
	}

	if err := s.opts.Backend.Set(ctx, user.Id, string(userData), UserPrefix); err != nil {
		return errors.Wrap(err, "failed to set user")
	}

	return nil
}

func (s State) DeleteUser(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("id cannot be empty")
	}

	if ctx == nil {
		ctx = s.opts.ShutdownCtx
	}

	if err := s.opts.Backend.Delete(ctx, id); err != nil {
		return errors.Wrap(err, "failed to delete user")
	}

	return nil
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
