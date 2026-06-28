package service

import (
	"testing"
	"time"

	"github.com/integraltech/brainsentry/internal/domain"
)

// fixed reference: Thursday, 2026-06-25 15:00:00 UTC
var refNow = time.Date(2026, 6, 25, 15, 0, 0, 0, time.UTC)

func TestExtractTimeWindow_NoTemporalIntent(t *testing.T) {
	for _, q := range []string{"", "how do I configure the database", "qual a senha do postgres"} {
		if w, ok := ExtractTimeWindow(q, refNow); ok {
			t.Errorf("query %q: expected no window, got %+v", q, w)
		}
	}
}

func TestExtractTimeWindow_Yesterday(t *testing.T) {
	for _, q := range []string{"o que deu errado ontem?", "what failed yesterday"} {
		w, ok := ExtractTimeWindow(q, refNow)
		if !ok {
			t.Fatalf("query %q: expected a window", q)
		}
		wantFrom := time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC)
		wantTo := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
		if !w.From.Equal(wantFrom) || !w.To.Equal(wantTo) {
			t.Errorf("query %q: got [%v, %v], want [%v, %v]", q, w.From, w.To, wantFrom, wantTo)
		}
	}
}

func TestExtractTimeWindow_Today(t *testing.T) {
	w, ok := ExtractTimeWindow("erros de hoje", refNow)
	if !ok {
		t.Fatal("expected a window for 'hoje'")
	}
	wantFrom := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	if !w.From.Equal(wantFrom) || !w.To.Equal(refNow) {
		t.Errorf("got [%v, %v], want [%v, %v]", w.From, w.To, wantFrom, refNow)
	}
}

func TestExtractTimeWindow_LastNDays(t *testing.T) {
	w, ok := ExtractTimeWindow("incidentes nos últimos 7 dias", refNow)
	if !ok {
		t.Fatal("expected a window for 'últimos 7 dias'")
	}
	if want := refNow.AddDate(0, 0, -7); !w.From.Equal(want) {
		t.Errorf("From = %v, want %v", w.From, want)
	}
	if !w.To.Equal(refNow) {
		t.Errorf("To = %v, want %v", w.To, refNow)
	}
}

func TestExtractTimeWindow_LastNHours(t *testing.T) {
	w, ok := ExtractTimeWindow("what broke in the last 3 hours", refNow)
	if !ok {
		t.Fatal("expected a window for 'last 3 hours'")
	}
	if want := refNow.Add(-3 * time.Hour); !w.From.Equal(want) {
		t.Errorf("From = %v, want %v", w.From, want)
	}
}

func TestExtractTimeWindow_LastWeekAndMonth(t *testing.T) {
	wk, ok := ExtractTimeWindow("deploys da semana passada", refNow)
	if !ok || !wk.From.Equal(refNow.AddDate(0, 0, -7)) {
		t.Errorf("last week: ok=%v window=%+v", ok, wk)
	}
	mo, ok := ExtractTimeWindow("o que mudou no último mês", refNow)
	if !ok || !mo.From.Equal(refNow.AddDate(0, -1, 0)) {
		t.Errorf("last month: ok=%v window=%+v", ok, mo)
	}
}

func TestExtractTimeWindow_HoursTakePrecedenceOverGenericMatch(t *testing.T) {
	// "last 24 hours" must resolve to an hour window, not be swallowed by a
	// looser word match.
	w, ok := ExtractTimeWindow("alerts in the last 24 hours", refNow)
	if !ok {
		t.Fatal("expected window")
	}
	if want := refNow.Add(-24 * time.Hour); !w.From.Equal(want) {
		t.Errorf("From = %v, want %v", w.From, want)
	}
}

func TestMergeMemoriesByID_Dedupes(t *testing.T) {
	base := []domain.Memory{{ID: "a"}, {ID: "b"}}
	extra := []domain.Memory{{ID: "b"}, {ID: "c"}, {ID: "a"}, {ID: "d"}}

	got := mergeMemoriesByID(base, extra)

	var ids []string
	for _, m := range got {
		ids = append(ids, m.ID)
	}
	want := []string{"a", "b", "c", "d"}
	if len(ids) != len(want) {
		t.Fatalf("got %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Errorf("position %d: got %q, want %q (full: %v)", i, ids[i], want[i], ids)
		}
	}
}

func TestMergeMemoriesByID_EmptyExtra(t *testing.T) {
	base := []domain.Memory{{ID: "a"}}
	got := mergeMemoriesByID(base, nil)
	if len(got) != 1 || got[0].ID != "a" {
		t.Errorf("expected base unchanged, got %v", got)
	}
}
