// Package state is used to store state for the service in a global redis store.
//
// This package will automatically set a prefix
package state

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/bsm/redislock"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/superpowerdotcom/go-common-lib/clog"
)

var (
	ErrAlreadyExists = errors.New("key already exists")
	ErrDoesNotExist  = errors.New("key does not exist")
	ErrNilValue      = errors.New("value cannot be nil")
	ValidPrefixRegex = regexp.MustCompile("^[a-z0-9_:-]+$")
)

type IState interface {
	// Get will return the value of the key if it exists; takes optional,
	// additional prefixes that will be appended to the pre-configured prefix.
	Get(ctx context.Context, key string, prefix ...string) (string, error)

	// Add will add the key if it does not exist. If the key already exists, it
	// will return an error; takes optional, additional prefixes that will be
	// appended to the pre-configured prefix.
	Add(ctx context.Context, key, value string, prefix ...string) error

	// Set will overwrite the value if it already exists; takes optional,
	// additional prefixes that will be appended to the pre-configured prefix.
	Set(ctx context.Context, key, value string, prefix ...string) error

	// SetWithTTL will set a key with a specified TTL; takes optional, additional
	// prefixes that will be appended to the pre-configured prefix.
	SetWithTTL(ctx context.Context, key, value string, ttl time.Duration, prefix ...string) error

	// Delete will remove the key from the store; takes optional, additional
	// prefixes that will be appended to the pre-configured prefix.
	Delete(ctx context.Context, key string, prefix ...string) error

	// Exists returns true/false if the key exists in the store; takes optional,
	// additional prefixes that will be appended to the pre-configured prefix.
	Exists(ctx context.Context, key string, prefix ...string) (bool, error)

	// Obtain will obtain a new redis lock with the given key, tll and options
	// to facilitate distributed lock functionality.
	//
	// >> It is the responsibility of the caller to manage the lock lifetime.
	//
	// https://pkg.go.dev/github.com/bsm/redislock
	Obtain(ctx context.Context, key string, ttl time.Duration, opt *redislock.Options, prefix ...string) (*redislock.Lock, error)
}

type State struct {
	opts *Options
	log  clog.ICustomLog
}

type Options struct {
	Prefix      string
	Log         clog.ICustomLog
	RedisClient *redis.Client
	RedisLock   *redislock.Client
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
		return errors.New("options are required")
	}

	if opts.Prefix == "" {
		return errors.New("prefix is required")
	}

	if opts.Log == nil {
		return errors.New("Log is required")
	}

	if opts.RedisClient == nil {
		return errors.New("RedisClient is required")
	}

	if opts.RedisLock == nil {
		return errors.New("RedisLock is required")
	}

	if !ValidPrefixRegex.MatchString(opts.Prefix) {
		return fmt.Errorf("prefix must match '%s' regex", ValidPrefixRegex)
	}

	return nil
}

func (s *State) Get(ctx context.Context, key string, prefix ...string) (string, error) {
	key, err := s.buildKey(key, prefix...)
	if err != nil {
		return "", errors.Wrap(err, "unable to build key")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	data, err := s.opts.RedisClient.Get(ctx, key).Result()
	if err != nil {
		// Redis returns nil if key doesn't exist
		if err == redis.Nil {
			return "", ErrDoesNotExist
		}

		return "", errors.Wrap(err, "unable to get key")
	}

	return data, nil
}

func (s *State) Add(ctx context.Context, key, value string, prefix ...string) error {
	return s.set(ctx, key, value, true, 0, prefix...)
}

func (s *State) Set(ctx context.Context, key, value string, prefix ...string) error {
	return s.set(ctx, key, value, false, 0, prefix...)
}

func (s *State) SetWithTTL(ctx context.Context, key, value string, ttl time.Duration, prefix ...string) error {
	return s.set(ctx, key, value, false, ttl, prefix...)
}

func (s *State) Delete(ctx context.Context, key string, prefix ...string) error {
	key, err := s.buildKey(key, prefix...)
	if err != nil {
		return errors.Wrap(err, "unable to build key")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	if err := s.opts.RedisClient.Del(ctx, key).Err(); err != nil {
		return errors.Wrap(err, "unable to delete key")
	}

	return nil
}

func (s *State) Exists(ctx context.Context, key string, prefix ...string) (bool, error) {
	key, err := s.buildKey(key, prefix...)
	if err != nil {
		return false, errors.Wrap(err, "unable to build key")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	exists, err := s.opts.RedisClient.Exists(ctx, key).Result()
	if err != nil {
		return false, errors.Wrap(err, "unable to check if key exists")
	}

	return exists > 0, nil
}

func (s *State) Obtain(ctx context.Context, key string, ttl time.Duration, opt *redislock.Options, prefix ...string) (*redislock.Lock, error) {
	key, err := s.buildKey(key, prefix...)
	if err != nil {
		return nil, errors.Wrap(err, "unable to build key")
	}

	return s.opts.RedisLock.Obtain(ctx, key, ttl, opt)
}

func (s *State) set(ctx context.Context, key, value string, nx bool, ttl time.Duration, prefix ...string) error {
	key, err := s.buildKey(key, prefix...)
	if err != nil {
		return errors.Wrap(err, "unable to build key")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	if nx {
		err = s.opts.RedisClient.SetNX(ctx, key, value, ttl).Err()
	} else {
		err = s.opts.RedisClient.Set(ctx, key, value, ttl).Err()
	}

	if err != nil {
		return errors.Wrap(err, "unable to set key")
	}

	return nil
}

func (s *State) buildKey(inputKey string, inputPrefix ...string) (string, error) {
	prefix := s.opts.Prefix

	// If we have additional prefixes, append them to the pre-configured prefix
	if len(inputPrefix) > 0 {
		for _, p := range inputPrefix {
			if !ValidPrefixRegex.MatchString(p) {
				return "", fmt.Errorf("invalid additional prefix '%s'", p)
			}

			prefix = prefix + ":" + p
		}
	}

	return prefix + ":" + inputKey, nil
}
