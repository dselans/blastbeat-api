package processor

import (
	"time"

	"github.com/pkg/errors"
	"github.com/superpowerdotcom/events/build/proto/go/common"
	"github.com/superpowerdotcom/go-lib-common/validate"
	"go.uber.org/zap"

	"github.com/your_org/go-svc-template/backends/cache"
	"github.com/your_org/go-svc-template/backends/state"
)

func (p *Processor) handleUserCreated(event *common.Event) error {
	logger := p.log.With(zap.String("method", "handleUserCreatedEvent"))

	userCreated := event.GetUserCreated()

	logger.Debug("Validating user created event")

	if err := validate.UserCreatedEvent(userCreated); err != nil {
		logger.Error("failed to validate user created event", zap.Error(err))
		return errors.Wrap(err, "failed to validate user created event")
	}

	logger = logger.With(zap.String("id", userCreated.User.Id))

	logger.Debug("Writing user to cache")

	// Update cache for 5s
	p.options.Cache.Set(cache.UserPrefix+":"+userCreated.User.Id, userCreated.User, 5*time.Second)

	// Update global state
	//
	// NOTE: This is good and meh - all replicas will attempt to update the state.
	// It's good because if one replica fails, another one will succeed (probably).
	// It's meh because it is wasteful for all replicas to perform writes.
	if err := p.options.StateService.AddUser(p.options.ShutdownCtx, userCreated.User); err != nil {
		// Do not fail if user already exists - another replica may have already
		// added the user.
		if errors.Is(err, state.ErrAlreadyExists) {
			logger.Debug("user already exists in global state - skipping")
			return nil
		}

		logger.Error("failed to add user in global state", zap.Error(err))
		return errors.Wrap(err, "failed to add user in global state")
	}

	return nil
}

func (p *Processor) handleUserUpdated(event *common.Event) error {
	logger := p.log.With(zap.String("method", "handleUserUpdatedEvent"))
	logger.Debug("received user updated event")

	userUpdated := event.GetUserUpdated()

	logger.Debug("Validating user.updated event")

	if err := validate.UserUpdatedEvent(userUpdated); err != nil {
		logger.Error("failed to validate user updated event", zap.Error(err))
		return errors.Wrap(err, "failed to validate user updated event")
	}

	logger = logger.With(zap.String("id", userUpdated.User.Id))

	logger.Debug("Updating user in cache")

	// Update cache for 5s
	p.options.Cache.Set(cache.UserPrefix+":"+userUpdated.User.Id, userUpdated.User, 5*time.Second)

	// Update global state
	//
	// NOTE: This is good and meh - all replicas will attempt to update the state.
	// It's good because if one replica fails, another one will succeed (probably).
	// It's meh because it is wasteful for all replicas to perform writes.
	if err := p.options.StateService.SetUser(p.options.ShutdownCtx, userUpdated.User); err != nil {
		logger.Error("failed to update user in global state", zap.Error(err))
		return errors.Wrap(err, "failed to update user in global state")
	}

	return nil
}

func (p *Processor) handleUserDeleted(event *common.Event) error {
	logger := p.log.With(zap.String("method", "handleUserDeletedEvent"))

	userDeleted := event.GetUserDeleted()

	logger.Debug("Validating user.deleted event")

	if err := validate.UserDeletedEvent(userDeleted); err != nil {
		logger.Error("failed to validate user deleted event", zap.Error(err))
		return errors.Wrap(err, "failed to validate user deleted event")
	}

	logger = logger.With(zap.String("id", userDeleted.User.Id))

	logger.Debug("Removing user from cache")

	// Remove cache (if exists)
	p.options.Cache.Remove(cache.UserPrefix + ":" + userDeleted.User.Id)

	// Update global state
	if err := p.options.StateService.DeleteUser(p.options.ShutdownCtx, userDeleted.User.Id); err != nil {
		// TODO: Ignore error if user does not exist - another replace may have
		// deleted it already.

		logger.Error("failed to delete user from global state", zap.Error(err))
		return errors.Wrap(err, "failed to delete user from global state")
	}

	return nil
}
