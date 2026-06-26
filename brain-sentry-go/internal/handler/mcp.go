package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/integraltech/brainsentry/internal/mcp"
)

// MCPHandler handles MCP protocol endpoints (SSE transport).
type MCPHandler struct {
	server *mcp.Server
}

// NewMCPHandler creates a new MCPHandler.
func NewMCPHandler(server *mcp.Server) *MCPHandler {
	return &MCPHandler{server: server}
}

// HandleSSE handles the SSE endpoint for MCP.
// POST /v1/mcp/sse - receives JSON-RPC messages and returns responses via SSE.
func (h *MCPHandler) HandleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "SSE not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Read the JSON-RPC request from the body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", "failed to read request")
		flusher.Flush()
		return
	}

	resp := h.server.HandleMessage(r.Context(), body)
	if resp != nil {
		fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(resp))
		flusher.Flush()
	}
}

// HandleMessage handles a single JSON-RPC message via HTTP POST.
// POST /v1/mcp/message - standard request/response transport.
func (h *MCPHandler) HandleMessage(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request")
		return
	}

	resp := h.server.HandleMessage(r.Context(), body)
	if resp == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)
}

// HandleBatch handles batched JSON-RPC messages.
// POST /v1/mcp/batch - process multiple JSON-RPC messages.
func (h *MCPHandler) HandleBatch(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request")
		return
	}

	// Try to parse as array
	var requests []json.RawMessage
	if err := json.Unmarshal(body, &requests); err != nil {
		// Single message
		resp := h.server.HandleMessage(r.Context(), body)
		if resp == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(resp)
		return
	}

	// Process batch
	responses := make([]json.RawMessage, 0, len(requests))
	for _, reqData := range requests {
		resp := h.server.HandleMessage(r.Context(), reqData)
		if resp != nil {
			responses = append(responses, resp)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responses)
}

// HandleStdio processes MCP messages from stdin (for stdio transport mode).
// This is used when running the server as an MCP stdio server.
func HandleStdio(server *mcp.Server, reader io.Reader, writer io.Writer) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		resp := server.HandleMessage(context.Background(), line)
		if resp != nil {
			if _, err := writer.Write(resp); err != nil {
				return err
			}
			if _, err := writer.Write([]byte("\n")); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}
