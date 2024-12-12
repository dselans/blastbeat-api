package api

// Example new_user_handler.go. Shows how to parse request body, unmarshal,
// validate, check state, emit event to bus.

//package api
//
//import (
//	"io"
//	"net/http"
//
//	"github.com/pkg/errors"
//	r3labs "github.com/r3labs/diff/v3"
//	"github.com/superpowerdotcom/events/build/proto/go/user"
//	"go.uber.org/zap"
//	"google.golang.org/protobuf/encoding/protojson"
//
//	sb "github.com/superpowerdotcom/go-svc-template/backends/state"
//	""github.com/superpowerdotcom/go-lib-common/validate"
//)
//
//type NewUserRequest struct {
//	// This should be a JSON representation of the user.User structure
//	*user.User
//}
//
//func (a *API) newUserHandler(rw http.ResponseWriter, r *http.Request) {
//	// Parse request
//	data, err := io.ReadAll(r.Body)
//	if err != nil {
//		WriteJSON(rw, ResponseJSON{
//			Status:  http.StatusInternalServerError,
//			Message: "failed to read request body: " + err.Error(),
//		}, http.StatusInternalServerError)
//
//		return
//	}
//	defer r.Body.Close()
//
//	newUser := &user.User{}
//
//	if err := protojson.Unmarshal(data, newUser); err != nil {
//		WriteJSON(rw, ResponseJSON{
//			Status:  http.StatusBadRequest,
//			Message: "failed to parse request into user: " + err.Error(),
//		}, http.StatusBadRequest)
//
//		return
//	}
//
//	if err := validate.User(newUser); err != nil {
//		WriteJSON(rw, ResponseJSON{
//			Status:  http.StatusBadRequest,
//			Message: "invalid request: " + err.Error(),
//		}, http.StatusBadRequest)
//
//		return
//	}
//
//	// Check if user is in global state
//	existingUserEntry, err := a.deps.StateService.GetUser(r.Context(), newUser.Id)
//	if err != nil {
//		if errors.Is(err, sb.ErrDoesNotExist) {
//			a.log.Debug("user not found - emitting user.Created event", zap.String("email", newUser.Email))
//
//			if err := a.deps.PublisherService.PublishUserCreatedEvent(r.Context(), newUser); err != nil {
//				a.log.Error("failed to publish new user event", zap.Error(err))
//
//				WriteJSON(rw, ResponseJSON{
//					Status:  http.StatusInternalServerError,
//					Message: "failed to emit new user event",
//				}, http.StatusInternalServerError)
//
//				return
//			}
//
//			WriteJSON(rw, ResponseJSON{
//				Status:  http.StatusOK,
//				Message: "user not found - emitted user.Created event",
//			}, http.StatusOK)
//
//			return
//		}
//
//		// It's definitely an error
//		a.log.Error("failed to get user", zap.Error(err))
//
//		WriteJSON(rw, ResponseJSON{
//			Status:  http.StatusInternalServerError,
//			Message: "failed to get user",
//		}, http.StatusInternalServerError)
//
//		return
//	}
//
//	// We have a user record - let's make sure it contains valid data. If not,
//	// raise an alarm.
//	if err := validate.User(existingUserEntry); err != nil {
//		// TODO: Raise an error in NR
//		a.log.Error("existing user record validation failed",
//			zap.Error(err),
//			zap.String("userId", existingUserEntry.Id),
//		)
//
//		WriteJSON(rw, ResponseJSON{
//			Status:  http.StatusOK,
//			Message: "existing user record validation failed: " + err.Error(),
//		}, http.StatusInternalServerError)
//
//		return
//	}
//
//	// Diff user entries - if ours is different, update global state + emit
//	// updated event for other services.
//	changelog, err := r3labs.Diff(existingUserEntry, newUser)
//	if err != nil {
//		a.log.Error("failed to diff user entries",
//			zap.Error(err),
//			zap.String("userId", existingUserEntry.Id),
//		)
//
//		WriteJSON(rw, ResponseJSON{
//			Status:  http.StatusInternalServerError,
//			Message: "failed to diff existing VS new user entries",
//		}, http.StatusInternalServerError)
//	}
//
//	if len(changelog) > 0 {
//		if err := a.deps.PublisherService.PublishUserUpdatedEvent(r.Context(), newUser); err != nil {
//			a.log.Error("failed to publish updated user event", zap.Error(err))
//
//			WriteJSON(rw, ResponseJSON{
//				Status:  http.StatusInternalServerError,
//				Message: "failed to publish updated user event",
//			}, http.StatusInternalServerError)
//
//			return
//		}
//
//		WriteJSON(rw, ResponseJSON{
//			Status:  http.StatusOK,
//			Message: "user updated",
//		}, http.StatusOK)
//	}
//
//	// User exists, nothing left to do
//	WriteJSON(rw, ResponseJSON{
//		Status:  http.StatusOK,
//		Message: "user already exists",
//	}, http.StatusOK)
//}
