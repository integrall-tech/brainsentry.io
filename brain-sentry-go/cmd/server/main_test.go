package main

import (
	"strings"
	"testing"
)

func TestSplitNonEmpty_HandlesWhitespaceAndDoubles(t *testing.T) {
	cases := map[string][]string{
		"":                       nil,
		"graph":                  {"graph"},
		"graph,embeddings":       {"graph", "embeddings"},
		"graph, embeddings":      {"graph", "embeddings"},
		"graph,,embeddings":      {"graph", "embeddings"},
		" graph , embeddings , ": {"graph", "embeddings"},
	}
	for in, want := range cases {
		got := splitNonEmpty(in, ",")
		if len(got) != len(want) {
			t.Errorf("for %q: expected %d parts; got %d (%v)", in, len(want), len(got), got)
			continue
		}
		for i, w := range want {
			if got[i] != w {
				t.Errorf("for %q: at %d expected %q; got %q", in, i, w, got[i])
			}
		}
	}
}

func TestSplitNonEmpty_DifferentSeparator(t *testing.T) {
	got := splitNonEmpty("a|b|c", "|")
	if strings.Join(got, ",") != "a,b,c" {
		t.Errorf("expected a,b,c; got %v", got)
	}
}
