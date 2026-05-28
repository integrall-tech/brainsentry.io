// Package trust models the **trust level** of the caller behind every
// operation. Where memory came from is not enough — what the caller is
// allowed to *do* depends on whether they are the operator at the CLI
// (trusted), an authenticated agent over MCP/HTTP (untrusted-by-default),
// or a sub-agent the system itself spawned (limited).
//
// The contract:
//
//   - Every entrypoint that turns an external request into a context.Context
//     is responsible for setting the trust level. CLI sets Local. MCP/HTTP
//     handlers set Remote (default). Subagent dispatch sets Subagent.
//   - Sensitive operations (destructive deletes, bulk imports, shell-style
//     tool dispatch) call Require(ctx, trust.Local) and refuse otherwise.
//   - Reading the trust level returns Remote when nothing is set —
//     fail-closed by default. Operations that don't care never have to ask.
//
// Inspired by gbrain's OperationContext.remote (TypeScript). The TypeScript
// version learned this the hard way after v0.26.9 patched an RCE where an
// HTTP handler forgot to set remote=true; making the field required-via-type
// in TS would have caught it at compile time. We do the same in Go via
// context typing — the helpers FromContext returns Remote unless someone
// explicitly opted in, so a forgotten WithLocal stays safe.
package trust

import (
	"context"
	"errors"
	"fmt"
)

// Level enumerates the trust classes. Order is meaningful: Local > Subagent
// > Remote. Higher levels are *more* trusted.
type Level int

const (
	// Remote — caller arrived over the network (MCP, HTTP, webhook). Lowest
	// trust. Default when no level is explicitly set on the context.
	Remote Level = iota
	// Subagent — code path executed on behalf of the operator by an
	// internal sub-agent (e.g. a scheduled cycle, a tool-calling minion).
	// Trusted enough to read but cautious about writes.
	Subagent
	// Local — operator at the CLI / TUI / a fully authenticated admin over
	// localhost. Highest trust.
	Local
)

// String returns the canonical name for logs / telemetry.
func (l Level) String() string {
	switch l {
	case Local:
		return "local"
	case Subagent:
		return "subagent"
	case Remote:
		return "remote"
	default:
		return fmt.Sprintf("unknown(%d)", int(l))
	}
}

// AtLeast reports whether l is at or above min on the trust scale.
func (l Level) AtLeast(min Level) bool { return l >= min }

type contextKey struct{}

// With returns a new context tagged with the trust level.
func With(ctx context.Context, l Level) context.Context {
	return context.WithValue(ctx, contextKey{}, l)
}

// WithLocal is a convenience for the most-trusted entry path (CLI / TUI).
func WithLocal(ctx context.Context) context.Context { return With(ctx, Local) }

// WithSubagent is a convenience for internally-spawned sub-agent dispatch.
func WithSubagent(ctx context.Context) context.Context { return With(ctx, Subagent) }

// WithRemote is a convenience for explicitly tagging an untrusted path.
// Mostly cosmetic — the absence of any tag is already treated as Remote —
// but the explicit form is recommended at MCP/HTTP boundaries so an audit
// can grep for it.
func WithRemote(ctx context.Context) context.Context { return With(ctx, Remote) }

// FromContext returns the level set on ctx, or Remote when nothing is set
// (fail-closed default).
func FromContext(ctx context.Context) Level {
	if v, ok := ctx.Value(contextKey{}).(Level); ok {
		return v
	}
	return Remote
}

// ErrNotPermitted is returned by Require when the caller's trust level is
// below the required minimum. Callers can errors.Is against this sentinel
// to translate to HTTP 403 / domain.ErrForbidden.
var ErrNotPermitted = errors.New("operation not permitted at this trust level")

// Require checks the level on ctx is at or above min. Returns nil on pass,
// ErrNotPermitted otherwise, with a message that names what was required vs
// what was offered (useful for audit logs).
func Require(ctx context.Context, min Level) error {
	got := FromContext(ctx)
	if got.AtLeast(min) {
		return nil
	}
	return fmt.Errorf("%w: requires %s, caller is %s", ErrNotPermitted, min, got)
}
