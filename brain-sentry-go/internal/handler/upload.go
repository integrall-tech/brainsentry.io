package handler

import (
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/internal/dto"
	"github.com/integraltech/brainsentry/internal/service"
)

// maxUploadBytes caps an ingested document at 10 MiB — generous for the
// text formats we support, small enough to bound memory + chunk count.
const maxUploadBytes = 10 << 20

// maxUploadChunks caps how many memories one upload can create, so a
// pathological file can't flood the tenant.
const maxUploadChunks = 500

// Upload handles POST /v1/memories/upload (multipart/form-data).
//
// Form fields:
//   file        (required) the document — .txt/.md/.csv/.json/.docx
//   category    (optional) MemoryCategory applied to every chunk
//   importance  (optional) ImportanceLevel applied to every chunk
//   chunkChars  (optional) target chunk size; defaults to service default
//
// Each chunk becomes a memory with provenance IMPORTED and
// sourceReference set to the original filename, so ingested knowledge is
// trust-scored accordingly and traceable back to its document.
func (h *MemoryHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file field is required")
		return
	}
	defer file.Close()

	// Read with a hard cap so an over-large or lying Content-Length can't
	// exhaust memory.
	data, err := io.ReadAll(io.LimitReader(file, maxUploadBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read uploaded file")
		return
	}
	if len(data) > maxUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "file exceeds 10 MiB limit")
		return
	}

	filename := header.Filename
	ext := strings.TrimPrefix(filepath.Ext(filename), ".")

	text, err := service.ExtractText(ext, data)
	if err != nil {
		// Unsupported type is a 415; anything else is a 400 (bad content).
		if strings.Contains(err.Error(), service.ErrUnsupportedDoc.Error()) {
			writeError(w, http.StatusUnsupportedMediaType,
				"unsupported document type: ."+ext+" (supported: txt, md, csv, json, docx)")
			return
		}
		writeError(w, http.StatusBadRequest, "could not extract text: "+err.Error())
		return
	}

	chunkChars := 0
	if v := r.FormValue("chunkChars"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			chunkChars = n
		}
	}
	chunks := service.ChunkText(text, chunkChars)
	if len(chunks) == 0 {
		writeError(w, http.StatusBadRequest, "document produced no text to ingest")
		return
	}
	if len(chunks) > maxUploadChunks {
		writeError(w, http.StatusRequestEntityTooLarge,
			"document produced "+strconv.Itoa(len(chunks))+" chunks; max is "+strconv.Itoa(maxUploadChunks))
		return
	}

	category := domain.MemoryCategory(r.FormValue("category"))
	importance := domain.ImportanceLevel(r.FormValue("importance"))

	created := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		m, err := h.memoryService.CreateMemory(r.Context(), dto.CreateMemoryRequest{
			Content:         chunk,
			Category:        category,
			Importance:      importance,
			SourceType:      "upload",
			SourceReference: filename,
			Provenance:      domain.ProvenanceImported,
		})
		if err != nil {
			// Surface a partial result rather than silently dropping: the
			// caller learns how many landed before the failure.
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error":      "failed creating memory for a chunk: " + err.Error(),
				"filename":   filename,
				"chunks":     len(chunks),
				"created":    created,
				"createdLen": len(created),
			})
			return
		}
		created = append(created, m.ID)
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"filename":   filename,
		"chunks":     len(chunks),
		"created":    created,
		"createdLen": len(created),
		"provenance": domain.ProvenanceImported,
	})
}
