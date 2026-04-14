package runtime

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouteTablePrefersStaticRoute(t *testing.T) {
	router := newRouteTable()

	var got string
	router.Handle([]string{http.MethodGet}, "/tenants/:id", func(w http.ResponseWriter, req *http.Request, params routeParams) {
		got = "param:" + params.ByName("id")
		w.WriteHeader(http.StatusAccepted)
	})
	router.Handle([]string{http.MethodGet}, "/tenants/me", func(w http.ResponseWriter, req *http.Request, params routeParams) {
		got = "static"
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/tenants/me", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got != "static" {
		t.Fatalf("matched route = %q, want %q", got, "static")
	}
}

func TestRouteTableCapturesParamsAndWildcard(t *testing.T) {
	router := newRouteTable()

	var tenantID string
	var filePath string
	router.Handle([]string{http.MethodGet}, "/tenants/:tenantID/config", func(w http.ResponseWriter, req *http.Request, params routeParams) {
		tenantID = params.ByName("tenantID")
		w.WriteHeader(http.StatusNoContent)
	})
	router.Handle([]string{http.MethodGet}, "/files/*path", func(w http.ResponseWriter, req *http.Request, params routeParams) {
		filePath = params.ByName("path")
		w.WriteHeader(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/tenants/acme/config", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("config status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if tenantID != "acme" {
		t.Fatalf("tenantID = %q, want %q", tenantID, "acme")
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/files/assets/logo.svg", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("file status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if filePath != "assets/logo.svg" {
		t.Fatalf("wildcard = %q, want %q", filePath, "assets/logo.svg")
	}
}

func TestRouteTableMethodHandling(t *testing.T) {
	router := newRouteTable()
	router.GlobalOPTIONS = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	var gotMethod string
	router.Handle([]string{http.MethodGet}, "/healthz", func(w http.ResponseWriter, req *http.Request, params routeParams) {
		gotMethod = req.Method
		w.WriteHeader(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodHead, "/healthz", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("HEAD status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if gotMethod != http.MethodHead {
		t.Fatalf("HEAD routed as %q, want %q", gotMethod, http.MethodHead)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodOptions, "/healthz", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("OPTIONS status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if allow := rec.Header().Get("Allow"); allow != "GET, HEAD, OPTIONS" {
		t.Fatalf("Allow = %q, want %q", allow, "GET, HEAD, OPTIONS")
	}
}
