---
type: plan
title: Plan — CLI gandalf, configuración y doctor
description: Plan técnico a nivel de estructura, contratos, flujo de datos, desglose de tareas T001..T010 y estrategia de pruebas para cerrar Foundation 0.1.
tags: [sdd, plan, foundation, cli, config, doctor]
timestamp: 2026-11-18T02:20:00Z
topic: sdd
slug: cli-gandalf
status: approved-autopilot
issue: https://github.com/CodeMonkReaper/gandalf-wizard/issues/5
---

# Plan — CLI gandalf, configuración y doctor

> Spec: [../specs/cli-gandalf.md](../specs/cli-gandalf.md) · Constitution: [../constitution.md](../constitution.md) · Issue: [#5](https://github.com/CodeMonkReaper/gandalf-wizard/issues/5) · Status: approved-autopilot
> Last updated: 2026-11-18

## 0. Global Constraints

- **Stack y cero dependencias externas:** Go `1.27.3`, módulo `github.com/CodeMonkReaper/gandalf-wizard` en la raíz (`go.mod`), usando exclusivamente la librería estándar (`flag`, `encoding/json`, `net/http`, `net/url`, `os`, `os/exec`, `os/user`, `path/filepath`, `strings`, `time`, `io`, `fmt`, `context`, `errors`, `regexp`). Cero paquetes de terceros en `go.mod` (`constitution v1.1.0`, `spec §Qué debe respetar`).
- **Controles de calidad obligatorios:** `gofmt -l .` (salida vacía), `go vet ./...` y `go test ./...` (`02-DOCS/wiki/sdd/config.yaml`, `02-DOCS/wiki/stack/go.md`).
- **Contrato de códigos de salida (`spec §Qué resultado obtiene #5`):**
  - `ExitOK = 0` (`status == ok` o `status == empty`, `doctor` con `failures == 0`, `version`, `help`, `config show`, `config init` exitoso)
  - `ExitInternalError = 1` (fallo interno inesperado)
  - `ExitUsageError = 2` (sin argumentos, subcomando desconocido, flag inválido, `memory query` con pregunta vacía)
  - `ExitConfigError = 3` (archivo de configuración con JSON inválido, URL/timeout/ID de proyecto inválido, clave de credencial prohibida en configuración, proyecto ausente en `memory query` / `scope_required`, o `config init` cuando `.gandalf.json` ya existe)
  - `ExitAccessDenied = 4` (`status == access_denied` desde ROMERO)
  - `ExitPartialQuery = 5` (`status == partial` desde ROMERO)
  - `ExitIncompleteQuery = 6` (`status == incomplete` o ROMERO inalcanzable/timeout en `memory query`)
  - `ExitDoctorFailed = 7` (`doctor` completado con `failures >= 1`)
- **Higiene de credenciales y secretos (`constitution §Principios #5`, `spec AC3/AC5`):**
  - `ROMERO_API_KEY` se lee exclusivamente desde el entorno (`Getenv("ROMERO_API_KEY")`), nunca desde archivos ni flags.
  - Todo archivo de configuración que contenga una clave de credencial (`api_key`, `token`, `secret`, `password`, `credential`, `romero_api_key`, sin distinguir mayúsculas/minúsculas, en raíz o dentro de objetos anidados como `romero`) se rechaza con `ExitConfigError = 3` citando la ruta del archivo y el motivo `"credencial no permitida en archivo de configuración"`, sin imprimir nunca el valor del secreto.
  - Ni `stdout` ni `stderr` deben contener el valor de `ROMERO_API_KEY` en ningún subcomando ni ruta de error.
- **Formato de configuración y reglas de validación:**
  - Archivo de proyecto: `.gandalf.json` (buscado desde el directorio de trabajo actual hacia arriba hasta la raíz del volumen; gana el más cercano).
  - Archivo global: `filepath.Join(UserConfigDir, "gandalf", "config.json")`.
  - Esquema JSON admitido: `{"project": "<id>", "caller": "<id>", "romero": {"base_url": "<url>", "timeout": "<duration>"}}`.
  - Validación de `project`: no vacío tras `TrimSpace`, coincide con `^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`.
  - Validación de `romero.base_url`: parseable con `url.Parse`, esquema `http` o `https`, `Host` no vacío.
  - Validación de `romero.timeout`: parseable con `time.ParseDuration` y `> 0`.
  - Claves desconocidas no prohibidas en JSON: producen un aviso (`Warnings`) con nombre de archivo y clave; nunca bloquean la ejecución.
  - Valores predeterminados: `romero.base_url = "http://127.0.0.1:8420"`, `romero.timeout = 10s`, `caller = os/user` (nombre de usuario del sistema operativo cuando no está definido en capas superiores).
- **Frontera de solo lectura y soberanía humana (`constitution §Principios #10`, `spec AC8`):**
  - `gandalf` nunca escribe en `.agents/`, `.codex/`, `.vibe/` ni `.rsc/`.
  - Únicamente `gandalf config init --project <id>` escribe en disco, y solo crea `.gandalf.json` en el directorio de trabajo cuando ese archivo aún no existe (`os.OpenFile(..., os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)`).
  - Las operaciones hacia ROMERO son estrictamente de solo lectura (`POST /api/v1/memories/search` con `scopes: ["project_shared"]` e `include_superseded: true`, `GET /api/v1/memories/{id}` y `GET /api/v1/system/status`).
- **Autoría humana y trazabilidad (`constitution §Principios #8`, `v1.2.0`):** commits exclusivamente con la identidad Git del propietario sin pies `Co-Authored-By`, vinculados al [Issue #5](https://github.com/CodeMonkReaper/gandalf-wizard/issues/5) y su sub-Issue correspondiente.

## 1. Context & constraints

- **Criterios de aceptación que moldean el diseño:** `spec AC1`–`AC13` en [specs/cli-gandalf.md](../specs/cli-gandalf.md):
  - `AC1` (enrutamiento CLI, `help`, `version`, códigos `0`/`2`);
  - `AC2` (precedencia de 5 capas `flag → entorno → proyecto → global → predeterminado` y origen trazable);
  - `AC3` (ausencia absoluta de fuga de `ROMERO_API_KEY` en `stdout`/`stderr`);
  - `AC4` (`config init --project` exclusivo sin sobrescritura y validación de proyecto);
  - `AC5` (rechazo determinista con código `3` ante JSON inválido, valores inválidos o claves de credencial; aviso no bloqueante ante claves desconocidas);
  - `AC6`–`AC8` (`doctor` de 6 diagnósticos sin cortocircuito, detección solo lectura de harnesses/RSC y salida `0` o `7`);
  - `AC9`–`AC13` (`memory query` sobre `internal/romero` con tablas de referencia, `--json` `schema_version: "1"`, `source_ref: null`, `entries: []`, opacidad de denegación, 0 llamadas sin proyecto y corte por timeout en código `6`).
- **Herencia de Foundation 0.1 (`internal/romero`):** reutiliza `WorkspaceContext`, `MemoryQueryOrchestrator`, `ProvenancePresenter`, `RomeroPublicClientAdapter`, `RomeroReadPort` y las etiquetas canónicas ya probadas en [specs/consulta-memoria-romero.md](../specs/consulta-memoria-romero.md).
- **Fuera de alcance que el diseño no debe invadir:** escritura o sincronización en configuraciones de Codex/Vibe/Antigravity, registros de skills/patrones, escritura en ROMERO, almacenamiento de tokens en archivos o keychain, daemon/MCP/TUI/instaladores (`spec §Fuera de alcance`).

## 2. Architecture

```text
[ cmd/gandalf/main.go (INTERNAL) ]
         |
         v  (os.Args, os.Getenv, Getwd, UserConfigDir, Stdout, Stderr)
[ internal/cli (INTERNAL) ]
    |          |           |            |
    v          v           v            v
[ config ] [ doctor ]  [ romero ]   [ render ]
    |       |    |     (Orchestrator  (Table /
    |       |    |      + Presenter    JSON v1)
    |       |    |      + Adapter)
    |       v    |          |
    |  [ harness ]          |
    |  (LookPath + .rsc.json)
    v                       v
[ .gandalf.json /       [ ROMERO Service (EXTERNAL) ]
  config.json ]           POST /api/v1/memories/search
                          GET  /api/v1/memories/{id}
                          GET  /api/v1/system/status
```

- **`cmd/gandalf`** (internal) — Binario CLI mínimo que delega `os.Args[1:]`, flujos estándar, entorno y directorio actual a `internal/cli.Run` y retorna `os.Exit(code)`.
- **`internal/cli`** (internal) — Despachador de subcomandos (`help`, `version`, `config show`, `config init`, `doctor`, `memory query`), analizador de flags, mapeador de códigos de salida (`0`–`7`) e inyección de `Dependencies` para pruebas en proceso.
- **`internal/config`** (internal) — Resolución de configuración en 5 capas (`flag > entorno > proyecto > global > predeterminado`), descubrimiento ascendente de `.gandalf.json`, rechazo estricto de claves de credencial, avisos de claves desconocidas e inicialización exclusiva de `.gandalf.json`.
- **`internal/render`** (internal) — Formateo determinista de salida humana (tablas Markdown sin color/emojis) y JSON estable (`schema_version: "1"`) para `version`, `config show`, `doctor` y `memory query`.
- **`internal/harness`** (internal) — Detector de solo lectura de ejecutables `codex`, `vibe`, `antigravity`/`agy` vía `lookPath` y presencia de `.rsc.json`.
- **`internal/romero`** (internal, existente + extensión `RomeroStatusPort`) — Mantiene la consulta de solo lectura existente y añade `CheckStatus(ctx)` contra `GET /api/v1/system/status` para alimentar `doctor`.
- **`internal/doctor`** (internal) — Ejecuta en orden fijo los 6 diagnósticos (`configuración`, `proyecto`, `romero.url`, `credencial`, `romero.conexión`, `harness`) sin cortocircuitar cuando alguno falla.
- **`ROMERO Service`** (external) — Servicio HTTP REST externo consumido exclusivamente en modo lectura.

**Top architectural decision:** **Runner CLI inyectable en proceso (`internal/cli.Run(ctx, args, deps) int`) sobre paquetes de dominio aislados y disjuntos.**
- *Alternativas consideradas:*
  1. Lógica directa en `cmd/gandalf/main.go` leyendo `os.Getenv`/`os.Stdin` directamente. Descartada porque obliga a compilar subprocesos lentos en cada prueba de aceptación y dificulta aislar tareas entre trabajadores menores.
  2. Paquetes independientes por responsabilidad detrás de `internal/cli.Dependencies` (elegida). Permite ejecutar toda la suite `AC1`–`AC13` dentro del mismo proceso en milisegundos y asignar sub-Issues con archivos disjuntos por tarea.

## 3. Interfaces & contracts

```text
internal/cli (exitcodes.go):
  const ExitOK = 0, ExitInternalError = 1, ExitUsageError = 2, ExitConfigError = 3,
        ExitAccessDenied = 4, ExitPartialQuery = 5, ExitIncompleteQuery = 6, ExitDoctorFailed = 7
  type Dependencies struct { Version, WorkDir, UserConfigDir string; Stdout, Stderr io.Writer;
    Getenv func(string) string; LookPath func(string) (string, error);
    DefaultCaller func() string; HTTPClient *http.Client;
    ReadPort romero.RomeroReadPort; StatusPort romero.RomeroStatusPort }
  cli.Run(ctx context.Context, args []string, deps Dependencies) -> int
```

```text
internal/config (types.go, load.go, init.go):
  Origin: "flag" | "entorno" | "proyecto" | "global" | "predeterminado"
  Load(input LoadInput) -> (EffectiveConfig, error)
    - invariant: never stores or returns ROMERO_API_KEY value; only APIKeyPresent (bool) and APIKeyOrigin
    - error: *ConfigError{Path, Reason, Err} for invalid JSON, invalid URL/timeout/project, or credential keys
  InitProjectConfig(dir, projectID string) -> (createdPath string, error)
    - invariant: validates projectID; opens .gandalf.json with O_CREATE|O_EXCL (never overwrites)
```

```text
internal/romero (status_port.go, status_client.go):
  type RomeroStatusPort interface { CheckStatus(ctx context.Context) (SystemStatus, error) }
  (*RomeroPublicClientAdapter).CheckStatus(ctx context.Context) -> (SystemStatus, error)
    - invariant: calls GET /api/v1/system/status; maps 401/403 -> ErrAccessDenied; transport/non-200 -> ErrServiceInterrupted
```

```text
internal/harness (types.go, detect.go):
  Detect(projectRoot string, lookPath func(string) (string, error)) -> DetectionReport
    - invariant: purely read-only; probes codex, vibe, antigravity (fallback agy) and os.Stat(<projectRoot>/.rsc.json)
```

```text
internal/doctor (types.go, doctor.go):
  Run(ctx context.Context, input Input) -> Report
    - invariant: always returns exactly 6 CheckResult items in fixed order:
      ["configuración", "proyecto", "romero.url", "credencial", "romero.conexión", "harness"]
    - invariant: a failure in any check never skips subsequent checks; counts Failures and Warnings
```

```text
internal/render (types.go, render.go):
  RenderVersion(w io.Writer, view VersionView, asJSON bool) error
  RenderConfigShow(w io.Writer, view ConfigShowView, asJSON bool) error
  RenderDoctor(w io.Writer, report doctor.Report, asJSON bool) error
  RenderMemoryQuery(w io.Writer, view MemoryQueryView, asJSON bool) error
    - invariant: JSON always sets schema_version="1", command, non-nil slices ([] not null), and nullable source_ref (*string)
    - invariant: human output uses plain Markdown tables / [OK]/[FAIL]/[WARN] lines without ANSI color or emoji
```

## 4. Data model & flow

**Entities**

- **`EffectiveConfig`** — Contiene `Project`, `Caller`, `RomeroBaseURL`, `RomeroTimeout` como `TrackedValue[T]` (`Value`, `Origin`), además de `APIKeyPresent`, `APIKeyOrigin`, `ProjectConfigPath`, `GlobalConfigPath` y `Warnings`. No almacena nunca el texto del secreto.
- **`SystemStatus`** — `Status`, `Version`, `Database`, `ActiveProfile` devueltos por `GET /api/v1/system/status`.
- **`DetectionReport`** — Lista ordenada de `HarnessStatus` (`codex`, `vibe`, `antigravity`) más `RSCPresent` y `RSCConfigPath`.
- **`doctor.Report`** — Lista de 6 `CheckResult` (`Name`, `Status`, `Detail`, `Remediation`) con totales `Failures` y `Warnings`.
- **`MemoryQueryJSON`** — `schema_version`, `command`, `project`, `status`, `status_text`, `degraded`, `degradation_notice`, `entries` (`[]MemoryQueryJSONEntry` con `source_ref *string`).

**Primary flow** (`gandalf memory query [--json] "<pregunta>"`)

1. `cli.Run` parsea el subcomando `memory query`, flags de anulación (`--project`, `--caller`, `--romero-url`, `--timeout`, `--json`) y el argumento posicional `<pregunta>`. Si falta `<pregunta>` o hay flag inválido, escribe en `Stderr` y devuelve `ExitUsageError (2)`.
2. Invoca `config.Load(...)`. Si falla (JSON inválido, clave de credencial, valor inválido), escribe el error sanitizado en `Stderr` y devuelve `ExitConfigError (3)` con `0` llamadas a ROMERO.
3. Si `cfg.Project.Value == ""`, retorna `ExitConfigError (3)` con `"Identifica el proyecto activo antes de consultar ROMERO"` y `0` llamadas a ROMERO.
4. Construye `WorkspaceContext{ActiveProjectID: cfg.Project.Value, CallerIdentity: cfg.Caller.Value}`, obtiene el `RomeroReadPort` (inyectado o `NewRomeroPublicClientAdapter`) y ejecuta `orch.QueryProjectMemory(ctxTimeout, ws, query)` aplicando `cfg.RomeroTimeout.Value`.
5. Transforma el `MemoryQueryOutcome` con `romero.Present(outcome)`, llama a `render.RenderMemoryQuery(deps.Stdout, ...)` y mapea `outcome.Status` a `ExitOK (0)` (`ok`/`empty`), `ExitAccessDenied (4)`, `ExitPartialQuery (5)`, `ExitIncompleteQuery (6)` o `ExitConfigError (3)` (`scope_required`).

- **Consistency boundaries:** Cada invocación CLI es sin estado en memoria; la única escritura persistente permitida en todo el sistema es `config init --project <id>` creando `.gandalf.json` de forma exclusiva (`O_EXCL`).
- **Migration impact:** `none` — no existen esquemas previos ni bases de datos propias de Gandalf.

## 5. Testing strategy

| Acceptance criterion | Level | Asserts | Fakes / mocks |
| --- | --- | --- | --- |
| `AC1` (`--help`, `version`, desconocido, sin args) | unit + acceptance | Subcomandos listados, `gandalf <v>`, stderr y exit codes `0`/`2`. | `cli.Dependencies` en memoria con buffers `Stdout`/`Stderr`. |
| `AC2` (precedencia 5 capas en `config show`) | unit + acceptance | `flag > entorno > proyecto > global > predeterminado` y columna `Origen`. | Directorios temporales + `Getenv` controlado + flags. |
| `AC3` (cero fuga de `ROMERO_API_KEY` centinela) | acceptance + security | Valor centinela ausente en `stdout` y `stderr` en todos los comandos (`--json`, éxito y error); `config show` muestra `presente`. | `Getenv` con secreto centinela + `httptest.Server` 200/401/500. |
| `AC4` (`config init --project`) | unit + acceptance | Crea `.gandalf.json` con `project`, código `0`; existente no se toca y código `3`; ID inválido no crea archivo y código `3`. | `t.TempDir()`. |
| `AC5` (JSON/valor/credencial inválido y clave desconocida) | unit + acceptance | Indica archivo y motivo, `0` llamadas ROMERO, código `3`; clave desconocida emite aviso sin bloquear. | Archivos temporales + espía HTTP que falla si recibe petición. |
| `AC6` (`doctor` todo OK) | unit + acceptance | 6 checks en `OK` (`failures=0`), código `0`. | `httptest.Server` `/api/v1/system/status` 200 + `lookPath` simulado + `.rsc.json`. |
| `AC7` (`doctor` con fallos sin cortocircuito) | unit + acceptance | `FAIL` con línea `→` de remedio en los checks afectados, 6 checks ejecutados igual, código `7`. | `httptest.Server` 401/caído, `ROMERO_API_KEY` ausente, sin proyecto. |
| `AC8` (`doctor` detección harnesses/RSC solo lectura) | unit + acceptance | Detecta ejecutables simulados y `.rsc.json`; snapshot de árbol de archivos idéntico antes/después. | `lookPath` simulado + snapshot recursivo de `t.TempDir()`. |
| `AC9` (`memory query` tabla referencia 1) | acceptance | Salida exacta de tabla Markdown para `Faro` (`M1`, `F1`), código `0`. | `httptest.Server` / `RomeroFakeAdapter` con dataset Faro. |
| `AC10` (`memory query --json` referencia 2) | acceptance | JSON válido con `schema_version: "1"`, `source_ref: null` si falta, `entries: []` si vacío. | `httptest.Server` / `RomeroFakeAdapter`. |
| `AC11` (todos los `OutcomeStatus` y confidencialidad) | acceptance | Etiquetas y códigos `0`/`4`/`5`/`6`/`3`; sin resumen ni IDs en `access_denied`/`scope_required`. | `RomeroFakeAdapter` y `httptest.Server` por cada estado. |
| `AC12` (`memory query` sin proyecto) | acceptance | `0` peticiones HTTP recibidas y salida `3`. | `httptest.Server` con contador de peticiones. |
| `AC13` (`memory query` timeout 1s) | acceptance | Termina en `< 3s` con `"Consulta incompleta"` y código `6`. | `httptest.Server` que bloquea respuesta hasta cancelar contexto. |

- **Línea divisoria e2e/integración:** `AC1`–`AC13` se ejecutan en proceso vía `internal/cli.Run` sobre servidor `httptest.Server` real (probando HTTP real, serialización JSON y timeouts) y `t.TempDir()`; opcionalmente una prueba de humo manual contra ROMERO lite cuando existan variables de entorno reales.
- **Dependencia que NO debe falsearse:** el flujo de red HTTP/JSON en `internal/cli/acceptance_test.go` usa `net/http/httptest.Server` y `RomeroPublicClientAdapter` reales (no solo el fake en memoria) para verificar `AC3`, `AC6`, `AC7`, `AC9`, `AC10`, `AC12` y `AC13`.

## 6. Sequencing & dependencies

1. **T001 — Esqueleto de contratos (`contracts-first`)** — depends on: none — `[serial]`
2. **T002 (`config/load`), T003 (`config/init`), T004 (`render`), T005 (`harness`), T006 (`romero/status_client`)** — depend on: `T001` — `[parallelizable]` entre sí (archivos y paquetes disjuntos)
3. **T007 (`doctor`)** — depends on: `T002, T005, T006` — `[serial]`
4. **T008 (`cli` + `cmd/gandalf`)** — depends on: `T002, T003, T004, T007` — `[serial]`
5. **T009 (`acceptance_test` AC1–AC13)** — depends on: `T008` — `[serial]`
6. **T010 (gate final Go)** — depends on: `T001..T009` — `[serial]`

## 7. Risks & open decisions

**Risks**

| Risk | Trigger | Impact | Mitigation / spike to retire it |
| --- | --- | --- | --- |
| Fuga accidental de `ROMERO_API_KEY` en mensajes de error o JSON | Un struct o error de `config`/`doctor`/`http` imprime el token o `Authorization` header. | Incumple `constitution §P5` y `AC3`. | `EffectiveConfig` no almacena el valor del secreto (solo `APIKeyPresent bool`); prueba centinela en `AC3` recorre todos los comandos en éxito y error. |
| Deriva de formato entre `render` y referencias de la spec | Diferencias de espacios o columnas en tabla/JSON respecto a la spec. | Falla `AC9`–`AC10` o rompe agentes consumidores. | Pruebas de tabla y JSON comparan contra las cadenas y estructuras exactas de `spec §Qué resultado obtiene`; Q1 queda como `review-after`. |
| Desajuste en `GET /api/v1/system/status` | Cambio futuro en payload de estado de ROMERO. | Falla `doctor` en conexión viva. | `SystemStatus` mapea los campos estables de `pkg/romeroclient/types.go` (`status`, `version`, `database`, `active_profile`) comprobados en rev `3f46f9a`. |
| Bloqueo por timeout lento en `AC13` | `http.Client.Timeout` ignora o supera el timeout configurado. | `memory query` tarda demasiado ante ROMERO colgado. | Pasar `context.WithTimeout(ctx, cfg.RomeroTimeout.Value)` en cada petición de `memory query` y `doctor`; prueba cronometrada `< 3s` con `--timeout 1s`. |

**Open decisions**

- Suposiciones `Q1`–`Q4` registradas en [specs/cli-gandalf.md](../specs/cli-gandalf.md) como `review-after` en [progress/cli-gandalf.md](../progress/cli-gandalf.md).

## Tasks
<!-- generated by tasks on 2026-11-18; IDs are stable, do not renumber -->

| ID | [P] | Task | Done-check | Depends-on | Trace | Risk |
| --- | --- | --- | --- | --- | --- | --- |
| T001 |  | Implement contracts-first types, exit codes, and sentinel errors in `internal/config/types.go`, `internal/render/types.go`, `internal/harness/types.go`, `internal/romero/status_port.go`, `internal/doctor/types.go`, `internal/cli/exitcodes.go`, `internal/cli/contracts_test.go` | `go test -v ./internal/...` | — | AC1, AC2, AC3, AC6, AC9, AC10, AC11 | 1 |
| T002 | [P] | Implement 5-layer config loader, upward `.gandalf.json` discovery, credential-key rejection, and unknown-key warnings in `internal/config/load.go` and `internal/config/load_test.go` | `go test -v ./internal/config -run TestLoadConfig` | T001 | AC2, AC3, AC5 | 1 |
| T003 | [P] | Implement `InitProjectConfig` (`config init --project`) with exclusive file creation and project ID validation in `internal/config/init.go` and `internal/config/init_test.go` | `go test -v ./internal/config -run TestInitProjectConfig` | T001 | AC4 | 1 |
| T004 | [P] | Implement human table and `--json` (`schema_version: "1"`) formatters in `internal/render/render.go` and `internal/render/render_test.go` | `go test -v ./internal/render -run TestRender` | T001 | AC1, AC2, AC3, AC6, AC7, AC9, AC10, AC11 | 1 |
| T005 | [P] | Implement read-only harness & RSC detector in `internal/harness/detect.go` and `internal/harness/detect_test.go` | `go test -v ./internal/harness -run TestDetectHarnesses` | T001 | AC8 | 1 |
| T006 | [P] | Implement `CheckStatus` (`GET /api/v1/system/status`) on `RomeroPublicClientAdapter` and fake status double in `internal/romero/status_client.go` and `internal/romero/status_client_test.go` | `go test -v ./internal/romero -run TestRomeroStatusClient` | T001 | AC6, AC7 | 1 |
| T007 |  | Implement 6-check non-short-circuiting `doctor` runner in `internal/doctor/doctor.go` and `internal/doctor/doctor_test.go` | `go test -v ./internal/doctor -run TestDoctorChecks` | T002, T005, T006 | AC3, AC6, AC7, AC8 | 1 |
| T008 |  | Implement CLI dispatcher (`help`, `version`, `config show`, `config init`, `doctor`, `memory query`), flag overrides, exit-code mapping, and `cmd/gandalf/main.go` in `internal/cli/run.go`, `internal/cli/run_test.go`, and `cmd/gandalf/main.go` | `go test -v ./internal/cli -run TestCLIDispatch` | T002, T003, T004, T007 | AC1, AC2, AC3, AC4, AC5, AC6, AC7, AC8, AC9, AC10, AC11, AC12, AC13 | 1 |
| T009 |  | Implement end-to-end acceptance suite (`AC1`–`AC13`, sentinel secret leak check, read-only filesystem check, and timeout check) in `internal/cli/acceptance_test.go` | `go test -v ./internal/cli -run TestAcceptance_AC1_to_AC13` | T008 | AC1, AC2, AC3, AC4, AC5, AC6, AC7, AC8, AC9, AC10, AC11, AC12, AC13 | 1 |
| T010 |  | Run full quality gate across the Go module | `gofmt -l .` (empty output) AND `go vet ./...` AND `go test ./...` | T001, T002, T003, T004, T005, T006, T007, T008, T009 | all ACs | — |

**T001 — Interfaces**
- Consumes: `go.mod` (`go 1.27.3`), `internal/romero` (`RomeroReadPort`, `MemoryQueryOutcome`, `PresentedMemoryView`).
- Produces: contratos base en `internal/config/types.go`, `internal/render/types.go`, `internal/harness/types.go`, `internal/romero/status_port.go`, `internal/doctor/types.go`, `internal/cli/exitcodes.go` y `internal/cli/contracts_test.go`.

**T002 — Interfaces**
- Consumes: `internal/config/types.go` (`Origin`, `TrackedValue`, `EffectiveConfig`, `FlagOverrides`, `LoadInput`, `ConfigError`, `ErrInvalidConfig`, `ErrInvalidProjectID`, `ErrForbiddenCredentialKey`, constantes de entorno/rutas/defaults).
- Produces: `func Load(input LoadInput) (EffectiveConfig, error)` en `internal/config/load.go` y pruebas en `internal/config/load_test.go` (5 capas en orden `flag > entorno > proyecto > global > predeterminado`, búsqueda ascendente de `.gandalf.json`, rechazo de claves de credencial en cualquier nivel JSON, validación URL/timeout/project ID, avisos de claves desconocidas, fallback de `caller` al usuario del SO).

**T003 — Interfaces**
- Consumes: `internal/config/types.go` (`ProjectConfigFileName`, `ConfigError`, `ErrConfigExists`, `ErrInvalidProjectID`, `ValidateProjectID`).
- Produces: `func InitProjectConfig(dir, projectID string) (string, error)` en `internal/config/init.go` y pruebas en `internal/config/init_test.go` (valida `projectID`, crea `.gandalf.json` con `{"project":"<id>"}` usando `O_WRONLY|O_CREATE|O_EXCL` sin sobrescribir jamás, devuelve `*ConfigError` envolviendo `ErrConfigExists` si ya existe).

**T004 — Interfaces**
- Consumes: `internal/render/types.go` (`SchemaVersion`, `VersionView`, `ConfigShowView`, `MemoryQueryView`, `VersionJSON`, `ConfigShowJSON`, `DoctorJSON`, `MemoryQueryJSON`, `MemoryQueryJSONEntry`), `internal/doctor/types.go` (`Report`, `CheckResult`, `CheckStatus`), `internal/romero` (`LabelNoMatches`, `LabelMissingSource`, `OutcomeStatus`, `PresentedMemoryView`).
- Produces: `RenderVersion(w io.Writer, view VersionView, asJSON bool) error`, `RenderConfigShow(w io.Writer, view ConfigShowView, asJSON bool) error`, `RenderDoctor(w io.Writer, report doctor.Report, asJSON bool) error`, `RenderMemoryQuery(w io.Writer, view MemoryQueryView, asJSON bool) error` en `internal/render/render.go` y pruebas en `internal/render/render_test.go` comparando contra las referencias exactas de la spec (incluyendo `source_ref: null`, `entries: []`, sin colores ni emojis).

**T005 — Interfaces**
- Consumes: `internal/harness/types.go` (`HarnessStatus`, `DetectionReport`).
- Produces: `func Detect(projectRoot string, lookPath func(string) (string, error)) DetectionReport` en `internal/harness/detect.go` y pruebas en `internal/harness/detect_test.go` (busca `codex`, `vibe`, `antigravity` con fallback `agy` vía `lookPath` y comprueba `<projectRoot>/.rsc.json` en solo lectura sin escribir nada).

**T006 — Interfaces**
- Consumes: `internal/romero/status_port.go` (`SystemStatus`, `RomeroStatusPort`), `internal/romero/port.go` (`ErrAccessDenied`, `ErrServiceInterrupted`), `internal/romero/rest_client.go` (`RomeroPublicClientAdapter`, `do`, `decodeCapped`).
- Produces: `func (a *RomeroPublicClientAdapter) CheckStatus(ctx context.Context) (SystemStatus, error)` en `internal/romero/status_client.go` (GET `/api/v1/system/status`, mapea `401`/`403` a `ErrAccessDenied`, otros errores/estados a `ErrServiceInterrupted`) más doble de prueba `RomeroFakeStatusAdapter` y pruebas en `internal/romero/status_client_test.go`.

**T007 — Interfaces**
- Consumes: `internal/doctor/types.go` (`CheckStatus`, `CheckResult`, `Report`, `Input`), `internal/config/types.go` (`EffectiveConfig`), `internal/harness/types.go` (`DetectionReport`), `internal/romero/status_port.go` (`RomeroStatusPort`), `internal/romero/port.go` (`ErrAccessDenied`).
- Produces: `func Run(ctx context.Context, input Input) Report` en `internal/doctor/doctor.go` y pruebas en `internal/doctor/doctor_test.go` (ejecuta siempre las 6 comprobaciones `configuración`, `proyecto`, `romero.url`, `credencial`, `romero.conexión`, `harness` sin cortocircuito, emite remedio concreto en cada `FAIL` y cuenta `Failures` y `Warnings`).

**T008 — Interfaces**
- Consumes: `internal/cli/exitcodes.go` (`Exit*`, `Dependencies`), `internal/config.Load`, `internal/config.InitProjectConfig`, `internal/harness.Detect`, `internal/doctor.Run`, `internal/render.Render*`, `internal/romero` (`NewMemoryQueryOrchestrator`, `NewRomeroPublicClientAdapter`, `Present`).
- Produces: `func Run(ctx context.Context, args []string, deps Dependencies) int` en `internal/cli/run.go`, pruebas unitarias en `internal/cli/run_test.go` y punto de entrada `cmd/gandalf/main.go` (manejo de `help`/`--help`, `version`, `config show`, `config init --project`, `doctor`, `memory query`, flags `--project`/`--caller`/`--romero-url`/`--timeout`/`--json`, timeout de contexto en llamadas ROMERO y mapeo estricto de códigos `0`–`7` sin filtrar nunca `ROMERO_API_KEY`).

**T009 — Interfaces**
- Consumes: `internal/cli.Run`, `internal/cli.Dependencies`, `internal/cli.Exit*`, `net/http/httptest`.
- Produces: `internal/cli/acceptance_test.go` (`TestAcceptance_AC1_to_AC13`) probando en proceso los 13 criterios de aceptación (`AC1`–`AC13`), la ausencia de fuga del secreto centinela en todos los comandos/errores, la invariancia del árbol de archivos en `doctor` y el timeout `< 3s` en `AC13`.

## Review Workload Forecast

| Dimension | Forecast | Why |
| --- | --- | --- |
| Estimated changed lines | ~1200–1600 | 7 paquetes internos (`config`, `render`, `harness`, `romero`, `doctor`, `cli`, `cmd/gandalf`), pruebas unitarias y suite de aceptación `AC1`–`AC13` (supera `line_budget: 400`; se ejecuta por sub-Issues independientes). |
| Files / areas | ~19 archivos nuevos | 6 paquetes + 1 binario CLI + artefactos ASDD (`file_budget: 12` en PR final mitigado dividiendo el trabajo en 9 tareas verificadas por separado). |
| Risk | Máximo tier `1`; `0` tareas tier 2 | `rsc risk` clasifica los archivos Go como `default` (tier `1`, `1` revisor por tarea). |
| Suggested delivery | `ask-on-risk` | Contratos `T001` primero en rama de trabajo, luego implementación y revisión por cada sub-Issue `T002`–`T009`, cerrando con `verify` + `review` en el PR de la épica #5. |

```json result-envelope
{
  "status": "complete",
  "executive_summary": "Plan técnico, interfaces exactas y desglose T001..T010 generados para la épica #5 (cli-gandalf), listos para analyze y publicación de sub-Issues.",
  "artifact": "02-DOCS/wiki/sdd/plans/cli-gandalf.md",
  "next_recommended": "analyze",
  "risk": "medium",
  "skill_resolution": {
    "used": ["plan", "tasks", "go"],
    "missing": [],
    "fallback": [],
    "compact_rules": [
      "Every task needs a runnable done-check, a criterion trace and a risk forecast.",
      "Keep all dependencies inside Go standard library."
    ]
  },
  "evidence": [
    "02-DOCS/wiki/sdd/specs/cli-gandalf.md (PASS rsc spec gate)",
    "02-DOCS/wiki/sdd/plans/cli-gandalf.md (§0..§7 + T001..T010)"
  ]
}
```
