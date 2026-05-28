package graph

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/redis/go-redis/v9"
)

// Client wraps a Redis client for FalkorDB graph operations.
type Client struct {
	rdb       *redis.Client
	graphName string
}

// NewClient creates a new FalkorDB graph client.
func NewClient(addr, password, graphName string) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
	})

	// Test connection
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("connecting to FalkorDB: %w", err)
	}

	slog.Info("connected to FalkorDB", "addr", addr, "graph", graphName)

	return &Client{
		rdb:       rdb,
		graphName: graphName,
	}, nil
}

// Close closes the FalkorDB connection.
func (c *Client) Close() error {
	return c.rdb.Close()
}

// DropGraph wipes the entire FalkorDB graph this client points at. Used by
// the rebuild executor; do not call from request-path code. Idempotent:
// dropping an already-missing graph is treated as success.
func (c *Client) DropGraph(ctx context.Context) error {
	cmd := c.rdb.Do(ctx, "GRAPH.DELETE", c.graphName)
	if err := cmd.Err(); err != nil {
		// FalkorDB returns an error when the graph doesn't exist; treat as
		// no-op since the desired end state is "graph absent".
		if strings.Contains(err.Error(), "Graph was not found") || strings.Contains(err.Error(), "Invalid graph") {
			return nil
		}
		return fmt.Errorf("graph delete: %w", err)
	}
	return nil
}

// QueryResult represents the result of a Cypher query.
type QueryResult struct {
	Records []Record
}

// Record represents a single row in a query result.
type Record struct {
	Values map[string]any
}

// Query executes a Cypher query against the FalkorDB graph.
func (c *Client) Query(ctx context.Context, cypher string) (*QueryResult, error) {
	cmd := c.rdb.Do(ctx, "GRAPH.QUERY", c.graphName, cypher)
	if cmd.Err() != nil {
		return nil, fmt.Errorf("graph query: %w", cmd.Err())
	}

	result, err := parseGraphResult(cmd)
	if err != nil {
		return nil, fmt.Errorf("parsing graph result: %w", err)
	}

	return result, nil
}

// parseGraphResult parses the raw FalkorDB response into a QueryResult.
func parseGraphResult(cmd *redis.Cmd) (*QueryResult, error) {
	raw, err := cmd.Result()
	if err != nil {
		return &QueryResult{}, nil
	}

	result := &QueryResult{}

	// FalkorDB returns: [headers, data_rows, stats]
	topLevel, ok := raw.([]any)
	if !ok || len(topLevel) < 2 {
		return result, nil
	}

	// Parse headers
	headers, ok := topLevel[0].([]any)
	if !ok {
		return result, nil
	}

	headerNames := make([]string, 0, len(headers))
	for _, h := range headers {
		if hSlice, ok := h.([]any); ok && len(hSlice) >= 2 {
			if name, ok := hSlice[1].(string); ok {
				headerNames = append(headerNames, name)
			}
		}
	}

	// Parse data rows
	dataRows, ok := topLevel[1].([]any)
	if !ok {
		return result, nil
	}

	for _, row := range dataRows {
		rowSlice, ok := row.([]any)
		if !ok {
			continue
		}

		record := Record{Values: make(map[string]any)}
		for i, val := range rowSlice {
			if i < len(headerNames) {
				record.Values[headerNames[i]] = extractValue(val)
			}
		}
		result.Records = append(result.Records, record)
	}

	return result, nil
}

// extractValue extracts a typed value from a FalkorDB result cell.
func extractValue(val any) any {
	switch v := val.(type) {
	case []any:
		if len(v) == 0 {
			return nil
		}
		// FalkorDB returns [type, value] pairs
		if len(v) == 2 {
			return extractTypedValue(v)
		}
		// Could be a list of values
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = extractValue(item)
		}
		return result
	default:
		return v
	}
}

// extractTypedValue handles FalkorDB typed values [type_id, value].
func extractTypedValue(pair []any) any {
	if len(pair) != 2 {
		return pair
	}

	typeID, ok := pair[0].(int64)
	if !ok {
		return pair[1]
	}

	switch typeID {
	case 1: // NULL
		return nil
	case 2: // STRING
		return pair[1]
	case 3: // INTEGER
		return pair[1]
	case 4: // BOOLEAN
		if s, ok := pair[1].(string); ok {
			return s == "true"
		}
		return pair[1]
	case 5: // DOUBLE
		if s, ok := pair[1].(string); ok {
			var f float64
			fmt.Sscanf(s, "%f", &f)
			return f
		}
		return pair[1]
	case 6: // ARRAY
		if arr, ok := pair[1].([]any); ok {
			result := make([]any, len(arr))
			for i, item := range arr {
				result[i] = extractValue(item)
			}
			return result
		}
		return pair[1]
	case 8: // NODE
		return parseNode(pair[1])
	case 9: // EDGE
		return pair[1]
	default:
		return pair[1]
	}
}

// parseNode extracts properties from a FalkorDB node.
func parseNode(val any) map[string]any {
	node, ok := val.([]any)
	if !ok {
		return nil
	}

	props := make(map[string]any)
	// Node format: [node_id, [label_ids], [[prop_id, type, value], ...]]
	if len(node) >= 3 {
		if propList, ok := node[2].([]any); ok {
			for _, p := range propList {
				if prop, ok := p.([]any); ok && len(prop) >= 3 {
					// prop[0] = property_id, prop[1] = type, prop[2] = value
					if key, ok := prop[0].(string); ok {
						props[key] = extractTypedValue([]any{prop[1], prop[2]})
					}
				}
			}
		}
	}

	return props
}

// EscapeCypher escapes a string for use in Cypher queries.
func EscapeCypher(input string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`'`, `\'`,
		"\n", `\n`,
		"\r", `\r`,
		"\t", `\t`,
		"\x00", ``,
	)
	return r.Replace(input)
}

// EscapeCypherIdentifier escapes an identifier for use in Cypher labels/types.
func EscapeCypherIdentifier(input string) string {
	// Remove null bytes and control characters
	cleaned := strings.Map(func(r rune) rune {
		if r == 0 || r == '\n' || r == '\r' || r == '\t' {
			return -1
		}
		return r
	}, input)

	cleaned = strings.TrimSpace(cleaned)

	// Replace backticks
	cleaned = strings.ReplaceAll(cleaned, "`", "``")

	return "`" + cleaned + "`"
}

// GetString safely gets a string from a record value.
func GetString(values map[string]any, key string) string {
	if v, ok := values[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetInt64 safely gets an int64 from a record value.
func GetInt64(values map[string]any, key string) int64 {
	if v, ok := values[key]; ok {
		switch n := v.(type) {
		case int64:
			return n
		case float64:
			return int64(n)
		}
	}
	return 0
}

// GetFloat64 safely gets a float64 from a record value.
func GetFloat64(values map[string]any, key string) float64 {
	if v, ok := values[key]; ok {
		switch n := v.(type) {
		case float64:
			return n
		case int64:
			return float64(n)
		}
	}
	return 0
}
