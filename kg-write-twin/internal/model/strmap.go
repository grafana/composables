package model

import (
	"bytes"
	"encoding/json"
	"strconv"
)

// StringMap is a map[string]string that, when decoded from JSON, coerces scalar
// values (numbers, booleans) to their string form — matching the real API's
// Jackson Map<String,String> binding.
type StringMap map[string]string

func (m *StringMap) UnmarshalJSON(b []byte) error {
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		*m = nil
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	out := make(StringMap, len(raw))
	for k, v := range raw {
		out[k] = coerceScalar(v)
	}
	*m = out
	return nil
}

func coerceScalar(raw json.RawMessage) string {
	s := string(bytes.TrimSpace(raw))
	if len(s) == 0 {
		return ""
	}
	if s[0] == '"' {
		if unq, err := strconv.Unquote(s); err == nil {
			return unq
		}
	}
	return s // numbers, booleans, null -> literal text
}
