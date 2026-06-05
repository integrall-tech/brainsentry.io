package service

import "testing"

func TestParseResolutionAction_Valid(t *testing.T) {
	cases := map[string]ResolutionAction{
		"supersede": ResolveSupersede,
		"dismiss":   ResolveDismiss,
	}
	for in, want := range cases {
		got, err := ParseResolutionAction(in)
		if err != nil {
			t.Errorf("%q: unexpected error %v", in, err)
		}
		if got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestParseResolutionAction_Invalid(t *testing.T) {
	for _, in := range []string{"", "SUPERSEDE", "keep", "merge", "delete"} {
		if _, err := ParseResolutionAction(in); err == nil {
			t.Errorf("%q should be rejected", in)
		}
	}
}
