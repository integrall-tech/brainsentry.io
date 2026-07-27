package postgres

import (
	"strconv"
	"strings"
)

// pgvector columns (decisions.embedding, events.embedding — migration 8) need
// a different wire representation than the FLOAT4[] used by memories, and the
// difference is easy to miss because both are "a slice of floats" in Go.
//
// pgx has no encoder for the vector type, so it falls back to text and writes
// a []float32 the way Postgres writes arrays: {1,2,3}. vector_in only accepts
// the bracket form [1,2,3], so every INSERT carrying an embedding failed with
//
//	invalid input syntax for type vector: "{0.006,-0.004,...}" (SQLSTATE 22P02)
//
// leaving POST /v1/decisions/ and the event pipeline returning 400 for any
// memory that had an embedding — i.e. all of them.
//
// Use vectorParam on the way in (with an explicit ::vector cast on the
// placeholder) and vectorSelect on the way out (which casts back to float4[],
// a type pgx *can* scan into []float32).

// vectorParam renders an embedding as a pgvector literal, or nil when empty so
// the column stores NULL instead of an unparseable "[]".
//
// Returns `any` because the nil case must reach pgx as an untyped NULL: a
// typed (*string)(nil) would be encoded as the text "null".
func vectorParam(embedding []float32) any {
	if len(embedding) == 0 {
		return nil
	}

	var b strings.Builder
	// 12 chars/float is a close-enough guess for -0.006034851 and friends;
	// worst case the builder grows once.
	b.Grow(len(embedding)*12 + 2)
	b.WriteByte('[')
	for i, f := range embedding {
		if i > 0 {
			b.WriteByte(',')
		}
		// 'g' with -1 precision round-trips a float32 in the fewest digits,
		// so the stored vector matches what the embedding provider returned.
		b.WriteString(strconv.FormatFloat(float64(f), 'g', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}

// vectorSelect returns the SELECT expression for a vector column: pgx cannot
// scan pgvector's text output into []float32, but float4[] scans natively.
// The alias keeps the column name stable for scanners.
func vectorSelect(column string) string {
	return column + "::float4[] AS " + column
}
