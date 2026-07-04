package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"gopkg.in/yaml.v3"
)

type format int

const (
	formatJSON format = iota
	formatYAML
)

func contentTypeBase(ct string) string {
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return strings.TrimSpace(strings.ToLower(ct))
}

func isYAML(ct string) bool { return ct == "application/x-yaml" || ct == "application/x-yml" }

// decodeBody reads the request body per Content-Type (json default; x-yaml/x-yml
// converted to json) and unmarshals into dst. Returns an apiError on 415/400.
func decodeBody(r *http.Request, dst any) *apiError {
	ct := contentTypeBase(r.Header.Get("Content-Type"))
	data, _ := io.ReadAll(r.Body)
	switch {
	case ct == "" || ct == "application/json":
		if err := json.Unmarshal(data, dst); err != nil {
			e := errParse(err.Error())
			return &e
		}
	case isYAML(ct):
		var generic any
		if err := yaml.Unmarshal(data, &generic); err != nil {
			e := errParse(err.Error())
			return &e
		}
		jb, err := json.Marshal(generic)
		if err != nil {
			e := errParse(err.Error())
			return &e
		}
		if err := json.Unmarshal(jb, dst); err != nil {
			e := errParse(err.Error())
			return &e
		}
	default:
		e := errUnsupportedMedia(contentTypeBase(r.Header.Get("Content-Type")))
		return &e
	}
	return nil
}

func responseFormat(r *http.Request) format {
	accept := strings.ToLower(r.Header.Get("Accept"))
	if strings.Contains(accept, "application/x-yaml") || strings.Contains(accept, "application/x-yml") {
		return formatYAML
	}
	return formatJSON
}

// encode marshals payload as JSON, or converts JSON->YAML for the yaml format so
// keys come from json struct tags.
func encode(f format, payload any) []byte {
	jb, _ := json.Marshal(payload)
	if f == formatJSON {
		return jb
	}
	var generic any
	_ = json.Unmarshal(jb, &generic)
	yb, _ := yaml.Marshal(generic)
	return yb
}

func contentType(f format) string {
	if f == formatYAML {
		return "application/x-yaml"
	}
	return "application/json"
}

// rawJSON re-parses JSON bytes into a generic value so encode(formatYAML,...)
// can render them as YAML.
func rawJSON(b []byte) any {
	var v any
	_ = json.Unmarshal(b, &v)
	return v
}

// parseScopeParams extracts deepObject query params like scope[env]=dev into a map.
// prefix is e.g. "scope" or "from.scope".
func parseScopeParams(r *http.Request, prefix string) map[string]string {
	out := map[string]string{}
	for k, vs := range r.URL.Query() {
		if strings.HasPrefix(k, prefix+"[") && strings.HasSuffix(k, "]") && len(vs) > 0 {
			key := k[len(prefix)+1 : len(k)-1]
			out[key] = vs[0]
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
