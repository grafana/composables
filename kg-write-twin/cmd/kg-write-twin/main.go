package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/grafana/kg-write-twin/internal/api"
	"github.com/grafana/kg-write-twin/internal/store"
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func normalizeAddr(a string) string {
	if a == "" {
		return ":8030"
	}
	if !strings.Contains(a, ":") {
		return ":" + a // bare port
	}
	return a
}

func main() {
	addr := flag.String("addr", normalizeAddr(envOr("PORT", ":8030")), "listen address")
	basePath := flag.String("base-path", envOr("BASE_PATH", "/api-server"), "URL base path")
	seedFile := flag.String("seed-file", os.Getenv("SEED_FILE"), "optional JSON seed file")
	flag.Parse()

	now := func() int64 { return time.Now().UnixMilli() }
	st := store.New(now)

	if *seedFile != "" {
		if err := loadSeed(st, *seedFile); err != nil {
			log.Fatalf("seed: %v", err)
		}
		log.Printf("seeded from %s", *seedFile)
	}

	srv := api.NewServer(st, *basePath, now)
	log.Printf("kg-write-twin listening on %s (base path %s)", *addr, *basePath)
	if err := http.ListenAndServe(*addr, srv); err != nil {
		log.Fatal(err)
	}
}

// loadSeed reads a JSON file matching the /_control/seed body, keyed by tenant.
func loadSeed(st *store.Store, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var byTenant map[string]struct {
		Entities      []store.EntityInput       `json:"entities"`
		Relationships []store.RelationshipInput `json:"relationships"`
	}
	if err := json.Unmarshal(data, &byTenant); err != nil {
		return err
	}
	for tenant, s := range byTenant {
		for i := range s.Entities {
			if s.Entities[i].Origin == "" {
				s.Entities[i].Origin = store.OriginAPI
			}
		}
		for i := range s.Relationships {
			if s.Relationships[i].Origin == "" {
				s.Relationships[i].Origin = store.OriginAPI
			}
		}
		st.Seed(tenant, s.Entities, s.Relationships)
	}
	return nil
}
