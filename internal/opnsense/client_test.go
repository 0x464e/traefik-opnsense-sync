package opnsense

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGetHostAliasesFiltersByHost guards against a regression where the host
// override UUID was appended as a URL path segment instead of the "host"
// query parameter that OPNsense's searchHostAliasAction actually reads —
// which silently disabled server-side filtering and caused every host alias
// in Unbound (across every host override) to be treated as "current" during
// reconcile, leading to unrelated aliases being deleted.
func TestGetHostAliasesFiltersByHost(t *testing.T) {
	const wantUUID = "11111111-2222-3333-4444-555555555555"

	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query().Get("host")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"rows":[]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, true, "key", "secret")
	if _, err := c.GetHostAliases(context.Background(), wantUUID); err != nil {
		t.Fatalf("GetHostAliases returned error: %v", err)
	}

	if gotPath != searchHostAliasApi {
		t.Errorf("request path = %q, want %q (host UUID must not be a path segment)", gotPath, searchHostAliasApi)
	}
	if gotQuery != wantUUID {
		t.Errorf("host query param = %q, want %q", gotQuery, wantUUID)
	}
}
