---
type: plan
title: Plan — Workflow Runtime 0.3: motor local de workflows declarativos
description: Arquitectura, contratos, modelo de datos, estrategia de pruebas, secuencia y riesgos para Workflow Runtime 0.3.
tags: [sdd, plan, workflow-runtime, odd, gates]
timestamp: 2026-11-20T17:35:00Z
topic: sdd
slug: workflow-runtime-03
status: approved
approved: 2026-11-20
---

# Plan — Workflow Runtime 0.3: motor local de workflows declarativos

> Spec: [../specs/workflow-runtime-03.md](../specs/workflow-runtime-03.md) · Constitución: [../constitution.md](../constitution.md) · Épica: [Issue #40](https://github.com/CodeMonkReaper/gandalf-wizard/issues/40) · Estado: approved (2026-11-20)
> Última actualización: 2026-11-20

## 0. Restricciones globales

- **Stack y dependencias:** Go `1.27.3`, módulo `github.com/CodeMonkReaper/gandalf-wizard`, librería estándar exclusivamente (`go.mod` con cero dependencias externas), TDD estricto (`config.yaml`), controles obligatorios `gofmt -l .`, `go vet ./...` y `go test -count=1 ./...` (incluido `TestRepoSecurityGate`). [Configuración ASDD](../config.yaml) · [Convenciones Go](../../stack/go.md)
- **Autoría y proceso:** autoría Git humana sin `Co-Authored-By` ni pies de generación de IA (Constitución P8); desarrollo en rama aislada `feat/workflow-runtime-03` con sub-Issues vinculados a `#40` e **Informe de Cierre Técnico obligatorio** (revisión técnica + tabla de errores del desarrollo y soluciones aplicadas) antes de pasar al siguiente Issue ([guía de Issues](../../harness/github-issues-workflow.md)); delegación elegible en Mistral Vibe en lotes focalizados de 3–4 archivos (`rsc-vibe-delegation`).
- **Sin escritura en proyecto ni shell arbitrario:** ningún paso edita archivos del repositorio del usuario ni ejecuta comandos arbitrarios de shell; los pasos `router_task` heredan el contrato estricto de solo texto de Router 0.2; todo estado de runtime vive confinado en `.gandalf/workflows/` bajo la raíz del proyecto y queda excluido de Git en `.gitignore`.
- **Cero fugas de secretos:** los 5 patrones deterministas de `TestRepoSecurityGate` (`mstrl_`, `sk-`, `AIza`, `gh[pousr]_`/`github_pat_`, PEM) más los valores en memoria de credenciales declaradas (`ROMERO_API_KEY` y variables de API del router) se redactan en stdout, stderr, `--json`, `status`, `events.jsonl`, `state.json` y artefactos; `no_secrets` en `quality_gate` evalúa sobre la salida cruda antes de persistir, emite `blocked` sin reintentos y nunca admite modo informativo.
- **Catálogo cerrado de `quality_gate` (6 reglas):** `non_empty`, `min_length`/`max_length`, `contains`/`not_contains`, `regex_match`/`regex_forbid`, `json_valid` + `json_required_keys` y `no_secrets`. Modo bloqueante por defecto; el modo informativo aplica solo si se declara explícitamente y nunca sobre `no_secrets` ni criterios obligatorios del objetivo (`spec §Qué podrá hacer el usuario`).
- **Estados exactos:**
  - Corrida no terminales / reanudables: `created`, `validated`, `running`, `awaiting_approval`, `paused`, `failed` (cuando falla un paso recuperable).
  - Corrida terminales (no reanudables): `succeeded`, `partial` (únicamente fallos/omisiones en ramas o pasos explícitamente opcionales con todos los pasos obligatorios y criterios del objetivo cumplidos), `blocked`, `cancelled`.
  - Paso: `pending`, `running`, `awaiting_approval`, `passed`, `failed`, `skipped`, `cancelled`.
  - Gate (`GW-44`): `pass`, `fail`, `blocked`, `needs_human_review`.
- **Parada conservadora `4A` y aprobación vinculada:** al llegar cualquier rama a `human_approval`, no se lanza ningún paso nuevo en ninguna rama (solo concluyen ordenadamente los pasos `running`); `workflow approve` exige `--checkpoint <huella>` coincidente y `--reason` obligatorio en `reject`; `resume` bloquea antes de lanzar pasos nuevos si queda cualquier puerta obligatoria pendiente.
- **Códigos de salida y esquema JSON:** salida `--json` con `schema_version: "1"`; se conservan intactos códigos `0–10` de Foundation/Router/Sync (`0` OK, `1` interno, `2` uso, `3` configuración/definición/preflight/`run-id` no encontrado, `4` acceso denegado, `5–7` ROMERO/doctor, `8–9` router, `10` cancelado) y se reservan sin colisión para workflow: `11` (`awaiting_approval`), `12` (`paused`), `13` (`partial`), `14` (`blocked`), `15` (`failed`), `16` (`conflict` — concurrencia ocupada, huella obsoleta, definición alterada o estado terminal/puerta cerrada).

## 1. Contexto y restricciones

- **Criterios de aceptación rectores:** `spec AC-01` (preflight de solo lectura y proveedores condicionales `S6`), `AC-02` (paralelismo real y barrera de dependencias), `AC-03` (6 reglas de `quality_gate`, modo informativo y bloqueo de dependientes), `AC-04` (ciclos acotados `max_iterations`), `AC-05` (parada conservadora `4A`, `--checkpoint` y rechazo), `AC-06` (integridad de definición y alcance estricto de aprobaciones), `AC-07` (`paused`/`failed` vs terminales y reanudación idempotente), `AC-08` (cancelación cooperativa y exclusión mutua), `AC-09` (evaluación contra objetivo `GW-05` y frontera `partial`/`failed`) y `AC-10` (aislamiento, redacción de secretos y no regresión). [Especificación aprobada](../specs/workflow-runtime-03.md)
- **Restricciones constitucionales y previas:** [constitución v1.2.0](../constitution.md) (`P2` decisiones explícitas, `P3` ASDD, `P4` evidencia observable, `P5` mínimo privilegio y cero secretos, `P6` controles deterministas comprobables, `P7` Issues/PR, `P8` autoría humana). Reutiliza el despachador `internal/cli.Run` con `Dependencies` inyectables y los paquetes existentes `internal/config`, `internal/romero`, `internal/router`, `internal/render` y el patrón de confinamiento/bloqueo de `internal/syncfs` (adaptado de `internal/syncfs` sin romper su contrato).
- **Fuera de alcance protegido (`spec §Fuera de alcance`):** sin edición de archivos del proyecto ni ejecución de comandos de shell por modelos, sin compilador multi-harness (`Fase 04`), sin Composer automático ni descomposición interactiva ODD (`GW-05.2`), sin subworkflows anidados, sin demonio residente ni ejecución distribuida, sin presupuestos/cuotas concurrentes de Router 0.3, sin `LLM-as-a-Judge` bloqueante, sin escritura en ROMERO ni panel `GW-57`.

## 2. Arquitectura

```text
[ CLI y Render ] (INTERNO: internal/cli, internal/render)
       |
       v
[ Orquestador DAG y Runtime ] (INTERNO: internal/workflow/engine)
   |          |            |               |                 |
   v          v            v               v                 v
[ Schema ] [ Gates ]  [ Store OS ]   [ MemoryPort ]    [ RouterPort ]
(INTERNO)  (INTERNO)  (INTERNO)      (INTERNO)         (INTERNO)
                           |               |                 |
                           v               v                 v
               [ .gandalf/workflows/ ] [ ROMERO REST ] [ Router 0.2 CLIs/APIs ]
                     (EXTERNO FS)        (EXTERNO)          (EXTERNO)
```

- **CLI y Render (`internal/cli`, `internal/render`, internos):** parsean subcomandos `workflow <validate|run|status|approve|resume|cancel>`, ensamblan dependencias, traducen estados del motor a códigos de salida (`0–3`, `10`, `11–16`) y emiten tabla legible o `--json` redactado.
- **Esquema y Validador Preflight (`internal/workflow/schema`, interno):** parsean definiciones declarativas estrictas, calculan `definition_sha256`, validan el bloque ODD, verifican grafo acíclico, límites de ciclo, referencias entrada/salida, reglas de gate y disponibilidad condicional de proveedores (`S6`) en solo lectura.
- **Evaluador de Gates, Secretos y Objetivo (`internal/workflow/gates`, interno):** ejecutan de forma pura las 6 reglas deterministas de `quality_gate`, detectan/redactan secretos antes de persistir o emitir salida y calculan el veredicto final contra `ObjectiveSpec` aplicando la frontera `Q2` (`succeeded` vs `partial` vs `failed`).
- **Almacén Confinado y Bloqueo OS (`internal/workflow/store`, interno):** aíslan el espacio `.gandalf/workflows/runs/<run-id>/`, gestionan exclusión mutua por corrida (`run.lock`), escrituras atómicas de checkpoints (`state.json`) y artefactos redactados, bitácora append-only (`events.jsonl`) y el marcador cooperativo de cancelación (`cancel.req`).
- **Orquestador DAG y Runtime (`internal/workflow/engine`, interno):** planifica pasos listos respetando concurrencia acotada, gestiona ciclos de corrección `max_iterations`, aplica la parada conservadora `4A` en `human_approval`, responde a `Ctrl+C` y `cancel.req`, y coordina `Approve` y `Resume` sin repetir pasos `passed`.
- **Puertos MemoryPort y RouterPort (internos) → ROMERO / Router 0.2 (externos):** interfaces mínimas definidas del lado consumidor en `internal/workflow/engine` adaptadas en `internal/cli` hacia `romero.MemoryQueryOrchestrator` y `router.Select` + `router.Run`, permitiendo pruebas deterministas sin red ni cuota.

**Decisiones estructurales clave:**

1. **Motor en primer plano con bloqueo exclusivo por corrida (`run.lock`) + señal cooperativa (`cancel.req`)** en lugar de demonio residente: cumple `spec §Fuera de alcance` (cero procesos residentes), garantiza exclusión mutua determinista en Windows y POSIX para `run`/`resume`/`approve` (`AC-08`) y permite que `workflow cancel` detenga una corrida `running` desde otra invocación sin tomar el lock de ejecución ni corromper checkpoints.
2. **Definición canónica en JSON estricto (`schema_version: "1"`, rechazo de campos desconocidos)** en lugar de añadir una dependencia YAML externa: preserva `go.mod` en librería estándar pura (como `internal/config` y `internal/projectsync`), elimina ambigüedades de tipado implícito en YAML y mantiene el modelo IR desacoplado tras `schema.LoadAndValidate` para admitir adaptadores YAML futuros sin tocar el núcleo.
3. **Almacén confinado por corrida bajo `.gandalf/workflows/runs/<run-id>/`** (confinado con `os.Root`, escritura atómica temp+rename y exclusión Git) en lugar de una base de datos embebida: evita drivers nativos/CGO en Windows, preserva legibilidad auditable de artefactos y eventos por corrida, e imita las primitivas de confinamiento probadas en `internal/syncfs`.

## 3. Interfaces y contratos

> Las firmas Go concretas de `### Interfaces por tarea` son el contrato canónico de implementación; esta sección resume sus invariantes y códigos de salida.

### 3.1 Esquema y Preflight (`internal/workflow/schema`)

```text
schema.LoadAndValidate(defPath: string, opts: ValidateOptions) -> ValidatedWorkflow | ValidationErrors
  - precondition: solo lectura; opts incluye CheckProviders (bool) y disponibilidad local de Memory/Router
  - invariant: 0 escrituras en disco, 0 llamadas de red/ejecución a ROMERO o Router
  - postcondition:
      - rechaza campos desconocidos, DAG con ciclos no declarados, dependencias/artefactos rotos
      - exige objective completo (id, version, statement, deliverables, success_criteria)
      - exige max_iterations >= 1 en todo ciclo de corrección y no_secrets/objective_criteria bloqueantes
      - valida ROMERO solo si existe >= 1 paso memory_query; Router solo si >= 1 paso router_task (S6)
      - calcula definition_sha256 canónico sobre el archivo validado
```

### 3.2 Evaluador determinista de Gates y Objetivo (`internal/workflow/gates`)

```text
gates.EvaluateGate(spec: GateSpec, rawOutput: string, secretValues: []string) -> GateResult
  - invariant: función pura y determinista; no muta estado ni red
  - postcondition:
      - evalúa primero no_secrets contra patrones fijos y secretValues sobre rawOutput crudo
      - si no_secrets falla -> GateResult{Status: "blocked", Retryable: false} sin revelar el valor
      - aplica non_empty, min_length/max_length, contains/not_contains, regex_match/regex_forbid, json_valid/json_required_keys
      - si falla regla normal -> Status: "fail" (Retryable: !spec.Informative)

gates.EvaluateObjective(obj: ObjectiveSpec, steps: map[string]StepOutcome, artifacts: map[string]string) -> ObjectiveVerdict
  - postcondition:
      - si algún paso obligatorio está failed/cancelled/blocked o falta algún entregable/criterio -> failed (o blocked si hubo bloqueo duro)
      - si todos los pasos obligatorios y criterios del objetivo pasan, pero >= 1 paso/rama opcional es failed/skipped -> partial
      - en caso contrario -> succeeded (conservando warnings de gates informativos)
```

### 3.3 Almacén confinado y Bloqueo OS (`internal/workflow/store`)

```text
store.OpenRoot(projectRoot: string) -> *WorkflowStore | ErrUnsafePath
store.CreateRun(def: ValidatedWorkflow) -> (runID: string, *RunState) | error
store.AcquireExclusive(runID: string) -> (*RunLock, error /* ErrRunBusy si otro proceso avanza/decide */)
store.SaveArtifact(runID: string, stepID: string, iter: int, name: string, sanitizedContent: string) -> ArtifactRef | error
store.WriteCheckpoint(runID: string, state: RunState) -> CheckpointDigest | error
store.LoadRun(runID: string) -> RunState | ErrRunNotFound | ErrCorruptState
store.WriteCancelRequest(runID: string, reason: string) -> error
store.HasCancelRequest(runID: string) -> (bool, string, error)
  - invariant: todas las rutas resueltas dentro de .gandalf/workflows/ vía os.Root; rechazo de symlinks/escapes
  - invariant: WriteCheckpoint calcula checkpoint_digest SHA-256 canónico (run_id, definition_sha256, state_revision, pasos, artefactos, puertas pendientes) y publica vía archivo temporal en el mismo directorio + reemplazo atómico
  - invariant: AcquireExclusive usa bloqueo OS no bloqueante liberado por el kernel si el proceso muere
  - invariant: WriteCancelRequest no requiere RunLock; permite cancelación cooperativa de una corrida running
```

### 3.4 Motor de ejecución y puertos (`internal/workflow/engine`)

```text
MemoryPort.Query(ctx, projectID, callerID, query) -> (MemoryArtifact, error)
RouterPort.Execute(ctx, req: RouterRequest) -> (RouterResult, error)

engine.Validate(ctx, defPath, deps) -> ValidateReport | error
engine.Run(ctx, defPath, deps) -> RunSummary | error
engine.Status(runID, store) -> StatusView | error
engine.Approve(runID, gateID, checkpointDigest, decision, reason, store) -> ApproveResult | error
engine.Resume(ctx, runID, deps) -> RunSummary | error
engine.Cancel(ctx, runID, reason, store) -> CancelResult | error
  - invariant: Run y Resume adquieren RunLock; limitan paralelismo por concurrency_limit (por defecto conservador, ej. 4)
  - invariant: ante human_approval (4A), detiene despacho de pasos nuevos en todas las ramas, espera terminación ordenada de pasos running, persiste checkpoint con checkpoint_digest y pausa en awaiting_approval
  - invariant: Approve verifica estado awaiting_approval, puerta pendiente, definition_sha256 vigente y coincidencia exacta de checkpoint_digest; rechaza reject sin reason; nunca lanza pasos
  - invariant: Resume verifica definition_sha256 intacto y todas las puertas obligatorias decididas; no reejecuta pasos passed; en estado terminal devuelve estado actual con ErrTerminalRun
  - invariant: Cancel sobre running escribe cancel.req y espera acotadamente confirmación de cierre en cancelled; sobre paused/awaiting_approval adquiere RunLock y cierra directamente en cancelled
```

### 3.5 Contrato CLI y mapeo de códigos de salida (`internal/cli`)

| Estado / Error resultante | Código de salida | JSON `status` |
|---|---:|---|
| `succeeded`, `validate` válido, `status`, `approve` válido | `0` | `succeeded` / `validated` / `ok` |
| Error interno inesperado o checkpoint corrupto | `1` | `error` |
| Uso inválido de flags, falta `--checkpoint`, falta `--reason` en `reject` | `2` | `rejected` |
| Error de configuración, definición inválida, proveedor no listo o `run-id` inexistente (`ErrRunNotFound`) | `3` | `invalid` / `config_error` / `not_found` |
| Cancelado mediante `workflow cancel` (`cancelled`) | `10` | `cancelled` |
| Detenido esperando aprobación humana (`awaiting_approval`) | `11` | `awaiting_approval` |
| Interrumpido reanudable (`paused`, `Ctrl+C` durante corrida) | `12` | `paused` |
| Cierre terminal parcial por rama opcional fallida (`partial`) | `13` | `partial` |
| Bloqueado por rechazo humano o `no_secrets` / política dura (`blocked`) | `14` | `blocked` |
| Fallo de paso obligatorio o criterio de objetivo (`failed`) | `15` | `failed` |
| Conflicto (`ErrRunBusy`, `--checkpoint` obsoleto, definición alterada, corrida terminal en `resume` informando su estado terminal en el payload) | `16` | `conflict` |

## 4. Modelo de datos y flujo

### Entidades principales

- **`WorkflowDefinition`:** `schema_version` (`"1"`), `id`, `version`, `concurrency_limit` (opcional, defecto conservador `4`), `objective` (`ObjectiveSpec`), `steps` (`[]StepSpec`).
- **`ObjectiveSpec`:** `id`, `version`, `statement`, `constraints` (`[]string`), `deliverables` (`[]DeliverableRef` mapeando nombre → `<step_id>.<output>`), `success_criteria` (`[]CriterionSpec{id, deliverable, rules}` compartiendo el catálogo cerrado de 6 reglas deterministas de `GateSpec`, siempre bloqueantes y nunca informativas).
- **`StepSpec`:** `id`, `kind` (`memory_query` | `router_task` | `quality_gate` | `human_approval`), `depends_on` (`[]string`), `optional` (`bool`), `inputs` (`map[string]string`), `outputs` (`[]string`), más configuración específica por tipo (`memory`, `router`, `gate` con reglas + `informative`, `correction.max_iterations` + `gate_step_id`, `approval.prompt`).
- **`RunState` (persistido en `.gandalf/workflows/runs/<run-id>/state.json`):** `run_id`, `definition_path`, `definition_sha256`, `state_revision`, `checkpoint_digest`, `status`, `steps` (`map[string]StepRecord`), `gates` (`map[string]GateRecord`), `approvals` (`[]ApprovalRecord`), `warnings` (`[]string`), `objective_verdict`.
- **`StepRecord`:** `status`, `iteration`, `attempts` (historial de intentos con diagnóstico redactado), `artifacts` (`[]ArtifactRef` con `name`, `rel_path`, `sha256`, `size_bytes`, `summary`), `error`.
- **`RunEvent` (append-only en `.gandalf/workflows/runs/<run-id>/events.jsonl`):** `timestamp`, `seq`, `event_type`, `step_id`, `iteration`, `status`, `message` (redactado).

### Flujo primario (`analisis-propuesta@1.0.0`)

1. `gandalf workflow validate analisis-propuesta.json` → `schema.LoadAndValidate` verifica objetivo `OBJ-PROP-001@1.0.0`, grafo, límites (`max_iterations: 2`), reglas de gate y elegibilidad local de ROMERO y Router (`0` ejecuciones) → código `0`.
2. `gandalf workflow run analisis-propuesta.json` → crea `run-id`, adquiere `run.lock`, lanza `contexto_romero` y `borrador_riesgos` en paralelo dentro de `concurrency_limit`, guarda artefactos redactados + checkpoints por transición → une en `sintesis`, intento 1 falla `quality_gate` (`fail`), intento 2 recibe diagnóstico y pasa (`pass`) → alcanza `aprobacion_revision` (`human_approval`), aplica parada conservadora `4A`, emite `checkpoint_digest` y sale con código `11` (`awaiting_approval`).
3. `gandalf workflow status <run-id>` → lee `state.json` en solo lectura, muestra pasos `passed`, 2 intentos de `sintesis`, rutas/resúmenes redactados de artefactos y puerta `aprobacion_revision` con su `--checkpoint <huella>`.
4. `gandalf workflow approve <run-id> --gate aprobacion_revision --checkpoint <huella> --decision approve --reason "Riesgos verificados"` → toma `run.lock`, valida `definition_sha256` y `checkpoint_digest`, registra `ApprovalRecord`, actualiza `state.json` y sale con código `0` sin ejecutar pasos.
5. `gandalf workflow resume <run-id>` → toma `run.lock`, verifica `definition_sha256` intacto y puertas obligatorias decididas, omite `contexto_romero`, `borrador_riesgos` y `sintesis` (`passed`), avanza tras `aprobacion_revision`, evalúa `OBJ-PROP-001@1.0.0` (`gates.EvaluateObjective`), persiste veredicto `succeeded` y sale con código `0`.

- **Fronteras de consistencia:** cada paso completado persiste primero su artefacto redactado en `artifacts/` y después publica atómicamente `state.json` + línea en `events.jsonl`; ante caída entre artefacto y `state.json`, `resume` considera el paso aún pendiente y reescribe de forma limpia sin contaminar pasos anteriores.
- **Impacto de migración:** ninguno; `.gandalf/workflows/` es un espacio nuevo bajo demanda y no migra ni modifica `.gandalf.json` ni `.gandalf-sync.json`.

## 5. Estrategia de pruebas

| Criterio spec | Nivel | Qué afirma | Dobles / dependencias reales |
|---|---|---|---|
| `AC-01` | Unit + Contrato CLI | Valida definición válida con `0` llamadas a proveedores; rechaza ciclo, ref rota, ciclo sin `max_iterations`, `no_secrets` informativo, objetivo incompleto; valida `S6` | Puerto falso contador (`0` llamadas); archivo JSON temporal real |
| `AC-02` | Integración (`engine`) | Dos ramas independientes arrancan antes de terminar la otra (barrera de sincronización determinista) y el paso dependiente solo inicia tras ambas en `passed` | `RouterPort`/`MemoryPort` con canal de barrera (sin `time.Sleep`) + `WorkflowStore` real |
| `AC-03` | Unit + Integración | Cubre las 6 reglas de `quality_gate`, modo informativo (advertencia sin bloqueo), `no_secrets` → `blocked` sin reintentos, y dependientes en `skipped` tras fallo | Contenido sintético y centinela de secreto; `WorkflowStore` real |
| `AC-04` | Integración (`engine`) | Intento 1 falla gate, intento 2 recibe diagnóstico y pasa; agotamiento de `max_iterations` detiene sin bucle infinito en `failed` | `RouterPort` programado por intento + historial de intentos en `state.json` |
| `AC-05` | Contrato CLI + Integración | Parada conservadora `4A` (`0` pasos nuevos al aparecer puerta), `approve` exige `--checkpoint` y `--reason` en `reject`, `resume` bloqueado si falta otra puerta obligatoria, rechazo opcional omite solo su rama | `cli.Run` in-process + puertos deterministas + `WorkflowStore` real |
| `AC-06` | Integración (`store` + `engine`) | Rechazo con código `16` ante definición editada post-inicio, `--checkpoint` obsoleto/ajeno o reutilización de aprobación | Sistema de archivos temporal real con modificación de archivo entre pasos |
| `AC-07` | Integración + Proceso real | Interrupción (`Ctrl+C`/cancelación de contexto) → `paused` (`12`), fallo de paso → `failed` (`15`), `resume` conserva `run-id` sin repetir `passed`; `resume` en terminal devuelve código `16` (`conflict`) con el estado terminal en el diagnóstico/JSON sin reejecutar | Proceso/contexto cancelado determinísticamente + inspección de contadores de llamada |
| `AC-08` | Integración multiproceso | `workflow cancel` sobre corrida `running` activa y sobre `paused`/`awaiting_approval` → `cancelled` (`10`); dos procesos compitiendo por `run`/`resume`/`approve` → uno avanza y el otro recibe código `16` sin corromper `state.json` | Dos goroutines/subprocesos reales contra bloqueo OS (`run.lock`) en directorio temporal real |
| `AC-09` | E2E in-process | Escenario completo `analisis-propuesta@1.0.0` → `succeeded`; rama opcional fallida con objetivo cumplido → `partial` (`13`); criterio de objetivo incumplido → `failed` (`15`) | `cli.Run` completo + puertos dobles controlados + `WorkflowStore` real |
| `AC-10` | Seguridad + Regresión | Centinela de secreto ausente en stdout, stderr, `--json`, `status`, `state.json`, `events.jsonl` y `artifacts/`; árbol de trabajo intacto fuera de `.gandalf/workflows/`; `go test -count=1 ./...` y `TestRepoSecurityGate` en verde | Centinelas sintéticos + escáner real de repositorio + suite completa existente |

- **Dependencias obligatoriamente reales:** sistema de archivos en Windows (`os.Root`, escritura atómica temp+rename y bloqueo OS `LockFileEx`), evaluación real de las 6 reglas y `TestRepoSecurityGate`.
- **Dependencias simuladas en suite determinista:** `MemoryPort` y `RouterPort` mediante dobles deterministas inyectables en `cli.Dependencies` (cero cuota y cero red).

## 6. Secuencia y dependencias

Pensada para dividirse en sub-Issues `T001`–`T00N` tras `tasks` y delegarse por lotes focalizados de 3–4 archivos en Mistral Vibe (`rsc-vibe-delegation`) con Puerta de Cierre Técnico por Issue (`#41`):

1. **Bloque 1 — Esquema y preflight (`internal/workflow/schema`)** — *depende de:* ninguna — `[serial]` (fija IR, tipos, `definition_sha256` y validador `AC-01`/`S6`).
2. **Bloque 2 — Gates deterministas, redactor y objetivo (`internal/workflow/gates`)** — *depende de:* #1 — `[paralelizable con #3]` (implementa 6 reglas `Q1`, redacción y evaluación `Q2`/`AC-09`).
3. **Bloque 3 — Almacén confinado, checkpoints y bloqueo OS (`internal/workflow/store`)** — *depende de:* #1 — `[paralelizable con #2]` (implementa `.gandalf/workflows/`, escritura atómica, `checkpoint_digest`, `run.lock` Windows/POSIX y `cancel.req`).
4. **Bloque 4 — Motor DAG, ciclos, parada `4A` y ciclo de vida (`internal/workflow/engine`)** — *depende de:* #1, #2, #3 — `[serial]` (planificador paralelo acotado, correcciones `max_iterations`, `Run`/`Status`/`Approve`/`Resume`/`Cancel`).
5. **Bloque 5 — CLI, adaptadores reales, render y `.gitignore` (`internal/cli`, `internal/render`)** — *depende de:* #4 — `[serial]` (cableado hacia `romero` y `router`, flags, códigos `11–16`, tabla/`--json` y exclusión Git de `.gandalf/`).
6. **Bloque 6 — Aceptación E2E (`analisis-propuesta@1.0.0`, concurrencia, seguridad y regresión)** — *depende de:* #5 — `[serial]` (cierra `AC-01`–`AC-10`, prueba de humo autorizada opcional y verificación final de rama).

## 7. Riesgos y decisiones abiertas

### Riesgos priorizados

| Riesgo | Disparador | Impacto | Mitigación |
|---|---|---|---|
| Bloqueo `LockFileEx` o reemplazo atómico en Windows | Dos procesos compiten o caída durante `WriteCheckpoint` | Corrupción de `state.json` o falso bloqueo tras caída | Reutilizar patrón de handles/lock OS de `internal/syncfs`, pruebas de contención real con procesos/handles en Windows y lectura segura de `state.json`. |
| Carrera entre `human_approval` (`4A`) y workers en vuelo | Una rama llega a `human_approval` mientras otra termina otro paso a la vez | Despacho indebido de un paso nuevo tras aparecer la puerta | Bucle único coordinador de estado en `engine`: solo el coordinador inspecciona resultados y decide nuevos despachos; bandera `haltedForApproval` bloquea nuevos lanzamientos al instante. |
| Cancelación cooperativa lenta si un ejecutor tarda en abortar | `workflow cancel` sobre `running` con llamada externa bloqueada | Espera excesiva o estado ambiguo | Propagar `context.Context` cancelable al observar `cancel.req` y aplicar ventana de espera acotada devolviendo diagnóstico claro si el ejecutor aún drena. |
| Fuga de secretos vía interpolación de salidas o `events.jsonl` | Paso `router_task` o `memory_query` devuelve o refleja un token | Exposición en artefactos, `status` o logs | Ejecutar `no_secrets` sobre salida cruda antes de guardar y aplicar redacción en las fronteras de `store`, `engine` y `render`. |
| Límite `MAX_PATH` en Windows (260 caracteres) | Nombres largos en `.gandalf/workflows/runs/<run-id>/artifacts/...` | Error de E/S en Windows | Generar `run-id` compactos y validar longitud acotada de nombres de artefactos y rutas en `schema`/`store`. |

### Decisiones abiertas y rollback

- **Decisiones abiertas:** ninguna bloqueante para `tasks` ni `implement`. El streaming/control en tiempo real para `GW-57` (`OPEN-13`) permanece diferido fuera de 0.3; añadir un adaptador YAML de entrada sobre `schema.LoadAndValidate` queda abierto como extensión aditiva futura sin romper contratos.
- **Estrategia de rollback:** la capacidad queda encapsulada en `internal/workflow/*` y el subcomando `gandalf workflow`. Revertir el PR de la épica `#40` desactiva el subcomando sin alterar configuraciones ni contratos de Foundation 0.1, CLI, Router 0.2 ni `sync-skills-patrones`; el estado privado en `.gandalf/workflows/` queda excluido de Git.

## Propuesta de aislamiento

El trabajo ya reside en la rama aislada **`feat/workflow-runtime-03`** (creada desde `master` `6b46728`), separada de `master`. Si trabajas en una sola sesión sobre este directorio, continuar en `feat/workflow-runtime-03` cumple la regla de aislamiento; si vas a operar otra sesión simultánea en la misma carpeta, conviene aislar en `.worktrees/feat-workflow-runtime-03`.

## Tasks

<!-- generated by tasks on 2026-11-20; IDs are stable, do not renumber -->

Los comandos de prueba son compromisos de implementación (TDD estricto según `config.yaml`). Cada ejecución filtrada debe mostrar `PASS` en las pruebas nombradas; una prueba ausente u omitida no cierra la tarea. Las rutas y niveles de riesgo fueron calculados con `rsc risk --json` sobre la política de `origin/master` y se recalcularán sobre el diff real durante `implement`.

| ID | [P] | Tarea | Comprobación de cierre | Depende de | Criterios | Riesgo previsto |
| --- | --- | --- | --- | --- | --- | --- |
| T001 |  | Implementar esquema JSON estricto, IR y preflight `schema.LoadAndValidate` | `go test -count=1 -v ./internal/workflow/schema -run '^TestWorkflowSchema$'` valida definición válida con 0 escrituras y 0 llamadas a proveedores, `S6` condicional, `definition_sha256` canónico y rechazo tipado de ciclos, referencias rotas, `max_iterations < 1`, `no_secrets`/objetivo informativos y campos desconocidos | — | AC-01, AC-03, AC-04, AC-06 | 1 |
| T002 | [P] | Implementar evaluador determinista `gates`, redactor de secretos y veredicto de objetivo | `go test -count=1 -v ./internal/workflow/gates -run '^TestWorkflowGates$'` demuestra las 6 reglas (`non_empty`, `min_length`/`max_length`, `contains`/`not_contains`, `regex_match`/`regex_forbid`, `json_valid`+`json_required_keys`, `no_secrets` → `blocked` no reintentable sobre crudo), modo informativo con advertencia, redacción de patrones + `secretValues` y frontera `succeeded`/`partial`/`failed`/`blocked` | T001 | AC-03, AC-09, AC-10 | 1 |
| T003 | [P] | Implementar almacén confinado `.gandalf/workflows/`, checkpoints atómicos y bloqueo OS `run.lock` | `go test -count=1 -v ./internal/workflow/store -run '^TestWorkflowStore$'` prueba confinamiento `os.Root`, rechazo de escapes/symlinks/`MAX_PATH`, `checkpoint_digest` SHA-256, escritura atómica de `state.json` + `events.jsonl` redactados, `AcquireExclusive` (`ErrRunBusy` entre dos procesos/handles en Windows) y `cancel.req` sin adquirir `RunLock` | T001 | AC-06, AC-07, AC-08, AC-10 | 1 |
| T004 |  | Implementar núcleo del motor DAG paralelo, puertos, gates y ciclos acotados de corrección | `go test -count=1 -v ./internal/workflow/engine -run '^TestWorkflowEngineExecution$'` prueba `Validate` sin cuota, paralelismo real con barrera determinista (sin `time.Sleep`) respetando `concurrency_limit`, `memory_query`, `router_task`, ciclo `max_iterations` (recuperación en intento 2 y parada exacta en el límite), omisión (`skipped`) de dependientes tras fallo y cierre contra objetivo | T001, T002, T003 | AC-01, AC-02, AC-03, AC-04, AC-09, AC-10 | 1 |
| T005 |  | Implementar parada conservadora `4A`, `Status`, `Approve`, `Resume`, interrupción y `Cancel` cooperativo | `go test -count=1 -v ./internal/workflow/engine -run '^TestWorkflowEngineLifecycle$'` prueba parada `4A` (0 pasos nuevos al llegar a `human_approval`), rechazo de `--checkpoint` obsoleto/ajeno o `reject` sin motivo, bloqueo de `resume` con puerta obligatoria pendiente, rechazo de definición alterada y corridas terminales, `Ctrl+C`→`paused`, reanudación sin repetir `passed` y `Cancel` en `running`/`paused`/`awaiting_approval`→`cancelled` | T004 | AC-05, AC-06, AC-07, AC-08, AC-09 | 1 |
| T006 |  | Implementar presentación tabla/`--json` de workflow, `.gitignore` de `.gandalf/` y guardia de seguridad | `go test -count=1 -v ./internal/render -run '^TestWorkflowRender$'` y `go test -count=1 -v ./internal/cli -run '^TestRepoSecurityGate$'` prueban `schema_version: "1"`, redacción de secretos en tabla/JSON, exclusión `.gandalf/` en `.gitignore` y detección de `.gandalf/` en `scanPathAndContent` | T005 | AC-01, AC-05, AC-07, AC-09, AC-10 | 1 |
| T007 |  | Integrar subcomandos `gandalf workflow`, adaptadores ROMERO/Router y códigos de salida `11–16` | `go test -count=1 -v ./internal/cli -run '^TestWorkflowCLI$'` cubre `validate`, `run`, `status`, `approve`, `resume` y `cancel`, flags y errores de uso (`2`), configuración/preflight (`3`), mapeo de `10` y `11–16` preservando `0–10`, puertos inyectables en `cli.Dependencies` y adaptadores de producción hacia `romero` y `router` respetando `deps.getenv` | T006 | AC-01, AC-05, AC-06, AC-07, AC-08, AC-09, AC-10 | 1 |
| T008 |  | Implementar aceptación E2E `analisis-propuesta@1.0.0`, concurrencia multiproceso y barrido de secretos | `go test -count=1 -v ./internal/cli -run '^TestWorkflowAcceptance$'` cubre el escenario completo `analisis-propuesta@1.0.0` (`OBJ-PROP-001@1.0.0`), `AC-01`–`AC-10`, interrupción de proceso real (`paused` `12`) + `resume`, contención multiproceso (`16`), cancelación cooperativa (`10`), centinela de secreto ausente en stdout/stderr/JSON/disco y árbol del proyecto intacto | T007 | AC-01 a AC-10 | 1 |
| T009 |  | Ejecutar verificación integral de cierre, regresión completa y evidencia de rollback | Ejecutar todas las comprobaciones `T001`–`T008`, `gofmt -l internal/workflow internal/cli/workflow*.go internal/render/workflow*.go` (salida vacía), `go vet ./...`, `go build ./...` y `go test -count=1 ./...` (incluyendo Foundation/CLI/Router/Sync y `TestRepoSecurityGate`) en verde, y registrar veredicto + rollback en `02-DOCS/wiki/sdd/verifications/workflow-runtime-03-closing.md` | T008 | AC-01 a AC-10 | 0 |

### Interfaces por tarea

Las restricciones globales de §0 aplican implícitamente a todas las tareas. Nombres y firmas concretas para ejecución aislada (incluidos lotes focalizados de 3–4 archivos en Mistral Vibe):

- **T001 (`internal/workflow/schema`):** Produce `WorkflowDefinition`, `ObjectiveSpec`, `DeliverableRef`, `CriterionSpec{ID string, Deliverable string, Rules []GateRuleSpec}`, `StepSpec`, `GateSpec`, `GateRuleSpec`, `ValidateOptions{CheckProviders bool, MemoryReady bool, RouterReady bool}`, `ValidatedWorkflow{Definition WorkflowDefinition, DefinitionPath string, DefinitionSHA256 string, ExecutionOrder []string}`, `ValidationError{Code string, Field string, Message string}` y `LoadAndValidate(defPath string, opts ValidateOptions) (ValidatedWorkflow, error)`. Errores tipados verificables con `errors.As` (`*ValidationErrors`). La única relación de corrección permitida es `router_task.correction.gate_step_id` apuntando a un paso `quality_gate` bloqueante que declara a ese `router_task` en `depends_on` (con `max_iterations >= 1`); cualquier ciclo en `depends_on` se rechaza como ciclo no declarado. Cero escritura en disco y cero invocaciones externas.
- **T002 (`internal/workflow/gates`):** Consume `schema.GateSpec` y `schema.ObjectiveSpec`. Produce `Redact(input string, secretValues []string) string`, `HasSecrets(input string, secretValues []string) bool`, `GateResult{Status string /* pass|fail|blocked */, Retryable bool, Informative bool, Violations []string, Warnings []string}`, `EvaluateGate(spec schema.GateSpec, rawOutput string, secretValues []string) GateResult`, `StepOutcome{ID string, Optional bool, Status string, GateStatus string, Warnings []string}`, `ObjectiveVerdict{Status string /* succeeded|partial|failed|blocked */, MetCriteria []string, FailedCriteria []string, Warnings []string}` y `EvaluateObjective(obj schema.ObjectiveSpec, steps map[string]StepOutcome, artifacts map[string]string) ObjectiveVerdict`.
- **T003 (`internal/workflow/store`):** Consume `schema.ValidatedWorkflow`. Produce errores centinela `ErrUnsafePath`, `ErrRunBusy`, `ErrRunNotFound`, `ErrCorruptState`, tipos `RunState`, `StepRecord`, `AttemptRecord`, `ArtifactRef{Name, RelPath, SHA256, SizeBytes, Summary}`, `GateRecord`, `ApprovalRecord`, `RunEvent` y `WorkflowStore` con `OpenRoot(projectRoot string) (*WorkflowStore, error)`, `CreateRun(def schema.ValidatedWorkflow) (string, *RunState, error)`, `AcquireExclusive(runID string) (*RunLock, error)`, `SaveArtifact(runID, stepID string, iter int, name, sanitizedContent string) (ArtifactRef, error)`, `WriteCheckpoint(runID string, state *RunState) (string, error)`, `AppendEvent(runID string, ev RunEvent) error`, `LoadRun(runID string) (*RunState, error)`, `WriteCancelRequest(runID, reason string) error`, `HasCancelRequest(runID string) (bool, string, error)`.
- **T004 (`internal/workflow/engine`):** Consume `schema`, `gates` y `store`. Define puertos del lado consumidor `MemoryPort{Query(ctx, projectID, callerID, query) (MemoryResult, error)}` y `RouterPort{Execute(ctx, req RouterRequest) (RouterResult, error)}`, `Dependencies{Store *store.WorkflowStore, Memory MemoryPort, Router RouterPort, ProjectID, CallerID string, SecretValues []string, MemoryReady, RouterReady bool}`. Produce `Validate(ctx, defPath string, deps Dependencies) (ValidateReport, error)` y `Run(ctx, defPath string, deps Dependencies) (RunSummary, error)` con bucle coordinador único, paralelismo acotado por `concurrency_limit` (defecto `4`), interpolación redactada, `quality_gate` y ciclos `max_iterations`.
- **T005 (`internal/workflow/engine`):** Extiende `engine` consumiendo `store.AcquireExclusive`, `WriteCheckpoint`, `WriteCancelRequest`/`HasCancelRequest` y `schema.LoadAndValidate` (para verificar `definition_sha256` vigente). Produce errores centinela `ErrConflict`, `ErrTerminalRun`, `ErrInvalidApproval`, `ErrPendingApproval` y operaciones `Status(runID string, st *store.WorkflowStore) (StatusView, error)`, `Approve(runID, gateID, checkpointDigest, decision, reason string, st *store.WorkflowStore) (ApproveResult, error)`, `Resume(ctx context.Context, runID string, deps Dependencies) (RunSummary, error)` y `Cancel(ctx context.Context, runID, reason string, st *store.WorkflowStore) (CancelResult, error)`, más bandera `haltedForApproval` en el coordinador para parada conservadora `4A`.
- **T006 (`internal/render`, `.gitignore`, `internal/cli/repo_security_test.go`):** Consume `engine.ValidateReport`, `engine.RunSummary`, `engine.StatusView`, `engine.ApproveResult`, `engine.CancelResult` y `render.SchemaVersion` (`"1"`). Produce `RenderWorkflowValidate`, `RenderWorkflowRun`, `RenderWorkflowStatus`, `RenderWorkflowApprove`, `RenderWorkflowCancel` en tabla redactada y `--json`; añade `.gandalf/` a `.gitignore` y a la lista de prefijos prohibidos en `scanPathAndContent` de `internal/cli/repo_security_test.go`.
- **T007 (`internal/cli`):** Consume `engine`, `render`, `config.Load`, `romero.NewMemoryQueryOrchestrator` y `router.Select`/`router.Run`. Añade en `exitcodes.go` constantes `ExitAwaitingApproval = 11`, `ExitPaused = 12`, `ExitPartial = 13`, `ExitBlocked = 14`, `ExitFailed = 15`, `ExitConflict = 16`, y campos inyectables opcionales `WorkflowMemory engine.MemoryPort` / `WorkflowRouter engine.RouterPort` en `cli.Dependencies`. Despacha `workflow <validate|run|status|approve|resume|cancel>` en `internal/cli/run.go` y `internal/cli/workflow.go` respetando `deps.getenv` y `S6`.
- **T008 (`internal/cli`):** Consume `cli.Run` y `cli.Dependencies` con puertos deterministas inyectables y un subproceso real compilado/invocado en test (`os/exec` + `CREATE_NEW_PROCESS_GROUP`/`CTRL_BREAK_EVENT` en Windows como `router-02`, o patrón equivalente multiplataforma). Produce cobertura trazable `AC-01`–`AC-10` en `workflow_acceptance_test.go` y `workflow_process_test.go`.
- **T009 (`02-DOCS/wiki/sdd/`):** Consume las ocho comprobaciones `T001`–`T008` y los controles Go del proyecto. Produce `verifications/workflow-runtime-03-closing.md` y cierra `progress/workflow-runtime-03.md`.

## Previsión de revisión y entrega

| Dimensión | Previsión | Base |
| --- | --- | --- |
| Líneas modificadas | 1.800–2.800 | Cuatro paquetes nuevos (`schema`, `gates`, `store`, `engine`), CLI/render y aceptación E2E + multiproceso |
| Archivos | 24–30 | Lotes de 2–5 archivos por tarea (dimensionados para delegación en Mistral Vibe de 6 turnos) |
| Riesgo | 8 tareas `tier 1`, 1 documental `tier 0` | Clasificación calculada con `rsc risk --json` contra `origin/master` para las nueve tareas; `implement` recalculará sobre el diff real |
| Entrega | `ask-on-risk` — **PRs separados por corte** | Supera el presupuesto base (`400` líneas / `12` archivos en `config.yaml`). Confirmado por el propietario: **PRs separados por corte**: Corte 1 (`T001`–`T003`, `#43`–`#45`), Corte 2 (`T004`–`T005`, `#46`–`#47`) y Corte 3 (`T006`–`T009`, `#48`–`#51` + cierre de `#40`) |

`T002` (`internal/workflow/gates`) y `T003` (`internal/workflow/store`) comparten cero archivos y cero estado; pueden ejecutarse en paralelo en cuanto concluya `T001`.

## Subissues en GitHub

| Tarea | Subissue |
| --- | --- |
| `T001` | [#43](https://github.com/CodeMonkReaper/gandalf-wizard/issues/43) |
| `T002` `[P]` | [#44](https://github.com/CodeMonkReaper/gandalf-wizard/issues/44) |
| `T003` `[P]` | [#45](https://github.com/CodeMonkReaper/gandalf-wizard/issues/45) |
| `T004` | [#46](https://github.com/CodeMonkReaper/gandalf-wizard/issues/46) |
| `T005` | [#47](https://github.com/CodeMonkReaper/gandalf-wizard/issues/47) |
| `T006` | [#48](https://github.com/CodeMonkReaper/gandalf-wizard/issues/48) |
| `T007` | [#49](https://github.com/CodeMonkReaper/gandalf-wizard/issues/49) |
| `T008` | [#50](https://github.com/CodeMonkReaper/gandalf-wizard/issues/50) |
| `T009` | [#51](https://github.com/CodeMonkReaper/gandalf-wizard/issues/51) |
