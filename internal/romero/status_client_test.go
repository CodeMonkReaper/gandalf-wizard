package romero

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func statusAdapter(t *testing.T, handler http.Handler) (*RomeroPublicClientAdapter, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	adapter := NewRomeroPublicClientAdapter(server.URL, server.Client(), func() string { return "test-token" })
	return adapter, server
}

func TestRomeroStatusClient(t *testing.T) {
	t.Run("200 mapea el payload de estado", func(t *testing.T) {
		adapter, _ := statusAdapter(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/api/v1/system/status" {
				t.Errorf("petición inesperada: %s %s", r.Method, r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer test-token" {
				t.Error("falta el encabezado Authorization")
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok","version":"1.2.3","database":"up","active_profile":"lite"}`))
		}))

		status, err := adapter.CheckStatus(context.Background())
		if err != nil {
			t.Fatalf("CheckStatus: error inesperado: %v", err)
		}
		want := SystemStatus{Status: "ok", Version: "1.2.3", Database: "up", ActiveProfile: "lite"}
		if status != want {
			t.Errorf("status = %+v, want %+v", status, want)
		}
	})

	t.Run("401 y 403 mapean a ErrAccessDenied", func(t *testing.T) {
		for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden} {
			adapter, _ := statusAdapter(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			}))
			if _, err := adapter.CheckStatus(context.Background()); !errors.Is(err, ErrAccessDenied) {
				t.Errorf("CheckStatus(%d): error %v no es ErrAccessDenied", code, err)
			}
		}
	})

	t.Run("500 mapea a ErrServiceInterrupted", func(t *testing.T) {
		adapter, _ := statusAdapter(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		if _, err := adapter.CheckStatus(context.Background()); !errors.Is(err, ErrServiceInterrupted) {
			t.Errorf("CheckStatus(500): error %v no es ErrServiceInterrupted", err)
		}
	})

	t.Run("JSON inválido mapea a ErrServiceInterrupted", func(t *testing.T) {
		adapter, _ := statusAdapter(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{no-json`))
		}))
		if _, err := adapter.CheckStatus(context.Background()); !errors.Is(err, ErrServiceInterrupted) {
			t.Errorf("CheckStatus(json inválido): error %v no es ErrServiceInterrupted", err)
		}
	})

	t.Run("transporte caído mapea a ErrServiceInterrupted", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		server.Close()
		adapter := NewRomeroPublicClientAdapter(server.URL, server.Client(), func() string { return "" })
		if _, err := adapter.CheckStatus(context.Background()); !errors.Is(err, ErrServiceInterrupted) {
			t.Errorf("CheckStatus(caído): error %v no es ErrServiceInterrupted", err)
		}
	})

	t.Run("el doble de prueba devuelve lo configurado", func(t *testing.T) {
		fake := &RomeroFakeStatusAdapter{
			Status: SystemStatus{Status: "ok", Version: "9.9.9"},
		}
		got, err := fake.CheckStatus(context.Background())
		if err != nil || got.Version != "9.9.9" {
			t.Errorf("fake = %+v, %v", got, err)
		}
		fake.Err = ErrAccessDenied
		if _, err := fake.CheckStatus(context.Background()); !errors.Is(err, ErrAccessDenied) {
			t.Errorf("fake con error = %v, want ErrAccessDenied", err)
		}
	})
}
