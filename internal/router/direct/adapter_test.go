package direct

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/config"
	"github.com/CodeMonkReaper/gandalf-wizard/internal/router"
)

const sentinel = "sentinel-credential-value"

// tlsServer starts an in-memory HTTPS server and returns a transport that
// trusts its certificate.
func tlsServer(t *testing.T, handler http.Handler) (*httptest.Server, *http.Transport) {
	t.Helper()
	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	return srv, &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}
}

func apiExecutor(protocol, baseURL string) config.RouterExecutor {
	return config.RouterExecutor{
		ID:            "api-x",
		Kind:          config.RouterKindAPI,
		Authorized:    true,
		Protocol:      protocol,
		BaseURL:       baseURL,
		CredentialEnv: "TEST_API_KEY",
	}
}

func cliExecutor() config.RouterExecutor {
	return config.RouterExecutor{ID: "vibe", Kind: config.RouterKindCLI, Authorized: true}
}

func execution(ex config.RouterExecutor, model, effort string) router.Execution {
	return router.Execution{
		Executor: ex,
		Profile: config.RouterProfile{
			ID:         "p1",
			ExecutorID: ex.ID,
			Model:      model,
		},
		Effort: effort,
		Task:   "Resume el diseño del módulo de pagos",
	}
}

func adapterWith(getenv func(string) string, tr http.RoundTripper) *Adapter {
	return &Adapter{Getenv: getenv, Transport: tr}
}

// recordingHandler captures one request and answers with a canned response.
type recordingHandler struct {
	status int
	body   string
	mu     chan *http.Request
	got    atomic.Int64
	block  <-chan struct{}
}

func (h *recordingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.got.Add(1)
	if h.block != nil {
		<-h.block
	}
	body, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(strings.NewReader(string(body)))
	h.mu <- r.Clone(r.Context())
	w.WriteHeader(h.status)
	w.Write([]byte(h.body))
}

func (h *recordingHandler) request() *http.Request {
	select {
	case r := <-h.mu:
		return r
	case <-time.After(2 * time.Second):
		return nil
	}
}

func bodyJSON(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("leer cuerpo: %v", err)
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		t.Fatalf("cuerpo no json: %q", raw)
	}
	return m
}

