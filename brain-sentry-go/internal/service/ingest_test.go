package service

import (
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestExtractText_PlainAndMarkdown(t *testing.T) {
	for _, ext := range []string{"txt", "md", "markdown", ".txt"} {
		got, err := ExtractText(ext, []byte("hello world"))
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", ext, err)
		}
		if got != "hello world" {
			t.Errorf("%s: got %q", ext, got)
		}
	}
}

func TestExtractText_CSV(t *testing.T) {
	csv := "name,role\nAna,AE\nBruno,SDR\n"
	got, err := ExtractText("csv", []byte(csv))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "name | role") || !strings.Contains(got, "Ana | AE") {
		t.Errorf("csv extraction lost structure: %q", got)
	}
}

func TestExtractText_JSON(t *testing.T) {
	got, err := ExtractText("json", []byte(`{"customer":"Acme","stage":"demo"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "Acme") || !strings.Contains(got, "stage") {
		t.Errorf("json extraction lost content: %q", got)
	}
}

func TestExtractText_JSONInvalid(t *testing.T) {
	if _, err := ExtractText("json", []byte("{not json")); err == nil {
		t.Error("expected error for invalid json")
	}
}

func TestExtractText_Unsupported(t *testing.T) {
	_, err := ExtractText("pdf", []byte("%PDF-1.7"))
	if !errors.Is(err, ErrUnsupportedDoc) {
		t.Errorf("expected ErrUnsupportedDoc for pdf; got %v", err)
	}
}

// buildDOCX creates a minimal valid .docx (zip with word/document.xml).
func buildDOCX(t *testing.T, bodyXML string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	doc := `<?xml version="1.0"?><w:document xmlns:w="x"><w:body>` + bodyXML + `</w:body></w:document>`
	if _, err := w.Write([]byte(doc)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestExtractText_DOCX(t *testing.T) {
	docx := buildDOCX(t,
		`<w:p><w:r><w:t>First para.</w:t></w:r></w:p>`+
			`<w:p><w:r><w:t>Second </w:t><w:t>para.</w:t></w:r></w:p>`)
	got, err := ExtractText("docx", docx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "First para.") || !strings.Contains(got, "Second") || !strings.Contains(got, "para.") {
		t.Errorf("docx extraction lost text: %q", got)
	}
}

func TestExtractText_DOCXEntities(t *testing.T) {
	docx := buildDOCX(t, `<w:p><w:r><w:t>A &amp; B &lt; C</w:t></w:r></w:p>`)
	got, err := ExtractText("docx", docx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "A & B < C") {
		t.Errorf("docx entities not unescaped: %q", got)
	}
}

func TestExtractText_DOCXNotAZip(t *testing.T) {
	if _, err := ExtractText("docx", []byte("not a zip")); err == nil {
		t.Error("expected error for non-zip docx bytes")
	}
}

func TestChunkText_Empty(t *testing.T) {
	if c := ChunkText("   \n\n  ", 100); c != nil {
		t.Errorf("blank input should yield no chunks; got %v", c)
	}
}

func TestChunkText_SmallStaysOneChunk(t *testing.T) {
	c := ChunkText("short paragraph", 100)
	if len(c) != 1 || c[0] != "short paragraph" {
		t.Errorf("got %v", c)
	}
}

func TestChunkText_SplitsOnParagraphs(t *testing.T) {
	// Two paragraphs that together exceed the limit should split into two.
	p1 := strings.Repeat("a", 60)
	p2 := strings.Repeat("b", 60)
	c := ChunkText(p1+"\n\n"+p2, 100)
	if len(c) != 2 {
		t.Fatalf("expected 2 chunks; got %d (%v)", len(c), c)
	}
	if c[0] != p1 || c[1] != p2 {
		t.Errorf("chunks not split on paragraph boundary: %v", c)
	}
}

func TestChunkText_PacksParagraphsUnderLimit(t *testing.T) {
	// Three tiny paragraphs under the limit should pack into one chunk.
	c := ChunkText("one\n\ntwo\n\nthree", 100)
	if len(c) != 1 {
		t.Errorf("small paragraphs should pack into one chunk; got %d (%v)", len(c), c)
	}
}

func TestChunkText_HardSplitsOversizedParagraph(t *testing.T) {
	big := strings.Repeat("x", 250)
	c := ChunkText(big, 100)
	if len(c) < 3 {
		t.Fatalf("expected oversized paragraph hard-split into >=3; got %d", len(c))
	}
	// Every chunk must respect the limit.
	for i, ch := range c {
		if len(ch) > 100 {
			t.Errorf("chunk %d exceeds maxChars: %d", i, len(ch))
		}
	}
}

func TestChunkText_DefaultsOnNonPositive(t *testing.T) {
	// maxChars <= 0 should not panic and should fall back to the default.
	c := ChunkText("hello", 0)
	if len(c) != 1 {
		t.Errorf("got %v", c)
	}
}
