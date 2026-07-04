package api

import (
	"net/http"
	"strconv"
	"strings"
)

// resolveTenant applies the real API's check order:
//  1. missing X-Scope-OrgID           -> 424
//  2. namespace not "stacks-<id>"     -> 400
//  3. namespace stackId != tenant     -> 403
//  4. tenant not an integer           -> 500
//
// On success it returns the tenant id (the org header value).
func resolveTenant(r *http.Request, namespace string) (string, *apiError) {
	org := r.Header.Get("X-Scope-OrgID")
	if org == "" {
		e := errTenantMissing()
		return "", &e
	}
	stackID, ok := strings.CutPrefix(namespace, "stacks-")
	if !ok || stackID == "" {
		e := errBadNamespace()
		return "", &e
	}
	if stackID != org {
		e := errNamespaceMismatch()
		return "", &e
	}
	if _, err := strconv.Atoi(org); err != nil {
		e := errTenantInit(org)
		return "", &e
	}
	return org, nil
}
