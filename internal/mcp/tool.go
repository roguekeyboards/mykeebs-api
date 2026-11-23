package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/roguekeyboards/mykeebs-api/internal/authz"
	ctxpkg "github.com/roguekeyboards/mykeebs-api/internal/ctx"
	"github.com/roguekeyboards/mykeebs-api/internal/log"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
	"github.com/roguekeyboards/mykeebs-api/internal/uploadlimit"
)

// defaultListLimit is api/openapi.yaml's Limit param default.
const defaultListLimit = 20

// errNoCallerIdentity is unreachable while requireBearerToken gates every
// MCP request (see server.go), but fails closed rather than defaulting to
// an empty owner ID if that wiring ever changes.
var errNoCallerIdentity = errors.New("no caller identity on context")

func resolveOwnerID(ctx context.Context, userID string) (string, error) {
	if id := strings.TrimSpace(userID); id != "" {
		return id, nil
	}

	subject, err := callerSubject(ctx)
	if err != nil {
		log.FromContext(ctx).Error("MCP tool ran with no caller identity on context")
		return "", err
	}

	return subject, nil
}

func callerSubject(ctx context.Context) (string, error) {
	subject, ok := ctxpkg.UserID(ctx)
	if !ok || subject == "" {
		return "", errNoCallerIdentity
	}

	return subject, nil
}

func callerPreferences(ctx context.Context, prefs repository.PreferencesReader) (repository.ProfilePreferences, error) {
	subject, err := callerSubject(ctx)
	if err != nil {
		return repository.ProfilePreferences{}, err
	}

	return prefs.GetPreferences(ctx, subject)
}

// ownedReadable resolves ownerID (defaulting to the caller when userID is
// blank), fetches the entity with id via get, and enforces the 404-not-403
// convention shared by every get_* MCP tool: notFoundErr both when the
// entity genuinely doesn't exist and when the caller can't read it, so a
// visibility-denied entity isn't distinguishable from a nonexistent one.
// Mirrors the equivalent per-handler logic in each entity's REST handler
// (e.g. [github.com/roguekeyboards/mykeebs-api/internal/handlers.GetKeyboard]).
func ownedReadable[T any](
	ctx context.Context,
	get func(ctx context.Context, ownerID, id string) (*T, error),
	visibilityOf func(T) repository.Visibility,
	entityName string,
	notFoundErr error,
	idLogField string,
	userID, id string,
) (*T, error) {
	ownerID, err := resolveOwnerID(ctx, userID)
	if err != nil {
		return nil, err
	}

	item, err := get(ctx, ownerID, id)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, notFoundErr
	}
	if err != nil {
		log.FromContext(ctx).Error("getting "+entityName, idLogField, id, log.Error, err)
		return nil, errors.New("failed to get " + entityName)
	}

	if !authz.CanReadVisibility(ctx, ownerID, visibilityOf(*item)) {
		log.DeniedRead(ctx, entityName, ownerID, string(visibilityOf(*item)), idLogField, id)
		return nil, notFoundErr
	}

	return item, nil
}

// errMutationNotFound, errMutationConflict, and errMutationFailed are
// returned by handleMutationError and handleTransactionError. Generic rather than per-entity: the
// calling tool is already known to whatever invoked it (unlike REST, MCP
// has no separate URL/status side channel, but the tool name itself serves
// the same role), so an entity name in the message would be redundant.
var (
	errMutationNotFound = errors.New("resource not found")
	errMutationConflict = errors.New("the resource is being modified concurrently, please retry")
	errMutationFailed   = errors.New("failed to mutate resource")
)

// errImageLimitReached is handleMutationError's mapping for
// repository.ErrImageLimitReached.
var errImageLimitReached = fmt.Errorf("the item already has %d images, the maximum; delete one first", repository.MaxImagesPerItem)

// handleMutationError is the standard error tail for a mutating repository
// call: repository.ErrNotFound -> errMutationNotFound, repository.ErrImageLimitReached -> errImageLimitReached, any other non-nil error -> errMutationFailed (logged as an error). logFields are passed through to the log call for correlation
// (e.g. log.SwitchID, id). Returns nil if err was nil.
func handleMutationError(ctx context.Context, err error, logFields ...any) error {
	if errors.Is(err, repository.ErrNotFound) {
		return errMutationNotFound
	}
	if errors.Is(err, repository.ErrImageLimitReached) {
		return errImageLimitReached
	}
	if err != nil {
		log.FromContext(ctx).Error("mutation failed", append([]any{log.Error, err}, logFields...)...)
		return errMutationFailed
	}
	return nil
}

// errUploadsUnavailable is the tool error for repository.ErrUploadLimitReached.
var errUploadsUnavailable = errors.New("image uploads aren't available at the moment")

// reserveUpload counts one upload against the daily limit before an upload
// URL is presigned: repository.ErrUploadLimitReached ->
// errUploadsUnavailable, any other error -> errMutationFailed (logged as an
// error). Returns nil if the upload was reserved.
func reserveUpload(ctx context.Context, uploads *uploadlimit.Limiter, logFields ...any) error {
	err := uploads.Reserve(ctx)
	if errors.Is(err, repository.ErrUploadLimitReached) {
		log.FromContext(ctx).Warn("daily upload limit reached", logFields...)
		return errUploadsUnavailable
	}
	if err != nil {
		log.FromContext(ctx).Error("reserving upload", append([]any{log.Error, err}, logFields...)...)
		return errMutationFailed
	}
	return nil
}

// handleTransactionError is handleMutationError for the repository calls
// that write a transaction and can return repository.ErrMutationConflict,
// which it maps to errMutationConflict (logged as a warning).
func handleTransactionError(ctx context.Context, err error, logFields ...any) error {
	if errors.Is(err, repository.ErrMutationConflict) {
		log.FromContext(ctx).Warn("mutation conflict", logFields...)
		return errMutationConflict
	}
	return handleMutationError(ctx, err, logFields...)
}

// handleClearImageError is handleMutationError's counterpart for the
// image-pointer clear that follows a successful S3 object delete in a
// single-image delete handler. The S3 object is already gone by this
// point, so repository.ErrNotFound (the entity was deleted concurrently)
// is the documented idempotent-success state, not errMutationNotFound: it's
// swallowed rather than routed through handleMutationError. Other errors
// still go through handleMutationError unchanged.
func handleClearImageError(ctx context.Context, err error, logFields ...any) error {
	if errors.Is(err, repository.ErrNotFound) {
		return nil
	}
	return handleMutationError(ctx, err, logFields...)
}

func listLimit(limit *int) int {
	if limit == nil {
		return defaultListLimit
	}
	return *limit
}
