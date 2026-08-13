package model

import (
	"encoding/json"
	"testing"
)

func ptr(i int64) *int64 { return &i }

func TestValidateEntity(t *testing.T) {
	cases := []struct {
		name string
		req  EntityWriteRequest
		want []FieldError // field+message only checked
	}{
		{"ok", EntityWriteRequest{Domain: "irm", Type: "Team", Name: "a", TTLSeconds: ptr(10)}, nil},
		{"domain kg", EntityWriteRequest{Domain: "kg", Type: "Team", Name: "a", TTLSeconds: ptr(10)},
			[]FieldError{{Field: "domain", Message: domainMsg}}},
		{"domain upper", EntityWriteRequest{Domain: "Bad", Type: "Team", Name: "a", TTLSeconds: ptr(10)},
			[]FieldError{{Field: "domain", Message: domainMsg}}},
		{"type bad", EntityWriteRequest{Domain: "irm", Type: "1bad", Name: "a", TTLSeconds: ptr(10)},
			[]FieldError{{Field: "type", Message: typeMsg}}},
		{"name blank", EntityWriteRequest{Domain: "irm", Type: "Team", Name: "", TTLSeconds: ptr(10)},
			[]FieldError{{Field: "name", Message: blankMsg}}},
		{"ttl nil", EntityWriteRequest{Domain: "irm", Type: "Team", Name: "a"},
			[]FieldError{{Field: "ttlSeconds", Message: nullMsg}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ValidateEntity(c.req)
			if len(got) != len(c.want) {
				t.Fatalf("got %d errors %v, want %d", len(got), got, len(c.want))
			}
			for i := range c.want {
				if got[i].Field != c.want[i].Field || got[i].Message != c.want[i].Message {
					t.Errorf("err[%d]=%+v want field=%s msg=%s", i, got[i], c.want[i].Field, c.want[i].Message)
				}
			}
		})
	}
}

func TestFieldErrorMarshal(t *testing.T) {
	withVal, _ := json.Marshal(FieldError{Field: "domain", RejectedValue: "kg", HasRejected: true, Message: domainMsg})
	if got := string(withVal); got != `{"@type":"ApiValidationError","field":"domain","rejectedValue":"kg","message":"`+domainMsg+`"}` {
		t.Errorf("with value: %s", got)
	}
	noVal, _ := json.Marshal(FieldError{Field: "name", Message: blankMsg})
	if got := string(noVal); got != `{"@type":"ApiValidationError","field":"name","message":"`+blankMsg+`"}` {
		t.Errorf("no value: %s", got)
	}
}
