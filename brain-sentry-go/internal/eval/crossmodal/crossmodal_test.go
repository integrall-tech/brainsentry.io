package crossmodal

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- RepairJSON ---

func TestRepairJSON_PlainObject(t *testing.T) {
	got, err := RepairJSON(`{"a":1}`)
	if err != nil || string(got) != `{"a":1}` {
		t.Errorf("plain object should pass through; got %s err=%v", got, err)
	}
}

func TestRepairJSON_StripsCodeFences(t *testing.T) {
	in := "```json\n{\"a\":1}\n```"
	got, err := RepairJSON(in)
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	if !strings.Contains(string(got), `"a":1`) {
		t.Errorf("expected stripped JSON; got %s", got)
	}
}

func TestRepairJSON_StripsBareFences(t *testing.T) {
	in := "```\n{\"a\":1}\n```"
	got, _ := RepairJSON(in)
	if !strings.Contains(string(got), `"a":1`) {
		t.Errorf("expected stripped JSON; got %s", got)
	}
}

func TestRepairJSON_ExtractsFirstObjectFromProse(t *testing.T) {
	in := `Sure, here is your json: {"scores":[{"dim":"correctness","value":9}]}  let me know!`
	got, err := RepairJSON(in)
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	if !strings.HasPrefix(string(got), `{"scores"`) {
		t.Errorf("expected object extracted; got %s", got)
	}
}

func TestRepairJSON_RemovesTrailingComma(t *testing.T) {
	in := `{"a":1,}`
	got, err := RepairJSON(in)
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	if string(got) == in {
		t.Errorf("expected trailing-comma fix")
	}
}

func TestRepairJSON_UnparseableReturnsErr(t *testing.T) {
	_, err := RepairJSON(`not even close`)
	if err == nil {
		t.Errorf("expected error for garbage input")
	}
}

// --- ParseJudgement ---

func TestParseJudgement_WellFormed(t *testing.T) {
	raw := `{"scores":[{"dim":"correctness","value":9,"comment":"solid"}]}`
	j := ParseJudgement("gpt-4o", raw)
	if !j.OK {
		t.Fatalf("expected OK; got %+v", j)
	}
	if len(j.Scores) != 1 || j.Scores[0].Dim != "correctness" || j.Scores[0].Value != 9 {
		t.Errorf("scores mis-parsed: %+v", j.Scores)
	}
}

func TestParseJudgement_NoScoresMarksFail(t *testing.T) {
	j := ParseJudgement("gpt", `{"scores":[]}`)
	if j.OK {
		t.Errorf("empty scores should be OK=false")
	}
}

func TestParseJudgement_UnparseableMarksFail(t *testing.T) {
	j := ParseJudgement("gpt", `nope`)
	if j.OK || !strings.Contains(j.Detail, "unparseable") {
		t.Errorf("expected unparseable failure; got %+v", j)
	}
}

// --- Aggregate ---

func mkJudge(model string, ok bool, scores map[Dimension]int) Judgement {
	var s []Score
	for d, v := range scores {
		s = append(s, Score{Dim: d, Value: v})
	}
	return Judgement{Model: model, OK: ok, Scores: s}
}

func TestAggregate_PassPath(t *testing.T) {
	j1 := mkJudge("a", true, map[Dimension]int{
		DimCorrectness: 9, DimCompleteness: 8, DimFaithfulness: 9, DimFormat: 9, DimSafety: 10,
	})
	j2 := mkJudge("b", true, map[Dimension]int{
		DimCorrectness: 8, DimCompleteness: 9, DimFaithfulness: 8, DimFormat: 8, DimSafety: 9,
	})
	j3 := mkJudge("c", true, map[Dimension]int{
		DimCorrectness: 9, DimCompleteness: 8, DimFaithfulness: 9, DimFormat: 9, DimSafety: 10,
	})
	r := Aggregate([]Judgement{j1, j2, j3})
	if r.Verdict != VerdictPass {
		t.Errorf("expected pass; got %s (%s)", r.Verdict, r.Reason)
	}
}

