package trust

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestFromContext_DefaultsToRemoteFailClosed(t *testing.T) {
	if got := FromContext(context.Background()); got != Remote {
		t.Errorf("expected Remote default; got %s", got)
	}
}

func TestWithLocal_SetsLocal(t *testing.T) {
	ctx := WithLocal(context.Background())
	if FromContext(ctx) != Local {
		t.Errorf("expected Local; got %s", FromContext(ctx))
	}
}

func TestWithSubagent_SetsSubagent(t *testing.T) {
	ctx := WithSubagent(context.Background())
	if FromContext(ctx) != Subagent {
		t.Errorf("expected Subagent; got %s", FromContext(ctx))
	}
}

func TestLevels_Order(t *testing.T) {
	if !Local.AtLeast(Subagent) || !Local.AtLeast(Remote) {
		t.Errorf("Local must be the highest level")
	}
	if !Subagent.AtLeast(Remote) || Subagent.AtLeast(Local) {
		t.Errorf("Subagent must be > Remote and < Local")
	}
	if Remote.AtLeast(Subagent) {
		t.Errorf("Remote must be lowest")
	}
}

func TestRequire_PassesWhenAtOrAboveMin(t *testing.T) {
	ctx := WithLocal(context.Background())
	if err := Require(ctx, Local); err != nil {
		t.Errorf("Local should pass Local require; got %v", err)
	}
	if err := Require(ctx, Subagent); err != nil {
		t.Errorf("Local should pass Subagent require; got %v", err)
	}
	if err := Require(ctx, Remote); err != nil {
		t.Errorf("Local should pass Remote require; got %v", err)
	}
}

func TestRequire_RefusesWhenBelowMin(t *testing.T) {
	err := Require(context.Background(), Local)
	if err == nil {
		t.Fatalf("expected error for Remote ctx asking Local")
	}
	if !errors.Is(err, ErrNotPermitted) {
		t.Errorf("expected ErrNotPermitted sentinel; got %v", err)
	}
	if !strings.Contains(err.Error(), "requires local") || !strings.Contains(err.Error(), "caller is remote") {
		t.Errorf("expected explanation in message; got %q", err)
	}
}

func TestWithRemote_ExplicitFlagSurvivesShadowing(t *testing.T) {
	ctx := WithLocal(context.Background())
	ctx = WithRemote(ctx)
	if FromContext(ctx) != Remote {
		t.Errorf("WithRemote should overwrite an upstream Local; got %s", FromContext(ctx))
	}
}

func TestLevel_StringStable(t *testing.T) {
	cases := map[Level]string{Local: "local", Subagent: "subagent", Remote: "remote"}
	for lvl, want := range cases {
		if lvl.String() != want {
			t.Errorf("expected %q for %d; got %q", want, lvl, lvl.String())
		}
	}
	if !strings.HasPrefix(Level(99).String(), "unknown") {
		t.Errorf("unknown level should print 'unknown(...)'; got %q", Level(99).String())
	}
}
