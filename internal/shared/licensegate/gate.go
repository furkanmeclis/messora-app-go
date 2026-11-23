// Package licensegate lets modules ask whether the installation may send.
// The licensing module installs the check at start-up; until then (and in
// tests) everything is allowed.
package licensegate

import (
	"context"
	"sync/atomic"

	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// ErrRestricted is returned while the licence restricts the installation.
var ErrRestricted = apperr.Forbidden("LICENSE_RESTRICTED",
	"Messora is in restricted mode: sending, campaigns and automations are paused until a valid licence is entered in Settings → License.")

var check atomic.Pointer[func(context.Context) bool]

// Install sets the function that reports whether sending is allowed.
func Install(allowed func(context.Context) bool) { check.Store(&allowed) }

// Check returns ErrRestricted when the licence does not allow sending.
func Check(ctx context.Context) error {
	if fn := check.Load(); fn != nil && !(*fn)(ctx) {
		return ErrRestricted
	}
	return nil
}
