package model

import "encoding/json"

// EntityRef identifies an entity endpoint of a relationship.
type EntityRef struct {
	Domain string    `json:"domain"`
	Type   string    `json:"type"`
	Name   string    `json:"name"`
	Scope  StringMap `json:"scope"`
}

// EntityWriteRequest is the POST /entities body.
type EntityWriteRequest struct {
	Domain     string    `json:"domain"`
	Type       string    `json:"type"`
	Name       string    `json:"name"`
	Scope      StringMap `json:"scope"`
	Properties StringMap `json:"properties"`
	TTLSeconds *int64    `json:"ttlSeconds"`
}

// RelationshipWriteRequest is the POST /relationships body.
type RelationshipWriteRequest struct {
	Domain     string     `json:"domain"`
	Type       string     `json:"type"`
	From       *EntityRef `json:"from"`
	To         *EntityRef `json:"to"`
	Properties StringMap  `json:"properties"`
	TTLSeconds *int64     `json:"ttlSeconds"`
}

// FieldError is one ApiValidationError sub-error.
type FieldError struct {
	Field         string
	RejectedValue any
	HasRejected   bool
	Message       string
}

func (e FieldError) MarshalJSON() ([]byte, error) {
	// A struct preserves key order (@type, field, rejectedValue, message) to
	// match the real API; encoding/json would sort a map's keys.
	type ordered struct {
		Type          string `json:"@type"`
		Field         string `json:"field"`
		RejectedValue any    `json:"rejectedValue,omitempty"`
		Message       string `json:"message"`
	}
	o := ordered{Type: "ApiValidationError", Field: e.Field, Message: e.Message}
	if e.HasRejected {
		o.RejectedValue = e.RejectedValue
	}
	return json.Marshal(o)
}
