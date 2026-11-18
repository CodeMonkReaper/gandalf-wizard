package romero

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRomeroPublicClientAdapter_Contract(t *testing.T) {
	t.Run("SearchSendsCorrectPayload", func(t *testing.T) {
		var gotBody map[string]any
		var gotAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/api/v1/memories/search" {
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
			gotAuth = r.Header.Get("Authorization")
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &gotBody)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"candidates":[{"id":"m1-uuid","summary":"test summary","status":"active","epistemic_status":"observed"}],"degraded_mode":false,"total_found":1}`))
		}))
		defer srv.Close()

		adapter := NewRomeroPublicClientAdapter(srv.URL, srv.Client(), func() string { return "secret-token-123" })
		scope := ResolvedScope{ProjectID: "faro-uuid", CallerIdentity: "tester"}
		result, err := adapter.Search(context.Background(), scope, "informes")
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if gotAuth != "Bearer secret-token-123" {
			t.Errorf("auth header: got %q", gotAuth)
		}
		if gotBody["query"] != "informes" {
			t.Errorf("query: got %v", gotBody["query"])
		}
		if gotBody["include_superseded"] != true {
			t.Errorf("include_superseded: got %v", gotBody["include_superseded"])
		}
		scopes, _ := gotBody["scopes"].([]any)
		if len(scopes) != 1 || scopes[0] != "project_shared" {
			t.Errorf("scopes: got %v", gotBody["scopes"])
		}
		pids, _ := gotBody["project_ids"].([]any)
		if len(pids) != 1 || pids[0] != "faro-uuid" {
			t.Errorf("project_ids: got %v", gotBody["project_ids"])
		}
		if len(result.Hits) != 1 || result.Hits[0].MemoryID != "m1-uuid" {
			t.Errorf("hits: %v", result.Hits)
		}
		if result.Hits[0].ProjectID != "faro-uuid" {
			t.Errorf("hit ProjectID should come from scope: got %q", result.Hits[0].ProjectID)
		}
		if result.Hits[0].VigenciaState != VigenciaActive {
			t.Errorf("vigencia: got %q", result.Hits[0].VigenciaState)
		}
	})

	t.Run("SearchMapsSupersededStatus", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"candidates":[{"id":"m2","summary":"old","status":"superseded","epistemic_status":"observed"}],"degraded_mode":false,"total_found":1}`))
		}))
		defer srv.Close()
		adapter := NewRomeroPublicClientAdapter(srv.URL, srv.Client(), func() string { return "t" })
		result, _ := adapter.Search(context.Background(), ResolvedScope{ProjectID: "p"}, "x")
		if result.Hits[0].VigenciaState != VigenciaSuperseded {
			t.Errorf("superseded not mapped: %q", result.Hits[0].VigenciaState)
		}
	})

	t.Run("SearchDegradedMode", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"candidates":[],"degraded_mode":true,"total_found":0}`))
		}))
		defer srv.Close()
		adapter := NewRomeroPublicClientAdapter(srv.URL, srv.Client(), func() string { return "t" })
		result, _ := adapter.Search(context.Background(), ResolvedScope{ProjectID: "p"}, "x")
		if !result.Degraded || result.DegradationNotice == "" {
			t.Errorf("degraded not propagated: %+v", result)
		}
	})

	t.Run("Search401IsAccessDenied", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()
		adapter := NewRomeroPublicClientAdapter(srv.URL, srv.Client(), func() string { return "t" })
		_, err := adapter.Search(context.Background(), ResolvedScope{ProjectID: "p"}, "x")
		if err != ErrAccessDenied {
			t.Errorf("err: got %v, want ErrAccessDenied", err)
		}
	})

	t.Run("Search500IsServiceInterrupted", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()
		adapter := NewRomeroPublicClientAdapter(srv.URL, srv.Client(), func() string { return "t" })
		_, err := adapter.Search(context.Background(), ResolvedScope{ProjectID: "p"}, "x")
		if err == nil || !strings.Contains(err.Error(), "service interrupted") {
			t.Errorf("err: got %v", err)
		}
	})

	t.Run("GetDetailMapsFieldsAndZeroSource", func(t *testing.T) {
		var gotAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, "/api/v1/memories/") {
				t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
			}
			gotAuth = r.Header.Get("Authorization")
			w.Write([]byte(`{"memory":{"ID":"m1","ProjectID":"faro-uuid","Status":"active","EpistemicStatus":"observed","PrimarySourceID":"00000000-0000-0000-0000-000000000000"}}`))
		}))
		defer srv.Close()
		adapter := NewRomeroPublicClientAdapter(srv.URL, srv.Client(), func() string { return "tok" })
		detail, err := adapter.GetDetail(context.Background(), ResolvedScope{ProjectID: "faro-uuid"}, "m1")
		if err != nil {
			t.Fatalf("getDetail: %v", err)
		}
		if gotAuth != "Bearer tok" {
			t.Errorf("auth: %q", gotAuth)
		}
		if detail.MemoryID != "m1" || detail.ProjectID != "faro-uuid" {
			t.Errorf("detail: %+v", detail)
		}
		if detail.SourceRef != "" {
			t.Errorf("zero UUID should map to empty SourceRef, got %q", detail.SourceRef)
		}
	})

	t.Run("GetDetailWithRealSourceRef", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"memory":{"ID":"m1","ProjectID":"faro","Status":"active","EpistemicStatus":"observed","PrimarySourceID":"F1-uuid"}}`))
		}))
		defer srv.Close()
		adapter := NewRomeroPublicClientAdapter(srv.URL, srv.Client(), func() string { return "t" })
		detail, _ := adapter.GetDetail(context.Background(), ResolvedScope{ProjectID: "faro"}, "m1")
		if detail.SourceRef != "F1-uuid" {
			t.Errorf("SourceRef: got %q", detail.SourceRef)
		}
	})

	t.Run("GetDetail404IsAccessDenied", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()
		adapter := NewRomeroPublicClientAdapter(srv.URL, srv.Client(), func() string { return "t" })
		_, err := adapter.GetDetail(context.Background(), ResolvedScope{ProjectID: "p"}, "m1")
		if err != ErrAccessDenied {
			t.Errorf("err: %v, want ErrAccessDenied", err)
		}
	})

	t.Run("ResponseBodyCappedAt4MiB", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"candidates":[{"id":"` + strings.Repeat("x", 5<<20) + `"}],"degraded_mode":false,"total_found":1}`))
		}))
		defer srv.Close()
		adapter := NewRomeroPublicClientAdapter(srv.URL, srv.Client(), func() string { return "t" })
		_, err := adapter.Search(context.Background(), ResolvedScope{ProjectID: "p"}, "x")
		if err == nil {
			t.Errorf("expected error for oversized body")
		}
	})
}