// TestAdapter covers the direct API adapter port (AC1, AC2, AC5, AC6, AC11-AC13, AC15, AC16, AC18).
func TestAdapter(t *testing.T) {
	cases := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "probe rechaza ejecutor no api",
			run: func(t *testing.T) {
				a := adapterWith(nil, nil)
				p := a.Probe(context.Background(), cliExecutor())
				if p.Installed {
					t.Fatal("un ejecutor cli no debe reportarse instalado por este adaptador")
				}
				if p.Reason == "" {
					t.Fatal("falta el motivo de rechazo")
				}
			},
		},
		{
			name: "probe rechaza protocolo no soportado",
			run: func(t *testing.T) {
				a := adapterWith(nil, nil)
				p := a.Probe(context.Background(), apiExecutor("opencode-xyz", "https://api.example"))
				if p.Installed {
					t.Fatal("un protocolo no soportado no debe reportarse instalado")
				}
				if p.Reason == "" {
					t.Fatal("falta el motivo de rechazo")
				}
			},
		},
		{
			name: "probe reporta credencial configurada",
			run: func(t *testing.T) {
				a := adapterWith(func(string) string { return sentinel }, nil)
				p := a.Probe(context.Background(), apiExecutor(ProtocolMistral, "https://api.example"))
				if !p.Installed {
					t.Fatal("la api debe reportarse instalada")
				}
				if p.Auth != router.AuthReady {
					t.Fatalf("esperaba auth ready, got %s", p.Auth)
				}
				if p.TextOnly != router.TextOnlyEnforced {
					t.Fatalf("esperaba text-only enforced por diseño, got %s", p.TextOnly)
				}
				if p.Reason == "" {
					t.Fatal("falta el motivo del sondeo")
				}
			},
		},
		{
			name: "probe credencial declarada ausente",
			run: func(t *testing.T) {
				a := adapterWith(func(string) string { return "" }, nil)
				p := a.Probe(context.Background(), apiExecutor(ProtocolMistral, "https://api.example"))
				if p.Auth != router.AuthMissing {
					t.Fatalf("esperaba auth missing, got %s", p.Auth)
				}
			},
		},
		{
			name: "probe sin credencial declarada",
			run: func(t *testing.T) {
				a := adapterWith(nil, nil)
				ex := apiExecutor(ProtocolMistral, "https://api.example")
				ex.CredentialEnv = ""
				p := a.Probe(context.Background(), ex)
				if p.Auth != router.AuthUnknown {
					t.Fatalf("esperaba auth unknown sin variable declarada, got %s", p.Auth)
				}
			},
		},
		{
			name: "discover siempre unavailable",
			run: func(t *testing.T) {
				a := adapterWith(nil, nil)
				_, err := a.Discover(context.Background(), apiExecutor(ProtocolOpenAI, "https://api.example"))
				if !errors.Is(err, router.ErrDiscoveryUnavailable) {
					t.Fatalf("esperaba ErrDiscoveryUnavailable, got %v", err)
				}
			},
		},
		{
			name: "discover rechaza ejecutor no api",
			run: func(t *testing.T) {
				a := adapterWith(nil, nil)
				if _, err := a.Discover(context.Background(), cliExecutor()); err == nil {
					t.Fatal("esperaba error para ejecutor no soportado")
				}
			},
		},
		{
			name: "execute rechaza ejecutor no api",
			run: func(t *testing.T) {
				a := adapterWith(nil, nil)
				_, err := a.Execute(context.Background(), execution(cliExecutor(), "m", ""))
				var sf *router.SafeFailure
				if !errors.As(err, &sf) {
					t.Fatalf("esperaba SafeFailure, got %v", err)
				}
			},
		},
		{
			name: "execute rechaza protocolo no soportado",
			run: func(t *testing.T) {
				a := adapterWith(func(string) string { return sentinel }, nil)
				_, err := a.Execute(context.Background(), execution(apiExecutor("opencode-xyz", "https://api.example"), "m", ""))
				var sf *router.SafeFailure
				if !errors.As(err, &sf) || sf.Kind != router.FailureUnavailable {
					t.Fatalf("esperaba unavailable, got %v", err)
				}
			},
		},
		{
			name: "execute sin credencial declarada no envia",
			run: func(t *testing.T) {
				h := &recordingHandler{status: 200, body: "{}", mu: make(chan *http.Request, 4)}
				srv, tr := tlsServer(t, h)
				a := adapterWith(func(string) string { return sentinel }, tr)
				ex := apiExecutor(ProtocolMistral, srv.URL)
				ex.CredentialEnv = ""
				_, err := a.Execute(context.Background(), execution(ex, "m", ""))
				var sf *router.SafeFailure
				if !errors.As(err, &sf) || sf.Kind != router.FailureUnavailable {
					t.Fatalf("esperaba unavailable sin credencial declarada, got %v", err)
				}
				if h.got.Load() != 0 {
					t.Fatal("se envió una petición sin credencial declarada")
				}
			},
		},
		{
			name: "execute credencial no configurada no envia",
			run: func(t *testing.T) {
				h := &recordingHandler{status: 200, body: "{}", mu: make(chan *http.Request, 4)}
				srv, tr := tlsServer(t, h)
				a := adapterWith(func(string) string { return "" }, tr)
				_, err := a.Execute(context.Background(), execution(apiExecutor(ProtocolMistral, srv.URL), "m", ""))
				var sf *router.SafeFailure
				if !errors.As(err, &sf) || sf.Kind != router.FailureUnavailable {
					t.Fatalf("esperaba unavailable sin credencial configurada, got %v", err)
				}
				if h.got.Load() != 0 {
					t.Fatal("se envió una petición sin credencial configurada")
				}
			},
		},
		{
			name: "execute openai responses construye la peticion y extrae texto",
			run: func(t *testing.T) {
				resp := `{"output":[{"type":"reasoning","summary":[]},{"type":"message","role":"assistant","content":[{"type":"output_text","text":"respuesta "},{"type":"output_text","text":"final"}]}]}`
				h := &recordingHandler{status: 200, body: resp, mu: make(chan *http.Request, 4)}
				srv, tr := tlsServer(t, h)
				a := adapterWith(func(string) string { return sentinel }, tr)
				text, err := a.Execute(context.Background(), execution(apiExecutor(ProtocolOpenAI, srv.URL), "gpt-5.2", ""))
				if err != nil {
					t.Fatalf("ejecución inesperada: %v", err)
				}
				if text != "respuesta final" {
					t.Fatalf("texto inesperado: %q", text)
				}
				r := h.request()
				if r == nil {
					t.Fatal("sin petición registrada")
				}
				if r.Method != "POST" {
					t.Fatalf("método inesperado: %s", r.Method)
				}
				if r.URL.Path != "/responses" {
					t.Fatalf("ruta inesperada: %s", r.URL.Path)
				}
				if got := r.Header.Get("Authorization"); got != "Bearer "+sentinel {
					t.Fatalf("credencial inesperada: %q", got)
				}
				if r.Header.Get("Content-Type") == "" {
					t.Fatal("falta content-type")
				}
				body := bodyJSON(t, r)
				if body["model"] != "gpt-5.2" {
					t.Fatalf("modelo inesperado: %v", body["model"])
				}
				if body["input"] != "Resume el diseño del módulo de pagos" {
					t.Fatalf("tarea inesperada: %v", body["input"])
				}
				if _, ok := body["tools"]; ok {
					t.Fatal("la petición declara herramientas")
				}
				if _, ok := body["stream"]; ok {
					t.Fatal("la petición pide streaming")
				}
				if _, ok := body["reasoning"]; ok {
					t.Fatal("se envió esfuerzo sin declarar")
				}
			},
		},
		{
			name: "execute openai traduce esfuerzo declarado",
			run: func(t *testing.T) {
				h := &recordingHandler{status: 200, body: `{"output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}]}`, mu: make(chan *http.Request, 4)}
				srv, tr := tlsServer(t, h)
				a := adapterWith(func(string) string { return sentinel }, tr)
				if _, err := a.Execute(context.Background(), execution(apiExecutor(ProtocolOpenAI, srv.URL), "gpt-5.2", "high")); err != nil {
					t.Fatalf("ejecución inesperada: %v", err)
				}
				body := bodyJSON(t, h.request())
				reasoning, ok := body["reasoning"].(map[string]any)
				if !ok || reasoning["effort"] != "high" {
					t.Fatalf("falta el esfuerzo traducido: %v", body["reasoning"])
				}
			},
		},
		{
			name: "execute mistral chat completions construye y extrae",
			run: func(t *testing.T) {
				resp := `{"choices":[{"message":{"role":"assistant","content":"primera"}},{"message":{"role":"assistant","content":"respuesta final"}}]}`
				h := &recordingHandler{status: 200, body: resp, mu: make(chan *http.Request, 4)}
				srv, tr := tlsServer(t, h)
				a := adapterWith(func(string) string { return sentinel }, tr)
				text, err := a.Execute(context.Background(), execution(apiExecutor(ProtocolMistral, srv.URL), "mistral-large-latest", ""))
				if err != nil {
					t.Fatalf("ejecución inesperada: %v", err)
				}
				if text != "respuesta final" {
					t.Fatalf("texto inesperado: %q", text)
				}
				r := h.request()
				if r.URL.Path != "/chat/completions" {
					t.Fatalf("ruta inesperada: %s", r.URL.Path)
				}
				if got := r.Header.Get("Authorization"); got != "Bearer "+sentinel {
					t.Fatalf("credencial inesperada: %q", got)
				}
				body := bodyJSON(t, r)
				if body["model"] != "mistral-large-latest" {
					t.Fatalf("modelo inesperado: %v", body["model"])
				}
				msgs, ok := body["messages"].([]any)
				if !ok || len(msgs) != 1 {
					t.Fatalf("mensajes inesperados: %v", body["messages"])
				}
				m := msgs[0].(map[string]any)
				if m["role"] != "user" || m["content"] != "Resume el diseño del módulo de pagos" {
					t.Fatalf("mensaje inesperado: %v", m)
				}
			},
		},
		{
			name: "execute mistral omite esfuerzo sin campo en el protocolo",
			run: func(t *testing.T) {
				h := &recordingHandler{status: 200, body: `{"choices":[{"message":{"content":"ok"}}]}`, mu: make(chan *http.Request, 4)}
				srv, tr := tlsServer(t, h)
				a := adapterWith(func(string) string { return sentinel }, tr)
				if _, err := a.Execute(context.Background(), execution(apiExecutor(ProtocolMistral, srv.URL), "m", "high")); err != nil {
					t.Fatalf("ejecución inesperada: %v", err)
				}
				body := bodyJSON(t, h.request())
				for _, k := range []string{"reasoning_effort", "effort", "reasoning", "thinking"} {
					if _, ok := body[k]; ok {
						t.Fatalf("chat completions no declara esfuerzo pero se envió %s: %v", k, body[k])
					}
				}
			},
		},
		{
			name: "execute extension compatible por configuracion",
			run: func(t *testing.T) {
				h := &recordingHandler{status: 200, body: `{"choices":[{"message":{"content":"desde el compatible"}}]}`, mu: make(chan *http.Request, 4)}
				srv, tr := tlsServer(t, h)
				a := adapterWith(func(string) string { return sentinel }, tr)
				ex := apiExecutor(ProtocolMistral, srv.URL)
				ex.ID = "opencode-compatible"
				text, err := a.Execute(context.Background(), execution(ex, "some-model", ""))
				if err != nil {
					t.Fatalf("ejecución inesperada: %v", err)
				}
				if text != "desde el compatible" {
					t.Fatalf("texto inesperado: %q", text)
				}
				if h.request() == nil {
					t.Fatal("la extensión compatible no recibió petición")
				}
			},
		},
		{
			name: "execute gemini generatecontent construye y extrae",
			run: func(t *testing.T) {
				resp := `{"candidates":[{"content":{"parts":[{"text":"pensamiento","thought":true},{"text":"respuesta final"}]}}]}`
				h := &recordingHandler{status: 200, body: resp, mu: make(chan *http.Request, 4)}
				srv, tr := tlsServer(t, h)
				a := adapterWith(func(string) string { return sentinel }, tr)
				text, err := a.Execute(context.Background(), execution(apiExecutor(ProtocolGemini, srv.URL), "gemini-2.5 pro", ""))
				if err != nil {
					t.Fatalf("ejecución inesperada: %v", err)
				}
				if text != "respuesta final" {
					t.Fatalf("texto inesperado: %q", text)
				}
				r := h.request()
				want := "/models/" + "gemini-2.5%20pro" + ":generateContent"
				if r.URL.EscapedPath() != want {
					t.Fatalf("ruta inesperada: %s", r.URL.EscapedPath())
				}
				if got := r.Header.Get("x-goog-api-key"); got != sentinel {
					t.Fatalf("credencial inesperada: %q", got)
				}
				if got := r.Header.Get("Authorization"); got != "" {
					t.Fatalf("gemini no usa bearer: %q", got)
				}
				body := bodyJSON(t, r)
				contents, ok := body["contents"].([]any)
				if !ok || len(contents) != 1 {
					t.Fatalf("contents inesperados: %v", body["contents"])
				}
				c := contents[0].(map[string]any)
				if c["role"] != "user" {
					t.Fatalf("rol inesperado: %v", c["role"])
				}
				parts, ok := c["parts"].([]any)
				if !ok || len(parts) != 1 {
					t.Fatalf("parts inesperados: %v", c["parts"])
				}
				if parts[0].(map[string]any)["text"] != "Resume el diseño del módulo de pagos" {
					t.Fatalf("tarea inesperada: %v", parts[0])
				}
				if _, ok := body["tools"]; ok {
					t.Fatal("la petición declara herramientas")
				}
			},
		},
		{
			name: "execute gemini traduce presupuesto de pensamiento entero",
			run: func(t *testing.T) {
				h := &recordingHandler{status: 200, body: `{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`, mu: make(chan *http.Request, 4)}
				srv, tr := tlsServer(t, h)
				a := adapterWith(func(string) string { return sentinel }, tr)
				if _, err := a.Execute(context.Background(), execution(apiExecutor(ProtocolGemini, srv.URL), "gemini-2.5-pro", "128")); err != nil {
					t.Fatalf("ejecución inesperada: %v", err)
				}
				body := bodyJSON(t, h.request())
				gen, ok := body["generationConfig"].(map[string]any)
				if !ok {
					t.Fatalf("falta generationConfig: %v", body)
				}
				th, ok := gen["thinkingConfig"].(map[string]any)
				if !ok || th["thinkingBudget"] != float64(128) {
					t.Fatalf("presupuesto inesperado: %v", gen["thinkingConfig"])
				}
			},
		},
		{
			name: "execute gemini omite esfuerzo no entero",
			run: func(t *testing.T) {
				h := &recordingHandler{status: 200, body: `{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`, mu: make(chan *http.Request, 4)}
				srv, tr := tlsServer(t, h)
				a := adapterWith(func(string) string { return sentinel }, tr)
				if _, err := a.Execute(context.Background(), execution(apiExecutor(ProtocolGemini, srv.URL), "gemini-2.5-pro", "high")); err != nil {
					t.Fatalf("ejecución inesperada: %v", err)
				}
				body := bodyJSON(t, h.request())
				if _, ok := body["generationConfig"]; ok {
					t.Fatalf("se envió configuración de generación sin presupuesto traducible: %v", body["generationConfig"])
				}
			},
		},
		{
			name: "execute trata salida malformada",
			run: func(t *testing.T) {
				for _, out := range []string{"no json", "{}", `{"output":[{"type":"reasoning"}]}`, `{"choices":[{"message":{"content":""}}]}`, `{"candidates":[{"content":{"parts":[{"text":"","thought":true}]}}]}`} {
					h := &recordingHandler{status: 200, body: out, mu: make(chan *http.Request, 4)}
					srv, tr := tlsServer(t, h)
					a := adapterWith(func(string) string { return sentinel }, tr)
					_, err := a.Execute(context.Background(), execution(apiExecutor(ProtocolMistral, srv.URL), "m", ""))
					var sf *router.SafeFailure
					if !errors.As(err, &sf) || sf.Kind != router.FailureInvalidOutput {
						t.Fatalf("salida %q: esperaba invalid-output, got %v", out, err)
					}
				}
			},
		},
		{
			name: "execute trata fallo de proveedor redactado",
			run: func(t *testing.T) {
				h := &recordingHandler{status: 500, body: `{"error":{"message":"` + sentinel + ` broke"}}`, mu: make(chan *http.Request, 4)}
				srv, tr := tlsServer(t, h)
				a := adapterWith(func(string) string { return sentinel }, tr)
				_, err := a.Execute(context.Background(), execution(apiExecutor(ProtocolOpenAI, srv.URL), "m", ""))
				var sf *router.SafeFailure
				if !errors.As(err, &sf) || sf.Kind != router.FailureExecutorError {
					t.Fatalf("esperaba executor-error, got %v", err)
				}
				if strings.Contains(sf.Reason, sentinel) {
					t.Fatalf("el motivo filtra el centinela: %q", sf.Reason)
				}
			},
		},
		{
			name: "execute rechaza origen no https sin enviar",
			run: func(t *testing.T) {
				h := &recordingHandler{status: 200, body: "{}", mu: make(chan *http.Request, 4)}
				srv := httptest.NewServer(h)
				t.Cleanup(srv.Close)
				a := adapterWith(func(string) string { return sentinel }, nil)
				_, err := a.Execute(context.Background(), execution(apiExecutor(ProtocolMistral, srv.URL), "m", ""))
				var sf *router.SafeFailure
				if !errors.As(err, &sf) || sf.Kind != router.FailureUnavailable {
					t.Fatalf("esperaba unavailable para origen no https, got %v", err)
				}
				if h.got.Load() != 0 {
					t.Fatal("se envió la credencial a un origen no https")
				}
			},
		},
		{
			name: "execute redirect no genera peticion al destino",
			run: func(t *testing.T) {
				target := &recordingHandler{status: 200, body: "{}", mu: make(chan *http.Request, 4)}
				targetSrv, _ := tlsServer(t, target)
				redirect := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					http.Redirect(w, r, targetSrv.URL+"/leak", http.StatusFound)
				})
				srv, tr := tlsServer(t, redirect)
				a := adapterWith(func(string) string { return sentinel }, tr)
				_, err := a.Execute(context.Background(), execution(apiExecutor(ProtocolMistral, srv.URL), "m", ""))
				var sf *router.SafeFailure
				if !errors.As(err, &sf) || sf.Kind != router.FailureExecutorError {
					t.Fatalf("esperaba executor-error ante redirect, got %v", err)
				}
				if target.got.Load() != 0 {
					t.Fatal("el redirect generó una petición al destino")
				}
			},
		},
		{
			name: "execute cancelacion termina sin reintento ni fuga",
			run: func(t *testing.T) {
				block := make(chan struct{})
				defer close(block)
				h := &recordingHandler{status: 200, body: "{}", mu: make(chan *http.Request, 4), block: block}
				srv, tr := tlsServer(t, h)
				a := adapterWith(func(string) string { return sentinel }, tr)
				ctx, cancel := context.WithCancel(context.Background())
				time.AfterFunc(150*time.Millisecond, cancel)
				start := time.Now()
				_, err := a.Execute(ctx, execution(apiExecutor(ProtocolMistral, srv.URL), "m", ""))
				elapsed := time.Since(start)
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("esperaba context.Canceled, got %v", err)
				}
				if elapsed > 10*time.Second {
					t.Fatalf("la cancelación no terminó a tiempo: %s", elapsed)
				}
				if h.got.Load() != 1 {
					t.Fatalf("esperaba exactamente 1 intento, got %d", h.got.Load())
				}
				if strings.Contains(fmt.Sprint(err), sentinel) {
					t.Fatal("el error filtra el centinela")
				}
			},
		},
		{
			name: "execute plazo vencido termina sin reintento",
			run: func(t *testing.T) {
				block := make(chan struct{})
				defer close(block)
				h := &recordingHandler{status: 200, body: "{}", mu: make(chan *http.Request, 4), block: block}
				srv, tr := tlsServer(t, h)
				a := adapterWith(func(string) string { return sentinel }, tr)
				ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
				defer cancel()
				start := time.Now()
				_, err := a.Execute(ctx, execution(apiExecutor(ProtocolMistral, srv.URL), "m", ""))
				elapsed := time.Since(start)
				var sf *router.SafeFailure
				if !errors.As(err, &sf) || sf.Kind != router.FailureTimeout {
					t.Fatalf("esperaba SafeFailure timeout, got %v", err)
				}
				if elapsed > 10*time.Second {
					t.Fatalf("el plazo no terminó a tiempo: %s", elapsed)
				}
				if h.got.Load() != 1 {
					t.Fatalf("esperaba exactamente 1 intento, got %d", h.got.Load())
				}
				if strings.Contains(sf.Reason, sentinel) {
					t.Fatal("el motivo filtra el centinela")
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t)
		})
	}
}
