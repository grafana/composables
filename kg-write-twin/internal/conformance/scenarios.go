package conformance

// Request is a single HTTP call (path is relative to the base URL, and must
// already include the base path, e.g. "/api-server/...").
type Request struct {
	Method  string
	Path    string
	Headers map[string]string
	Body    string
}

// Scenario is a named sequence: optional setup calls, then the asserted request.
type Scenario struct {
	Name    string
	Setup   []Request
	Request Request
}

var jsonHdr = map[string]string{"Content-Type": "application/json", "X-Scope-OrgID": "13", "Accept": "application/json"}
var tenantHdr = map[string]string{"X-Scope-OrgID": "13"}

const (
	entities = "/api-server/apis/kg.grafana.com/v1alpha1/namespaces/stacks-13/entities"
	rels     = "/api-server/apis/kg.grafana.com/v1alpha1/namespaces/stacks-13/relationships"
)

// Scenarios is the single catalog reused by the twin tests, the conformance
// test, and the record tool. Each captured behavior in the design spec appears
// here. Scenarios use unique names so runs are order-independent.
func Scenarios() []Scenario {
	return []Scenario{
		{Name: "entity_create", Setup: []Request{
			// Delete first so the asserted POST is a real create (201) even on a
			// stateful live server. c1 carries scope env=prod, so the delete must match.
			{Method: "DELETE", Path: entities + "/Team/c1?domain=irm&scope%5Benv%5D=prod", Headers: tenantHdr}},
			Request: Request{Method: "POST", Path: entities, Headers: jsonHdr,
				Body: `{"domain":"irm","type":"Team","name":"c1","scope":{"env":"prod"},"properties":{"k":"v"},"ttlSeconds":10}`}},
		{Name: "entity_update", Setup: []Request{
			{Method: "POST", Path: entities, Headers: jsonHdr, Body: `{"domain":"irm","type":"Team","name":"u1","ttlSeconds":10}`}},
			Request: Request{Method: "POST", Path: entities, Headers: jsonHdr,
				Body: `{"domain":"irm","type":"Team","name":"u1","ttlSeconds":10}`}},
		{Name: "entity_ttl_zero", Setup: []Request{
			// Delete first so the asserted POST is a real create (201). TTL=0 is
			// metadata-only, so a leftover z1 would otherwise make this an update.
			{Method: "DELETE", Path: entities + "/Team/z1?domain=irm", Headers: tenantHdr}},
			Request: Request{Method: "POST", Path: entities, Headers: jsonHdr,
				Body: `{"domain":"irm","type":"Team","name":"z1","ttlSeconds":0}`}},
		{Name: "entity_domain_kg_422",
			Request: Request{Method: "POST", Path: entities, Headers: jsonHdr,
				Body: `{"domain":"kg","type":"Team","name":"x","ttlSeconds":10}`}},
		{Name: "entity_missing_ttl_422",
			Request: Request{Method: "POST", Path: entities, Headers: jsonHdr,
				Body: `{"domain":"irm","type":"Team","name":"x"}`}},
		{Name: "entity_missing_tenant_424",
			Request: Request{Method: "POST", Path: entities, Headers: map[string]string{"Content-Type": "application/json", "Accept": "application/json"},
				Body: `{"domain":"irm","type":"Team","name":"x","ttlSeconds":10}`}},
		{Name: "entity_ns_mismatch_403",
			Request: Request{Method: "POST", Path: "/api-server/apis/kg.grafana.com/v1alpha1/namespaces/stacks-99/entities", Headers: jsonHdr,
				Body: `{"domain":"irm","type":"Team","name":"x","ttlSeconds":10}`}},
		{Name: "entity_bad_namespace_400",
			Request: Request{Method: "POST", Path: "/api-server/apis/kg.grafana.com/v1alpha1/namespaces/foo/entities", Headers: jsonHdr,
				Body: `{"domain":"irm","type":"Team","name":"x","ttlSeconds":10}`}},
		{Name: "entity_bad_content_type_415",
			Request: Request{Method: "POST", Path: entities, Headers: map[string]string{"Content-Type": "text/plain", "X-Scope-OrgID": "13", "Accept": "application/json"},
				Body: `{"domain":"irm","type":"Team","name":"x","ttlSeconds":10}`}},
		{Name: "entity_get_405",
			Request: Request{Method: "GET", Path: entities, Headers: tenantHdr}},
		{Name: "relationship_create_200", Setup: []Request{
			{Method: "POST", Path: entities, Headers: jsonHdr, Body: `{"domain":"irm","type":"Team","name":"r-from","ttlSeconds":3600}`},
			{Method: "POST", Path: entities, Headers: jsonHdr, Body: `{"domain":"irm","type":"Service","name":"r-to","ttlSeconds":3600}`}},
			Request: Request{Method: "POST", Path: rels, Headers: jsonHdr,
				Body: `{"domain":"irm","type":"owns","from":{"domain":"irm","type":"Team","name":"r-from"},"to":{"domain":"irm","type":"Service","name":"r-to"},"ttlSeconds":3600}`}},
		{Name: "relationship_missing_to_404", Setup: []Request{
			{Method: "POST", Path: entities, Headers: jsonHdr, Body: `{"domain":"irm","type":"Team","name":"r2-from","ttlSeconds":3600}`}},
			Request: Request{Method: "POST", Path: rels, Headers: jsonHdr,
				Body: `{"domain":"irm","type":"owns","from":{"domain":"irm","type":"Team","name":"r2-from"},"to":{"domain":"irm","type":"Service","name":"ghost"},"ttlSeconds":3600}`}},
		// The KG Write API validates entity/relationship `type` against the
		// identifier contract ^[A-Za-z][A-Za-z0-9_]*$; values containing ':' or
		// '-' (e.g. raw Gamma relation types) must be rejected with 422.
		{Name: "entity_bad_type_422",
			Request: Request{Method: "POST", Path: entities, Headers: jsonHdr,
				Body: `{"domain":"irm","type":"Foo-Bar","name":"x","ttlSeconds":10}`}},
		{Name: "relationship_bad_type_422", Setup: []Request{
			{Method: "POST", Path: entities, Headers: jsonHdr, Body: `{"domain":"irm","type":"Team","name":"bt-from","ttlSeconds":3600}`},
			{Method: "POST", Path: entities, Headers: jsonHdr, Body: `{"domain":"irm","type":"Service","name":"bt-to","ttlSeconds":3600}`}},
			Request: Request{Method: "POST", Path: rels, Headers: jsonHdr,
				Body: `{"domain":"irm","type":"depends-on","from":{"domain":"irm","type":"Team","name":"bt-from"},"to":{"domain":"irm","type":"Service","name":"bt-to"},"ttlSeconds":3600}`}},
	}
}
