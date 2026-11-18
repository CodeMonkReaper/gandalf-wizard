package router

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/CodeMonkReaper/gandalf-wizard/internal/config"
)

// runAdapter is a deterministic adapter double for orchestration tests.
type runAdapter struct {
	id      string
	mu      sync.Mutex
	events  []string
	calls   int
	execs   []Execution
	text    string
	err     error
	block   chan struct{} // when non-nil, Execute waits for it to close
	started chan struct{} // when non-nil, signals each Execute start
	waitCtx bool          // when true, Execute waits for ctx.Done() and fails
}

func (a *runAdapter) Probe(ctx context.Context, ex config.RouterExecutor) Probe {
	return readyProbe
}

func (a *runAdapter) Discover(ctx context.Context, ex config.RouterExecutor) ([]Candidate, error) {
	return nil, ErrDiscoveryUnavailable
}

func (a *runAdapter) Execute(ctx context.Context, exec Execution) (string, error) {
	a.mu.Lock()
	a.calls++
	a.execs = append(a.execs, exec)
	a.events = append(a.events, a.id+"-start")
	started := a.started
	a.mu.Unlock()
	if started != nil {
		select {
		case started <- struct{}{}:
		default:
		}
	}
	if a.waitCtx {
		<-ctx.Done()
		a.mu.Lock()
		a.events = append(a.events, a.id+"-end")
		a.mu.Unlock()
		return "", &SafeFailure{Kind: FailureTimeout, Reason: "tiempo límite"}
	}
	if a.block != nil {
		<-a.block
	}
	a.mu.Lock()
	a.events = append(a.events, a.id+"-end")
	a.mu.Unlock()
	if a.err != nil {
		return "", a.err
	}
	return a.text, nil
}

func (a *runAdapter) snapshot() (int, []string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]string, len(a.events))
	copy(out, a.events)
	return a.calls, out
}

func runChoice(profileID, executorID, model, effort string) Choice {
	return Choice{
		Profile:     config.RouterProfile{ID: profileID, ExecutorID: executorID, Model: model, Efforts: map[string]string{"high": "alto"}},
		Effort:      effort,
		Explanation: "posición 1 del orden declarado",
	}
}

func runPolicy(executors []config.RouterExecutor, timeout time.Duration) config.RouterPolicy {
	return config.RouterPolicy{Executors: executors, Profiles: []config.RouterProfile{}, AttemptTimeout: timeout}
}

func apiExecutor(id string, authorized bool, credentialEnv string) config.RouterExecutor {
	return config.RouterExecutor{ID: id, Kind: config.RouterKindAPI, Authorized: authorized, Protocol: "mistral", BaseURL: "https://api.example", CredentialEnv: credentialEnv}
}

const runSentinel = "sentinel-run-xyz"

// revokedAdapter captures the adapter of the revoked-executor case so its
// check function can assert it was never invoked.
var revokedAdapter *runAdapter

