package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/grafana/kg-write-twin/internal/conformance"
)

// Records every scenario against KG_TWIN_REAL_URL and writes normalized goldens.
func main() {
	base := os.Getenv("KG_TWIN_REAL_URL")
	if base == "" {
		log.Fatal("set KG_TWIN_REAL_URL (e.g. http://localhost:8030)")
	}
	base = strings.TrimSuffix(base, "/")
	outDir := "internal/conformance/testdata/golden"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatal(err)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	for _, sc := range conformance.Scenarios() {
		for _, s := range sc.Setup {
			mustCall(client, base, s)
		}
		resp := mustCall(client, base, sc.Request)
		n := conformance.Normalize(resp)
		b, _ := json.MarshalIndent(n, "", "  ")
		path := filepath.Join(outDir, sc.Name+".json")
		if err := os.WriteFile(path, b, 0o644); err != nil {
			log.Fatal(err)
		}
		log.Printf("recorded %s -> %s (status %d)", sc.Name, path, resp.Status)
	}
}

func mustCall(c *http.Client, base string, r conformance.Request) conformance.Response {
	var body io.Reader
	if r.Body != "" {
		body = strings.NewReader(r.Body)
	}
	req, err := http.NewRequest(r.Method, base+r.Path, body)
	if err != nil {
		log.Fatal(err)
	}
	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}
	resp, err := c.Do(req)
	if err != nil {
		log.Fatalf("%s %s: %v", r.Method, r.Path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return conformance.Response{Status: resp.StatusCode, Body: string(b)}
}
