package service

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// ingest.go turns an uploaded document into chunked plain text ready to
// become memories. Everything here is PURE (bytes in, strings out) so the
// extraction + chunking rules are fully unit-testable without a DB, an
// HTTP server, or external document-parsing services.
//
// Supported formats use only the Go stdlib — no heavyweight deps:
//   .txt / .md   plain text
//   .csv         one logical line per row (header + rows)
//   .json        pretty-flattened text
//   .docx        word/document.xml extracted via archive/zip + regex
// PDF is intentionally out of scope here (needs a third-party parser);
// callers get a clear ErrUnsupportedDoc they can surface as 415.

// ErrUnsupportedDoc is returned for a file extension we can't extract.
var ErrUnsupportedDoc = fmt.Errorf("unsupported document type")

// DefaultChunkChars is the target size for a single memory chunk. Tuned
// so a chunk is a meaningful paragraph-ish unit, not a sentence and not a
// whole document.
const DefaultChunkChars = 1500

// ExtractText pulls plain text out of a document's raw bytes based on its
// lowercase extension (without the dot), e.g. "txt", "md", "csv", "json",
// "docx".
func ExtractText(ext string, data []byte) (string, error) {
	switch strings.ToLower(strings.TrimPrefix(ext, ".")) {
	case "txt", "md", "markdown", "text":
		return string(data), nil
	case "csv":
		return extractCSV(data)
	case "json":
		return extractJSON(data)
	case "docx":
		return extractDOCX(data)
	default:
		return "", fmt.Errorf("%w: %q", ErrUnsupportedDoc, ext)
	}
}

func extractCSV(data []byte) (string, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1 // tolerate ragged rows
	var b strings.Builder
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("parsing csv: %w", err)
		}
		b.WriteString(strings.Join(rec, " | "))
		b.WriteByte('\n')
	}
	return b.String(), nil
}

func extractJSON(data []byte) (string, error) {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return "", fmt.Errorf("parsing json: %w", err)
	}
	// Re-marshal pretty so the text carries the structure readably.
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", fmt.Errorf("formatting json: %w", err)
	}
	return string(out), nil
}

// docxRunText matches the text inside <w:t>...</w:t> runs in a docx's
// document.xml. We strip tags rather than full XML-decode because we only
// want the prose, and a regex over the runs is robust to the namespace
// noise Word emits.
var docxRunText = regexp.MustCompile(`(?s)<w:t[^>]*>(.*?)</w:t>`)

func extractDOCX(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("opening docx (zip): %w", err)
	}
	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("reading document.xml: %w", err)
		}
		xmlBytes, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return "", fmt.Errorf("reading document.xml: %w", err)
		}
		var b strings.Builder
		for _, m := range docxRunText.FindAllSubmatch(xmlBytes, -1) {
			b.Write(m[1])
			b.WriteByte(' ')
		}
		// Word paragraphs are <w:p>; turn them into newlines for readability.
		text := strings.ReplaceAll(b.String(), "</w:p>", "\n")
		return unescapeXML(strings.TrimSpace(text)), nil
	}
	return "", fmt.Errorf("docx has no word/document.xml")
}

func unescapeXML(s string) string {
	r := strings.NewReplacer(
		"&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&apos;", "'",
	)
	return r.Replace(s)
}

// ChunkText splits text into chunks of at most maxChars, breaking on
// paragraph boundaries (blank lines) where possible so a chunk stays a
// coherent unit. A paragraph longer than maxChars is hard-split. Empty
// input yields no chunks. maxChars <= 0 falls back to DefaultChunkChars.
func ChunkText(text string, maxChars int) []string {
	if maxChars <= 0 {
		maxChars = DefaultChunkChars
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	paras := splitParagraphs(text)
	chunks := make([]string, 0, len(paras))
	var cur strings.Builder

	flush := func() {
		if cur.Len() > 0 {
			chunks = append(chunks, strings.TrimSpace(cur.String()))
			cur.Reset()
		}
	}

	for _, p := range paras {
		// A single oversized paragraph: flush what we have, then hard-split.
		if len(p) > maxChars {
			flush()
			for len(p) > maxChars {
				chunks = append(chunks, strings.TrimSpace(p[:maxChars]))
				p = p[maxChars:]
			}
			if strings.TrimSpace(p) != "" {
				cur.WriteString(p)
			}
			continue
		}
		// Would adding this paragraph overflow the current chunk? flush first.
		if cur.Len() > 0 && cur.Len()+len(p)+2 > maxChars {
			flush()
		}
		if cur.Len() > 0 {
			cur.WriteString("\n\n")
		}
		cur.WriteString(p)
	}
	flush()
	return chunks
}

var blankLine = regexp.MustCompile(`\n[ \t]*\n`)

func splitParagraphs(text string) []string {
	raw := blankLine.Split(text, -1)
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		if tp := strings.TrimSpace(p); tp != "" {
			out = append(out, tp)
		}
	}
	return out
}