// TestRun covers the orchestration state machine (AC5-AC13, AC18).
func TestRun(t *testing.T) {
	cases := []struct {
		name  string
		run   func(t *testing.T) Result
		check func(t *testing.T, r Result)
	}{
		{
			name: "exito con un solo intento",
			run: func(t *testing.T) Result {
				a := &runAdapter{id: "a1", text: "respuesta final"}
				reg := NewRegistry()
				reg.Register("api-x", a)
				pol := runPolicy([]config.RouterExecutor{apiExecutor("api-x", true, "")}, time.Minute)
				d := Decision{Ordered: []Choice{runChoice("api-x/m1", "api-x", "m1", "high")}}
				return Run(context.Background(), pol, d, "tarea", reg, nil)
			},
			check: func(t *testing.T, r Result) {
				if r.Status != StatusOK {
					t.Fatalf("estado inesperado: %s", r.Status)
				}
				if r.Output != "respuesta final" || r.Profile.ID != "api-x/m1" || r.Effort != "high" {
					t.Fatalf("resultado inesperado: %+v", r)
				}
				if len(r.Attempts) != 1 || r.Attempts[0].Outcome != AttemptOK {
					t.Fatalf("intentos inesperados: %+v", r.Attempts)
				}
			},
		},
		{
			name: "sin elegibles no llama adaptadores",
			run: func(t *testing.T) Result {
				a := &runAdapter{id: "a1"}
				reg := NewRegistry()
				reg.Register("api-x", a)
				pol := runPolicy([]config.RouterExecutor{apiExecutor("api-x", true, "")}, time.Minute)
				return Run(context.Background(), pol, Decision{}, "tarea", reg, nil)
			},
			check: func(t *testing.T, r Result) {
				if r.Status != StatusNoEligible {
					t.Fatalf("estado inesperado: %s", r.Status)
				}
				if len(r.Attempts) != 0 {
					t.Fatalf("intentos inesperados: %+v", r.Attempts)
				}
			},
		},
		{
			name: "fallo y exito usa fallback",
			run: func(t *testing.T) Result {
				a1 := &runAdapter{id: "a1", err: &SafeFailure{Kind: FailureExecutorError, Reason: "el ejecutor terminó con error"}}
				a2 := &runAdapter{id: "a2", text: "segundo ok"}
				reg := NewRegistry()
				reg.Register("api-x", a1)
				reg.Register("api-y", a2)
				pol := runPolicy([]config.RouterExecutor{apiExecutor("api-x", true, ""), apiExecutor("api-y", true, "")}, time.Minute)
				d := Decision{Ordered: []Choice{
					runChoice("api-x/m1", "api-x", "m1", "high"),
					{Profile: config.RouterProfile{ID: "api-y/m2", ExecutorID: "api-y", Model: "m2"}, Effort: "", Explanation: "posición 2"},
				}}
				return Run(context.Background(), pol, d, "tarea", reg, nil)
			},
			check: func(t *testing.T, r Result) {
				if r.Status != StatusOK || r.Profile.ID != "api-y/m2" {
					t.Fatalf("resultado inesperado: %+v", r)
				}
				if len(r.Attempts) != 2 {
					t.Fatalf("esperaba 2 intentos, got %d", len(r.Attempts))
				}
				if r.Attempts[0].Outcome != AttemptFailed || r.Attempts[1].Outcome != AttemptOK {
					t.Fatalf("outcomes inesperados: %+v", r.Attempts)
				}
				if !strings.Contains(r.Explanation, "fallback") {
					t.Fatalf("explicacion sin fallback: %q", r.Explanation)
				}
			},
		},
		{
			name: "agotamiento conserva intentos en orden",
			run: func(t *testing.T) Result {
				a1 := &runAdapter{id: "a1", err: &SafeFailure{Kind: FailureExecutorError, Reason: "el ejecutor terminó con error"}}
				a2 := &runAdapter{id: "a2", err: &SafeFailure{Kind: FailureUnavailable, Reason: "el ejecutor no está disponible"}}
				reg := NewRegistry()
				reg.Register("api-x", a1)
				reg.Register("api-y", a2)
				pol := runPolicy([]config.RouterExecutor{apiExecutor("api-x", true, ""), apiExecutor("api-y", true, "")}, time.Minute)
				d := Decision{Ordered: []Choice{
					runChoice("api-x/m1", "api-x", "m1", "high"),
					{Profile: config.RouterProfile{ID: "api-y/m2", ExecutorID: "api-y", Model: "m2"}, Explanation: "posición 2"},
				}}
				return Run(context.Background(), pol, d, "tarea", reg, nil)
			},
			check: func(t *testing.T, r Result) {
				if r.Status != StatusExhausted {
					t.Fatalf("estado inesperado: %s", r.Status)
				}
				if len(r.Attempts) != 2 || r.Attempts[0].Profile.ID != "api-x/m1" || r.Attempts[1].Profile.ID != "api-y/m2" {
					t.Fatalf("intentos inesperados: %+v", r.Attempts)
				}
				if r.Output != "" {
					t.Fatalf("agotamiento no debe entregar salida: %q", r.Output)
				}
			},
		},
		{
			name: "manual sin fallback aunque la decision traiga mas perfiles",
			run: func(t *testing.T) Result {
				a1 := &runAdapter{id: "a1", err: &SafeFailure{Kind: FailureExecutorError, Reason: "el ejecutor terminó con error"}}
				a2 := &runAdapter{id: "a2", text: "no debe correr"}
				reg := NewRegistry()
				reg.Register("api-x", a1)
				reg.Register("api-y", a2)
				pol := runPolicy([]config.RouterExecutor{apiExecutor("api-x", true, ""), apiExecutor("api-y", true, "")}, time.Minute)
				d := Decision{Manual: true, Ordered: []Choice{
					runChoice("api-x/m1", "api-x", "m1", "high"),
					{Profile: config.RouterProfile{ID: "api-y/m2", ExecutorID: "api-y", Model: "m2"}, Explanation: "posición 2"},
				}}
				return Run(context.Background(), pol, d, "tarea", reg, nil)
			},
			check: func(t *testing.T, r Result) {
				if r.Status != StatusExhausted || len(r.Attempts) != 1 {
					t.Fatalf("resultado manual inesperado: %+v", r)
				}
			},
		},
		{
			name: "ejecutor revocado registra el motivo y continua en automatico",
			run: func(t *testing.T) Result {
				revokedAdapter = &runAdapter{id: "a1", text: "no debe correr"}
				a2 := &runAdapter{id: "a2", text: "segundo ok"}
				reg := NewRegistry()
				reg.Register("api-x", revokedAdapter)
				reg.Register("api-y", a2)
				pol := runPolicy([]config.RouterExecutor{apiExecutor("api-x", false, ""), apiExecutor("api-y", true, "")}, time.Minute)
				d := Decision{Ordered: []Choice{
					runChoice("api-x/m1", "api-x", "m1", "high"),
					{Profile: config.RouterProfile{ID: "api-y/m2", ExecutorID: "api-y", Model: "m2"}, Explanation: "posición 2"},
				}}
				return Run(context.Background(), pol, d, "tarea", reg, nil)
			},
			check: func(t *testing.T, r Result) {
				if r.Status != StatusOK || r.Profile.ID != "api-y/m2" {
					t.Fatalf("resultado inesperado: %+v", r)
				}
				if len(r.Attempts) == 0 {
					t.Fatal("sin intentos registrados")
				}
				if r.Attempts[0].Outcome != AttemptFailed || r.Attempts[0].Reason != "ejecutor no autorizado" {
					t.Fatalf("intento revocado inesperado: %+v", r.Attempts[0])
				}
				if calls, _ := revokedAdapter.snapshot(); calls != 0 {
					t.Fatal("el ejecutor revocado fue invocado")
				}
			},
		},
		{
			name: "ejecutor ausente de la politica registra el motivo",
			run: func(t *testing.T) Result {
				a1 := &runAdapter{id: "a1", text: "no debe correr"}
				reg := NewRegistry()
				reg.Register("api-x", a1)
				pol := runPolicy(nil, time.Minute)
				d := Decision{Ordered: []Choice{runChoice("api-x/m1", "api-x", "m1", "high")}}
				return Run(context.Background(), pol, d, "tarea", reg, nil)
			},
			check: func(t *testing.T, r Result) {
				if r.Status != StatusExhausted {
					t.Fatalf("estado inesperado: %s", r.Status)
				}
				if len(r.Attempts) == 0 {
					t.Fatal("sin intentos registrados")
				}
				if r.Attempts[0].Reason != "ejecutor no declarado en la política" {
					t.Fatalf("motivo inesperado: %q", r.Attempts[0].Reason)
				}
			},
		},
		{
			name: "cancelacion antes del intento",
			run: func(t *testing.T) Result {
				a := &runAdapter{id: "a1"}
				reg := NewRegistry()
				reg.Register("api-x", a)
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				pol := runPolicy([]config.RouterExecutor{apiExecutor("api-x", true, "")}, time.Minute)
				d := Decision{Ordered: []Choice{runChoice("api-x/m1", "api-x", "m1", "high")}}
				return Run(ctx, pol, d, "tarea", reg, nil)
			},
			check: func(t *testing.T, r Result) {
				if r.Status != StatusCanceled {
					t.Fatalf("estado inesperado: %s", r.Status)
				}
				if len(r.Attempts) != 0 {
					t.Fatalf("intentos inesperados: %+v", r.Attempts)
				}
			},
		},
		{
			name: "cancelacion durante el intento no lanza fallback",
			run: func(t *testing.T) Result {
				block := make(chan struct{})
				a1 := &runAdapter{id: "a1", block: block}
				a2 := &runAdapter{id: "a2", text: "no debe correr"}
				reg := NewRegistry()
				reg.Register("api-x", a1)
				reg.Register("api-y", a2)
				ctx, cancel := context.WithCancel(context.Background())
				pol := runPolicy([]config.RouterExecutor{apiExecutor("api-x", true, ""), apiExecutor("api-y", true, "")}, time.Minute)
				d := Decision{Ordered: []Choice{
					runChoice("api-x/m1", "api-x", "m1", "high"),
					{Profile: config.RouterProfile{ID: "api-y/m2", ExecutorID: "api-y", Model: "m2"}, Explanation: "posición 2"},
				}}
				started := make(chan struct{}, 1)
				a1.started = started
				done := make(chan Result, 1)
				go func() { done <- Run(ctx, pol, d, "tarea", reg, nil) }()
				select {
				case <-started:
				case <-time.After(10 * time.Second):
					t.Fatal("Run no inicio el primer intento")
					return Result{}
				}
				cancel()
				close(block)
				select {
				case r := <-done:
					return r
				case <-time.After(10 * time.Second):
					t.Fatal("Run no termino tras la cancelacion")
					return Result{}
				}
			},
			check: func(t *testing.T, r Result) {
				if r.Status != StatusCanceled {
					t.Fatalf("estado inesperado: %s", r.Status)
				}
				if len(r.Attempts) != 1 || r.Attempts[0].Outcome != AttemptCanceled {
					t.Fatalf("intento cancelado inesperado: %+v", r.Attempts)
				}
			},
		},
		{
			name: "plazo vencido registra tiempo limite y permite fallback",
			run: func(t *testing.T) Result {
				a1 := &runAdapter{id: "a1", waitCtx: true}
				a2 := &runAdapter{id: "a2", text: "segundo ok"}
				reg := NewRegistry()
				reg.Register("api-x", a1)
				reg.Register("api-y", a2)
				pol := runPolicy([]config.RouterExecutor{apiExecutor("api-x", true, ""), apiExecutor("api-y", true, "")}, 50*time.Millisecond)
				d := Decision{Ordered: []Choice{
					runChoice("api-x/m1", "api-x", "m1", "high"),
					{Profile: config.RouterProfile{ID: "api-y/m2", ExecutorID: "api-y", Model: "m2"}, Explanation: "posición 2"},
				}}
				return Run(context.Background(), pol, d, "tarea", reg, nil)
			},
			check: func(t *testing.T, r Result) {
				if r.Status != StatusOK || r.Profile.ID != "api-y/m2" {
					t.Fatalf("resultado inesperado: %+v", r)
				}
				if len(r.Attempts) == 0 {
					t.Fatal("sin intentos registrados")
				}
				if r.Attempts[0].Outcome != AttemptFailed || r.Attempts[0].Reason != "tiempo límite" {
					t.Fatalf("intento con plazo inesperado: %+v", r.Attempts[0])
				}
			},
		},
		{
			name: "el primer adaptador termina antes de lanzar el segundo",
			run: func(t *testing.T) Result {
				block := make(chan struct{})
				a1 := &runAdapter{id: "a1", block: block, err: &SafeFailure{Kind: FailureExecutorError, Reason: "el ejecutor terminó con error"}}
				a2 := &runAdapter{id: "a2", text: "segundo ok"}
				reg := NewRegistry()
				reg.Register("api-x", a1)
				reg.Register("api-y", a2)
				pol := runPolicy([]config.RouterExecutor{apiExecutor("api-x", true, ""), apiExecutor("api-y", true, "")}, time.Minute)
				d := Decision{Ordered: []Choice{
					runChoice("api-x/m1", "api-x", "m1", "high"),
					{Profile: config.RouterProfile{ID: "api-y/m2", ExecutorID: "api-y", Model: "m2"}, Explanation: "posición 2"},
				}}
				started := make(chan struct{}, 1)
				a1.started = started
				done := make(chan Result, 1)
				go func() { done <- Run(context.Background(), pol, d, "tarea", reg, nil) }()
				select {
				case <-started:
				case <-time.After(10 * time.Second):
					t.Fatal("Run no inicio el primer intento")
					return Result{}
				}
				calls, events := a2.snapshot()
				if calls != 0 {
					t.Fatal("el segundo adaptador arranco antes de que terminara el primero")
				}
				if len(events) != 0 {
					t.Fatal("eventos inesperados del segundo adaptador")
				}
				close(block)
				r := <-done
				calls, events = a2.snapshot()
				if calls != 1 {
					t.Fatal("el segundo adaptador no arranco tras terminar el primero")
				}
				_, ev1 := a1.snapshot()
				want := []string{"a1-start", "a1-end", "a2-start", "a2-end"}
				got := append(append([]string{}, ev1...), events...)
				if !equalStrings(got, want) {
					t.Fatalf("orden de eventos inesperado: %v", got)
				}
				return r
			},
			check: func(t *testing.T, r Result) {
				if r.Status != StatusOK {
					t.Fatalf("estado inesperado: %s", r.Status)
				}
			},
		},
		{
			name: "redaccion de credenciales en texto y motivos",
			run: func(t *testing.T) Result {
				a1 := &runAdapter{id: "a1", text: "respuesta que contiene " + runSentinel + " dentro"}
				reg := NewRegistry()
				reg.Register("api-x", a1)
				pol := runPolicy([]config.RouterExecutor{apiExecutor("api-x", true, "TEST_KEY")}, time.Minute)
				d := Decision{Ordered: []Choice{runChoice("api-x/m1", "api-x", "m1", "high")}}
				getenv := func(key string) string {
					if key == "TEST_KEY" {
						return runSentinel
					}
					return ""
				}
				return Run(context.Background(), pol, d, "tarea", reg, getenv)
			},
			check: func(t *testing.T, r Result) {
				if strings.Contains(r.Output, runSentinel) {
					t.Fatalf("la salida revela el centinela: %q", r.Output)
				}
				if !strings.Contains(r.Output, "[redactado]") {
					t.Fatalf("la salida no muestra la redaccion: %q", r.Output)
				}
			},
		},
		{
			name: "errores crudos no se exponen",
			run: func(t *testing.T) Result {
				a1 := &runAdapter{id: "a1", err: errors.New("stderr crudo con " + runSentinel)}
				reg := NewRegistry()
				reg.Register("api-x", a1)
				pol := runPolicy([]config.RouterExecutor{apiExecutor("api-x", true, "TEST_KEY")}, time.Minute)
				d := Decision{Ordered: []Choice{runChoice("api-x/m1", "api-x", "m1", "high")}}
				getenv := func(key string) string {
					if key == "TEST_KEY" {
						return runSentinel
					}
					return ""
				}
				return Run(context.Background(), pol, d, "tarea", reg, getenv)
			},
			check: func(t *testing.T, r Result) {
				if len(r.Attempts) == 0 {
					t.Fatal("sin intentos registrados")
				}
				if r.Attempts[0].Reason != "el ejecutor terminó con error" {
					t.Fatalf("motivo inesperado: %q", r.Attempts[0].Reason)
				}
				if strings.Contains(r.Explanation, runSentinel) {
					t.Fatalf("la explicacion revela el centinela: %q", r.Explanation)
				}
			},
		},
		{
			name: "nunca reintenta el mismo perfil",
			run: func(t *testing.T) Result {
				a1 := &runAdapter{id: "a1", err: &SafeFailure{Kind: FailureExecutorError, Reason: "el ejecutor terminó con error"}}
				reg := NewRegistry()
				reg.Register("api-x", a1)
				pol := runPolicy([]config.RouterExecutor{apiExecutor("api-x", true, "")}, time.Minute)
				d := Decision{Ordered: []Choice{
					runChoice("api-x/m1", "api-x", "m1", "high"),
					runChoice("api-x/m1", "api-x", "m1", "high"),
				}}
				return Run(context.Background(), pol, d, "tarea", reg, nil)
			},
			check: func(t *testing.T, r Result) {
				if len(r.Attempts) != 1 {
					t.Fatalf("esperaba 1 intento, got %d", len(r.Attempts))
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := tc.run(t)
			if tc.check == nil {
				t.Fatal("caso sin comprobacion")
			}
			tc.check(t, r)
		})
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
