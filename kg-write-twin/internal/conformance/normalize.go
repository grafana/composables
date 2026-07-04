package conformance

import (
	"encoding/json"
	"sort"
	"strings"
)

// Response is a captured HTTP response.
type Response struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}

// Normalized is the comparable form of a response with volatile fields removed.
type Normalized struct {
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body"`
}

// Normalize strips/canonicalizes volatile fields so only meaningful differences
// between the twin and the real API surface.
func Normalize(resp Response) Normalized {
	n := Normalized{Status: resp.Status}
	trimmed := strings.TrimSpace(resp.Body)
	if trimmed == "" {
		n.Body = json.RawMessage(`null`)
		return n
	}
	var v any
	if err := json.Unmarshal([]byte(trimmed), &v); err != nil {
		// Non-JSON body: keep as a JSON string.
		b, _ := json.Marshal(trimmed)
		n.Body = b
		return n
	}
	v = scrub(v)
	b, _ := json.Marshal(v) // encoding/json sorts map keys -> canonical
	n.Body = b
	return n
}

// scrub recursively removes/rewrites volatile fields.
func scrub(v any) any {
	switch t := v.(type) {
	case map[string]any:
		delete(t, "requestId")
		delete(t, "timestamp")
		for k, val := range t {
			switch k {
			case "_expires_at", "_expired", "expiresAt", "expired", "createdAt":
				t[k] = 0 // absolute time -> sentinel
			case "message":
				if s, ok := val.(string); ok {
					t[k] = scrubMessage(s)
				}
			default:
				t[k] = scrub(val)
			}
		}
		if se, ok := t["subErrors"].([]any); ok {
			sort.Slice(se, func(i, j int) bool {
				return fieldOf(se[i]) < fieldOf(se[j])
			})
		}
		return t
	case []any:
		for i := range t {
			t[i] = scrub(t[i])
		}
		return t
	default:
		return v
	}
}

func fieldOf(v any) string {
	if m, ok := v.(map[string]any); ok {
		if f, ok := m["field"].(string); ok {
			return f
		}
	}
	return ""
}

func scrubMessage(s string) string {
	if strings.HasPrefix(s, "Invalid request: ServletWebRequest:") {
		return "Invalid request: ServletWebRequest"
	}
	if strings.HasPrefix(s, "JSON parse error:") {
		return "JSON parse error"
	}
	if strings.HasPrefix(s, "Failed initializing tenantId=") {
		return "Failed initializing tenantId"
	}
	return s
}
