package log

import (
	"context"
	"log/slog"
)

// Structured logging field names, shared across internal/handlers so the
// same concept (e.g. "the resource's own ID") always logs under the same
// key regardless of which handler emits the line.
const (
	Error          = "error"
	KeyboardID     = "keyboard_id"
	KeyboardPlate  = "keyboard_plate_id"
	KeyboardPCB    = "keyboard_pcb_id"
	SwitchID       = "switch_id"
	ECPartID       = "ec_part_id"
	KeycapSetID    = "keycap_set_id"
	KeycapKitID    = "keycap_kit_id"
	KeycapKitImage = "keycap_kit_image"
	BuildID        = "build_id"
	ProfileID      = "profile_id"
	LookupCategory = "lookup_category"
	OwnerID        = "owner_id"
	Visibility     = "visibility"
)

type loggerKey struct{}

// New returns a base logger with no request-specific fields yet.
func New() *slog.Logger {
	return slog.Default()
}

// WithRequestID returns l with a request_id field added.
func WithRequestID(l *slog.Logger, requestID string) *slog.Logger {
	return l.With("request_id", requestID)
}

// WithUserID returns l with a user_id field added.
func WithUserID(l *slog.Logger, userID string) *slog.Logger {
	return l.With("user_id", userID)
}

// WithLogger returns a context carrying l as the request-scoped logger.
func WithLogger(c context.Context, l *slog.Logger) context.Context {
	return context.WithValue(c, loggerKey{}, l)
}

// FromContext returns the request-scoped logger stored by WithLogger, or the
// default logger if none is present (e.g. outside a request, in tests).
func FromContext(c context.Context) *slog.Logger {
	if l, ok := c.Value(loggerKey{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

// DeniedRead records a read denied by authorization. Callers report it as
// not-found, so this line is the only thing telling an operator the item
// existed - without it an authz regression looks like a missing row. Info,
// not Error: a denial is expected traffic.
func DeniedRead(c context.Context, resource, ownerID, visibility, idKey, idValue string) {
	FromContext(c).Info("read denied by visibility, reporting as not found",
		"resource", resource,
		idKey, idValue,
		OwnerID, ownerID,
		Visibility, visibility,
	)
}
