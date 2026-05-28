package recency

import (
	"math"
	"testing"
	"time"
)

func TestPolicyForPath_LongestPrefixWins(t *testing.T) {
	cfg := Config{
		Default: PrefixPolicy{HalflifeDays: 100, Coefficient: 1},
		Prefixes: map[string]PrefixPolicy{
			"daily/":          {HalflifeDays: 14, Coefficient: 1.5},
			"daily/standup/":  {HalflifeDays: 3, Coefficient: 2},
			"weekly/":         {HalflifeDays: 30, Coefficient: 1.2},
		},
	}
	if p := cfg.PolicyForPath("daily/standup/2026-05-14"); p.HalflifeDays != 3 {
		t.Errorf("expected longest-prefix policy halflife 3; got %v", p.HalflifeDays)
	}
	if p := cfg.PolicyForPath("daily/notes/foo"); p.HalflifeDays != 14 {
		t.Errorf("expected daily/ policy; got %v", p.HalflifeDays)
	}
	if p := cfg.PolicyForPath("misc/blah"); p.HalflifeDays != 100 {
		t.Errorf("expected default policy; got %v", p.HalflifeDays)
	}
}

func TestFactor_AtHalflifeIsHalf(t *testing.T) {
	p := PrefixPolicy{HalflifeDays: 14, Coefficient: 1}
	now := time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC)
	rec := now.AddDate(0, 0, -14)
	got := Factor(p, rec, now)
	if math.Abs(got-0.5) > 0.001 {
		t.Errorf("expected ~0.5 at halflife; got %v", got)
	}
}

func TestFactor_FreshIsOne(t *testing.T) {
	p := PrefixPolicy{HalflifeDays: 14, Coefficient: 1}
	now := time.Now()
	if got := Factor(p, now, now); got != 1 {
		t.Errorf("expected 1 for now; got %v", got)
	}
}

func TestFactor_OldDecaysClose0(t *testing.T) {
	p := PrefixPolicy{HalflifeDays: 14, Coefficient: 1}
	now := time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC)
	rec := now.AddDate(-1, 0, 0)
	got := Factor(p, rec, now)
	if got >= 0.001 {
		t.Errorf("expected near-zero for one year on 14-day halflife; got %v", got)
	}
}

func TestFactor_HalflifeZeroIsEvergreen(t *testing.T) {
	p := PrefixPolicy{HalflifeDays: 0, Coefficient: 0}
	now := time.Now()
	rec := now.AddDate(-10, 0, 0)
	if got := Factor(p, rec, now); got != 1 {
		t.Errorf("expected 1 for evergreen; got %v", got)
	}
}

func TestFactor_CoefficientAcceleratesDecay(t *testing.T) {
	now := time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC)
	rec := now.AddDate(0, 0, -14)
	slow := Factor(PrefixPolicy{HalflifeDays: 14, Coefficient: 1}, rec, now)
	fast := Factor(PrefixPolicy{HalflifeDays: 14, Coefficient: 2}, rec, now)
	if !(fast < slow) {
		t.Errorf("expected coefficient=2 to decay faster than =1; slow=%v fast=%v", slow, fast)
	}
}

func TestFreshnessHint_Detects(t *testing.T) {
	cases := []string{
		"what happened today",
		"latest decisions on auth",
		"what was last week",
		"recent incidents",
		"o que decidimos hoje",  // pt-BR
		"problemas recentes",
		"esta semana",
		"o que está acontecendo agora",
	}
	for _, q := range cases {
		if !FreshnessHint(q) {
			t.Errorf("expected freshness hint detected in %q", q)
		}
	}
}

func TestFreshnessHint_BenignNoMatch(t *testing.T) {
	cases := []string{
		"what is postgres",
		"why does this fail",
		"how do I rotate keys",
	}
	for _, q := range cases {
		if FreshnessHint(q) {
			t.Errorf("expected no freshness hint in %q", q)
		}
	}
}

func TestApplyAutoDetect_AmplifiesCoefficient(t *testing.T) {
	p := PrefixPolicy{HalflifeDays: 14, Coefficient: 1}
	out := ApplyAutoDetect("what was decided today", p)
	if out.Coefficient <= p.Coefficient {
		t.Errorf("expected coefficient bumped; got %v", out.Coefficient)
	}
}

func TestApplyAutoDetect_NoSignalNoChange(t *testing.T) {
	p := PrefixPolicy{HalflifeDays: 14, Coefficient: 1}
	out := ApplyAutoDetect("what is postgres", p)
	if out != p {
		t.Errorf("expected unchanged policy; got %+v", out)
	}
}

func TestApplyAutoDetect_EvergreenGetsSoftRecency(t *testing.T) {
	p := PrefixPolicy{HalflifeDays: 0, Coefficient: 0}
	out := ApplyAutoDetect("latest concept", p)
	if out.HalflifeDays == 0 {
		t.Errorf("expected evergreen to get a soft tilt when 'latest' is in query; got %+v", out)
	}
}

func TestCompose_Multiplies(t *testing.T) {
	got := Compose(0.8, 0.5, 1.2)
	want := 0.8 * 0.5 * 1.2
	if math.Abs(got-want) > 0.0001 {
		t.Errorf("expected %v; got %v", want, got)
	}
}

func TestPolicySnapshot_DefaultFirst(t *testing.T) {
	rows := DefaultConfig.PolicySnapshot()
	if rows[0].Prefix != "(default)" {
		t.Errorf("expected default first; got %s", rows[0].Prefix)
	}
}

func TestPolicySnapshot_SortedAfterDefault(t *testing.T) {
	rows := DefaultConfig.PolicySnapshot()
	for i := 2; i < len(rows); i++ {
		if rows[i].Prefix < rows[i-1].Prefix {
			t.Errorf("not sorted: %s < %s", rows[i].Prefix, rows[i-1].Prefix)
		}
	}
}