func TestAggregate_InconclusiveWhenFewerThan2OK(t *testing.T) {
	r := Aggregate([]Judgement{
		mkJudge("a", true, map[Dimension]int{DimCorrectness: 9, DimCompleteness: 9, DimFaithfulness: 9, DimFormat: 9, DimSafety: 9}),
		mkJudge("b", false, nil),
		mkJudge("c", false, nil),
	})
	if r.Verdict != VerdictInconclusive {
		t.Errorf("expected inconclusive; got %s", r.Verdict)
	}
}

func TestAggregate_FailWhenMeanBelow7(t *testing.T) {
	r := Aggregate([]Judgement{
		mkJudge("a", true, map[Dimension]int{DimCorrectness: 6, DimCompleteness: 9, DimFaithfulness: 9, DimFormat: 9, DimSafety: 9}),
		mkJudge("b", true, map[Dimension]int{DimCorrectness: 6, DimCompleteness: 9, DimFaithfulness: 9, DimFormat: 9, DimSafety: 9}),
	})
	if r.Verdict != VerdictFail {
		t.Errorf("expected fail; got %s (%s)", r.Verdict, r.Reason)
	}
	if !strings.Contains(r.Reason, "correctness") {
		t.Errorf("expected dim called out in reason; got %q", r.Reason)
	}
}

func TestAggregate_FailWhenAnyDimMinBelow5(t *testing.T) {
	r := Aggregate([]Judgement{
		mkJudge("a", true, map[Dimension]int{DimCorrectness: 9, DimCompleteness: 9, DimFaithfulness: 9, DimFormat: 9, DimSafety: 4}),
		mkJudge("b", true, map[Dimension]int{DimCorrectness: 9, DimCompleteness: 9, DimFaithfulness: 9, DimFormat: 9, DimSafety: 10}),
	})
	if r.Verdict != VerdictFail {
		t.Errorf("expected fail (min safety=4); got %s (%s)", r.Verdict, r.Reason)
	}
}

func TestAggregate_DimensionsCarryMinMaxMean(t *testing.T) {
	r := Aggregate([]Judgement{
		mkJudge("a", true, map[Dimension]int{DimCorrectness: 6, DimCompleteness: 8, DimFaithfulness: 9, DimFormat: 9, DimSafety: 9}),
		mkJudge("b", true, map[Dimension]int{DimCorrectness: 10, DimCompleteness: 8, DimFaithfulness: 9, DimFormat: 9, DimSafety: 9}),
	})
	for _, d := range r.Dimensions {
		if d.Dim == DimCorrectness {
			if d.Min != 6 || d.Max != 10 || d.Mean != 8 {
				t.Errorf("expected min/max/mean 6/10/8; got %+v", d)
			}
		}
	}
}

// --- Runner ---

type fakeScorer struct {
	name string
	raw  string
	err  error
}

func (f *fakeScorer) Name() string { return f.name }
func (f *fakeScorer) Score(_ context.Context, _, _ string) (string, error) {
	return f.raw, f.err
}

func TestRun_AggregatesAcrossScorers(t *testing.T) {
	r := Run(context.Background(),
		[]Scorer{
			&fakeScorer{name: "openai", raw: `{"scores":[{"dim":"correctness","value":9},{"dim":"completeness","value":9},{"dim":"faithfulness","value":9},{"dim":"format","value":9},{"dim":"safety","value":9}]}`},
			&fakeScorer{name: "anthropic", raw: `{"scores":[{"dim":"correctness","value":9},{"dim":"completeness","value":9},{"dim":"faithfulness","value":9},{"dim":"format","value":9},{"dim":"safety","value":9}]}`},
			&fakeScorer{name: "google", raw: `{"scores":[{"dim":"correctness","value":9},{"dim":"completeness","value":9},{"dim":"faithfulness","value":9},{"dim":"format","value":9},{"dim":"safety","value":9}]}`},
		}, "task", "output", time.Second)
	if r.Verdict != VerdictPass {
		t.Errorf("expected pass; got %s (%s)", r.Verdict, r.Reason)
	}
	if r.OKCount != 3 {
		t.Errorf("expected 3 ok; got %d", r.OKCount)
	}
}

