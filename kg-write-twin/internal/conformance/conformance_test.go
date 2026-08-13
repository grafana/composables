package conformance

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/grafana/kg-write-twin/internal/api"
	"github.com/grafana/kg-write-twin/internal/store"
)

// startTwin returns a fresh in-process twin.
func startTwin() *httptest.Server {
	st := store.New(func() int64 { return 1000 })
	return httptest.NewServer(api.NewServer(st, "/api-server", func() int64 { return 1000 }))
}

func call(t *testing.T, base string, r Request) Response {
	t.Helper()
	var body io.Reader
	if r.Body != "" {
		body = strings.NewReader(r.Body)
	}
	req, err := http.NewRequest(r.Method, base+r.Path, body)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return Response{Status: resp.StatusCode, Body: string(b)}
}

func runScenario(t *testing.T, base string, sc Scenario) Normalized {
	for _, s := range sc.Setup {
		call(t, base, s)
	}
	return Normalize(call(t, base, sc.Request))
}

// TestTwinMatchesGoldens is hermetic: it compares the in-process twin against
// committed goldens. Populate goldens with `make record KG_TWIN_REAL_URL=...`.
func TestTwinMatchesGoldens(t *testing.T) {
	twin := startTwin()
	defer twin.Close()
	for _, sc := range Scenarios() {
		t.Run(sc.Name, func(t *testing.T) {
			goldenPath := filepath.Join("testdata", "golden", sc.Name+".json")
			gb, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Skipf("no golden (%s); run `make record`", goldenPath)
			}
			var golden Normalized
			if err := json.Unmarshal(gb, &golden); err != nil {
				t.Fatalf("golden parse: %v", err)
			}
			got := runScenario(t, twin.URL, sc)
			assertEqual(t, sc.Name, golden, got)
		})
	}
}

// TestTwinMatchesLive runs only when KG_TWIN_REAL_URL is set: it diffs the twin
// against the live instance directly.
func TestTwinMatchesLive(t *testing.T) {
	real := os.Getenv("KG_TWIN_REAL_URL")
	if real == "" {
		t.Skip("set KG_TWIN_REAL_URL to run live differential conformance")
	}
	real = strings.TrimSuffix(real, "/")
	twin := startTwin()
	defer twin.Close()
	for _, sc := range Scenarios() {
		t.Run(sc.Name, func(t *testing.T) {
			want := runScenario(t, real, sc)
			got := runScenario(t, twin.URL, sc)
			assertEqual(t, sc.Name, want, got)
		})
	}
}

func assertEqual(t *testing.T, name string, want, got Normalized) {
	t.Helper()
	if want.Status != got.Status {
		t.Errorf("%s: status got %d want %d", name, got.Status, want.Status)
	}
	// Compare canonicalized (re-compacted) JSON so stored golden indentation
	// (MarshalIndent) doesn't cause spurious mismatches.
	wb, gb := canon(want.Body), canon(got.Body)
	if wb != gb {
		t.Errorf("%s: body mismatch\n want: %s\n got:  %s", name, wb, gb)
	}
}

// canon re-marshals JSON to a compact, key-sorted canonical form.
func canon(b []byte) string {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return string(b)
	}
	out, _ := json.Marshal(v)
	return string(out)
}
