package postgres

import (
	"strings"
	"testing"
)

// The bug this guards against: pgx renders []float32 as the Postgres array
// literal {1,2,3}, which vector_in rejects. Brackets are the whole point.
func TestVectorParam_UsesBracketsNotBraces(t *testing.T) {
	got, ok := vectorParam([]float32{1, -2.5, 0.125}).(string)
	if !ok {
		t.Fatalf("expected a string literal, got %T", vectorParam([]float32{1}))
	}
	if want := "[1,-2.5,0.125]"; got != want {
		t.Errorf("vectorParam() = %q, want %q", got, want)
	}
	if strings.ContainsAny(got, "{}") {
		t.Errorf("literal must not use array braces: %q", got)
	}
}

// An empty embedding must become a real SQL NULL. Returning "[]" would make
// Postgres reject the row (a vector needs its declared dimension), and a typed
// nil pointer would be encoded as the text "null".
func TestVectorParam_EmptyIsUntypedNil(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []float32
	}{
		{"nil", nil},
		{"empty", []float32{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := vectorParam(tc.in); got != nil {
				t.Errorf("vectorParam(%v) = %#v, want untyped nil", tc.in, got)
			}
		})
	}
}

// float32 must round-trip in the fewest digits that preserve the value —
// formatting through float64 defaults would write 0.006034851074218750.
func TestVectorParam_RoundTripsFloat32(t *testing.T) {
	got := vectorParam([]float32{0.006034851, -0.00466156}).(string)
	if want := "[0.006034851,-0.00466156]"; got != want {
		t.Errorf("vectorParam() = %q, want %q", got, want)
	}
}

func TestVectorParam_SingleElement(t *testing.T) {
	if got := vectorParam([]float32{0.5}).(string); got != "[0.5]" {
		t.Errorf("vectorParam() = %q, want %q", got, "[0.5]")
	}
}

// A 1536-dim vector (text-embedding-3-small, the production size) must produce
// exactly 1535 separators — an off-by-one in the loop would corrupt every row.
func TestVectorParam_ProductionDimensions(t *testing.T) {
	embedding := make([]float32, 1536)
	for i := range embedding {
		embedding[i] = float32(i) / 1536
	}
	got := vectorParam(embedding).(string)

	if !strings.HasPrefix(got, "[") || !strings.HasSuffix(got, "]") {
		t.Fatal("literal is not bracketed")
	}
	if n := strings.Count(got, ","); n != 1535 {
		t.Errorf("got %d separators, want 1535", n)
	}
}

func TestVectorSelect_CastsToScannableArray(t *testing.T) {
	if got, want := vectorSelect("embedding"), "embedding::float4[] AS embedding"; got != want {
		t.Errorf("vectorSelect() = %q, want %q", got, want)
	}
}

// The SELECT projection must stay column-compatible with the INSERT list:
// scanDecision/scanEvent read positionally, so a divergence silently maps
// values onto the wrong fields.
func TestSelectColumns_MatchInsertColumnOrder(t *testing.T) {
	for _, tc := range []struct{ name, insert, sel string }{
		{"decisions", decisionColumns, decisionSelectColumns},
		{"events", eventColumns, eventSelectColumns},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ins := splitColumns(tc.insert)
			out := splitColumns(tc.sel)
			if len(ins) != len(out) {
				t.Fatalf("column count differs: insert=%d select=%d", len(ins), len(out))
			}
			for i := range ins {
				// The embedding column is the one that legitimately differs;
				// its alias must still restore the original name.
				if ins[i] == "embedding" {
					if !strings.HasSuffix(out[i], "AS embedding") {
						t.Errorf("position %d: %q must alias back to embedding", i, out[i])
					}
					continue
				}
				if ins[i] != out[i] {
					t.Errorf("position %d: insert %q vs select %q", i, ins[i], out[i])
				}
			}
		})
	}
}

func splitColumns(list string) []string {
	parts := strings.Split(list, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.Join(strings.Fields(p), " "))
	}
	return out
}