func TestRun_ScorerErrorMarksFailButLetsOthersVote(t *testing.T) {
	r := Run(context.Background(),
		[]Scorer{
			&fakeScorer{name: "anthropic", raw: `{"scores":[{"dim":"correctness","value":9},{"dim":"completeness","value":9},{"dim":"faithfulness","value":9},{"dim":"format","value":9},{"dim":"safety","value":9}]}`},
			&fakeScorer{name: "openai", raw: `{"scores":[{"dim":"correctness","value":9},{"dim":"completeness","value":9},{"dim":"faithfulness","value":9},{"dim":"format","value":9},{"dim":"safety","value":9}]}`},
			&fakeScorer{name: "google", err: errors.New("rate limit")},
		}, "task", "output", time.Second)
	if r.Verdict != VerdictPass {
		t.Errorf("expected pass (2 ok / 3 total still beats threshold); got %s (%s)", r.Verdict, r.Reason)
	}
	if r.OKCount != 2 {
		t.Errorf("expected 2 ok; got %d", r.OKCount)
	}
}

func TestRun_AllErrorIsInconclusive(t *testing.T) {
	r := Run(context.Background(),
		[]Scorer{
			&fakeScorer{name: "a", err: errors.New("down")},
			&fakeScorer{name: "b", err: errors.New("down")},
		}, "t", "o", time.Second)
	if r.Verdict != VerdictInconclusive {
		t.Errorf("expected inconclusive; got %s", r.Verdict)
	}
}

// --- Receipt round-trip ---

func TestSaveAndLoadReceipt(t *testing.T) {
	dir := t.TempDir()
	res := Result{Verdict: VerdictPass, Reason: "ok"}
	path, err := SaveReceipt(dir, "my-task", "task text", "output text", res)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if filepath.Dir(path) != dir {
		t.Errorf("expected receipt under tempdir; got %s", path)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	rcpt, err := LoadReceipt(f)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if rcpt.Slug != "my-task" || rcpt.Result.Verdict != VerdictPass {
		t.Errorf("round-trip lost data: %+v", rcpt)
	}
	if len(rcpt.Sha256) != 64 {
		t.Errorf("expected 64-char sha256; got %d", len(rcpt.Sha256))
	}
}

func TestSaveReceipt_IdempotentForSameInput(t *testing.T) {
	dir := t.TempDir()
	p1, _ := SaveReceipt(dir, "x", "task", "out", Result{Verdict: VerdictPass})
	p2, _ := SaveReceipt(dir, "x", "task", "out", Result{Verdict: VerdictPass})
	if p1 != p2 {
		t.Errorf("expected same path for same (task,output); got %s vs %s", p1, p2)
	}
}

func TestSaveReceipt_DifferentInputDifferentName(t *testing.T) {
	dir := t.TempDir()
	p1, _ := SaveReceipt(dir, "x", "task A", "out", Result{})
	p2, _ := SaveReceipt(dir, "x", "task B", "out", Result{})
	if p1 == p2 {
		t.Errorf("expected different filenames for different tasks")
	}
}

func TestReceipt_JSONIsHumanReadable(t *testing.T) {
	dir := t.TempDir()
	path, _ := SaveReceipt(dir, "slug", "t", "o", Result{Verdict: VerdictPass, Reason: "all good"})
	b, _ := os.ReadFile(path)
	var probe map[string]any
	if err := json.Unmarshal(b, &probe); err != nil {
		t.Fatalf("receipt not valid JSON: %v", err)
	}
	if !strings.Contains(string(b), "\n  ") {
		t.Errorf("expected indented JSON for readability")
	}
}
