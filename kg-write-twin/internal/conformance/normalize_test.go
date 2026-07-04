package conformance

import (
	"strings"
	"testing"
)

func TestNormalizeStripsVolatile(t *testing.T) {
	a := Normalize(Response{Status: 422, Body: `{"status":"X","requestId":"  abc","timestamp":111,"message":"Invalid request: ServletWebRequest: uri=/a;client=127.0.0.1","subErrors":[{"field":"b"},{"field":"a"}]}`})
	b := Normalize(Response{Status: 422, Body: `{"status":"X","requestId":"def","timestamp":999,"message":"Invalid request: ServletWebRequest: uri=/other;client=10.0.0.1","subErrors":[{"field":"a"},{"field":"b"}]}`})
	if string(a.Body) != string(b.Body) {
		t.Errorf("normalized bodies differ:\n a=%s\n b=%s", a.Body, b.Body)
	}
}

func TestNormalizeExpiryScrubbed(t *testing.T) {
	a := Normalize(Response{Status: 201, Body: `{"properties":{"_expires_at":111}}`})
	if !strings.Contains(string(a.Body), `"_expires_at":0`) {
		t.Errorf("expiry not scrubbed: %s", a.Body)
	}
}

func TestNormalizeEmptyBody(t *testing.T) {
	if got := string(Normalize(Response{Status: 204, Body: ""}).Body); got != "null" {
		t.Errorf("empty body = %s, want null", got)
	}
}
