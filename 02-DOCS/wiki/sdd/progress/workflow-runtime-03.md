---
type: progress
title: Avance — Workflow Runtime 0.3
description: Registro append-only de implementación TDD, delegación Vibe, riesgos, revisiones y cola de decisiones de workflow-runtime-03.
tags: [sdd, progress, workflow-runtime, odd, gates]
timestamp: 2026-11-20T18:26:00Z
topic: sdd
slug: workflow-runtime-03
---

# Avance — Workflow Runtime 0.3

[Épica #40](https://github.com/CodeMonkReaper/gandalf-wizard/issues/40) · [Constitución v1.2.0](../constitution.md) · [Especificación aprobada](../specs/workflow-runtime-03.md) · [Plan y tareas](../plans/workflow-runtime-03.md) · [Análisis de consistencia](../analysis/workflow-runtime-03.md)

## Inicio de Corte 1 (T001–T003)

- Rama de trabajo: `feat/workflow-runtime-03`.
- Estrategia confirmada por el propietario: **PRs separados por corte** (Corte 1: `T001`–`T003` [`#43`–`#45`], Corte 2: `T004`–`T005` [`#46`–`#47`], Corte 3: `T006`–`T009` [`#48`–`#51` + cierre de `#40`]).
- Modelo ejecutor en Vibe seleccionado por el propietario: `zai-glm-5-3` en `T001`; cambiado por autorización del propietario a `zai-glm-5-3-flash` desde `T002`–`T003` (`T009` en coordinador directo).
- Base pre-código verificada: los 15 paquetes Go actuales pasan `go test -count=1 ./...` antes de crear `internal/workflow/`.

## T001 · #43 · 2026-11-20

- **Tarea / Issue**: `T001` / [#43](https://github.com/CodeMonkReaper/gandalf-wizard/issues/43) — implementar esquema JSON estricto, IR y preflight `schema.LoadAndValidate` en `internal/workflow/schema`.
- **Estado**: `complete`.
- **Estado previo (RED)**: el coordinador creó [types.go](../../../../internal/workflow/schema/types.go), el stub [validate.go](../../../../internal/workflow/schema/validate.go) y [validate_test.go](../../../../internal/workflow/schema/validate_test.go); `go test -count=1 -v ./internal/workflow/schema -run '^TestWorkflowSchema$'` falló como se esperaba con `not_implemented (workflow): not implemented`.
- **Delegación a Vibe (intento 1)**:
  - **Fecha**: 2026-11-20
  - **Task ID / Modelo**: `workflow-t001-schema` · solicitado/efectivo `zai-glm-5-3`
  - **Run ID / Estado**: `6b7cd0fd-f7a0-4c17-9316-af738030e5b9` · `DONE_WITH_CONCERNS` (`exitCode: 3`, `stopReason: limit`, 94.1 s)
  - **Trabajo realizado por Vibe**: 0 archivos modificados (agotó los 6 turnos explorando `internal/router`).
  - **Verificaciones ejecutadas por el coordinador**: `git status --short` (sin cambios en `validate.go`) y `go test -count=1 -v ./internal/workflow/schema -run '^TestWorkflowSchema$'` (continuó en RED por `not_implemented`).
  - **Limitaciones y siguiente paso**: reintento acotado `workflow-t001-schema-r2` entregando explícitamente los valores válidos de complejidad/riesgo y restringiendo la lectura únicamente a `internal/workflow/schema/`.
- **Delegación a Vibe (intento 2)**:
  - **Fecha**: 2026-11-20
  - **Task ID / Modelo**: `workflow-t001-schema-r2` · solicitado/efectivo `zai-glm-5-3`
  - **Run ID / Estado**: `9f84c5b4-1a8e-4338-a1de-f91415094eb2` · `BLOCKED` (`exitCode: 2`, `stdout.log` vacío, 529.7 s)
  - **Trabajo realizado por Vibe**: 0 archivos modificados (`stderr.log` reportó `Settings for model 'glm-5-3' are ignored` y `Error: Model stream ended without a finish reason.`).
  - **Verificaciones ejecutadas por el coordinador**: inspección de `validate.go` (sin cambios) y confirmación de que `TestWorkflowSchema` permanecía en RED.
  - **Limitaciones y siguiente paso**: alcanzado el tope de 2 intentos por tarea de `rsc-vibe-delegation.md`, el coordinador asumió la implementación directa de `T001` y solicita al propietario confirmar el modelo ejecutor de Vibe antes de `T002`/`T003`.
- **Implementación y verificación GREEN / triangulación (coordinador)**:
  - `red`: `not_implemented` en 17 casos iniciales + triangulación posterior de `L23` (`correccion_apuntando_a_gate_que_evalua_otro_paso_paralelo` y `correccion_apuntando_a_gate_que_evalua_ancestro_transitivo` fallaron con `got nil` antes de corregir `validate.go`).
  - `green`: 28/28 subtests de `TestWorkflowSchema` en `PASS` (solo lectura verificada en `t.TempDir()`, 0 llamadas externas, `S6`, estricta decodificación JSON + EOF, Kahn determinista, catálogo cerrado de gates y ciclos `correction`).
  - **Commits / archivos**: `a0c20a4` + `66540d7` (`internal/workflow/schema/types.go`, `internal/workflow/schema/validate.go`, `internal/workflow/schema/validate_test.go`).
- **Riesgo y revisión (`rsc risk` + ledger)**:
  - **Riesgo real**: `rsc risk --base 9c64042 --json` → `tier 1` (`classes: ["default"]`, 1 revisor, `humanApproval: false`, `policySource: origin/master`).
  - **Revisión ronda 1 (`a0c20a4`)**: revisor en contexto fresco (`c31f5421`) → `1 reported → 1 unique → 1 confirmed` (`L23`, `should-fix`, `missing-invariant-check` en `validate.go:507`: `correction.gate_step_id` no verificaba que `gate.target` perteneciera al `router_task` vinculado; verificado `CONFIRMED` por `finding-verifier` fresco `7b785890`).
  - **Revisión ronda 2 (`a0c20a4..66540d7`)**: corregido en `validate.go:510-512` con 2 subtests de regresión (`66540d7`); revisor fresco (`e652e416`) sobre el diff incremental → `0` hallazgos, `VERDICT: pass`; `L23` cerrado como `fixed` en el ledger.
- **Controles ejecutados**:
  - `tests`: `go test -count=1 -v ./internal/workflow/schema -run '^TestWorkflowSchema$'` (`PASS`, 28/28 subtests) y `go test -count=1 ./...` (`16/16` paquetes en verde).
  - `build`: `gofmt -l internal/workflow/schema` (salida vacía), `go vet ./...` (`ok`) y `go build ./...` (`ok`).
  - `security`: `TestRepoSecurityGate` en verde dentro de la suite completa; 0 escrituras de disco y 0 fugas de valores/patrones en errores.
  - `traceability`: cubre `AC-01`, `AC-03`, `AC-04`, `AC-06` (y base de `S6`).
- **Skill resolution**: `used: [implement, go, review]`, `missing: []`, `fallback: []`.

## T002 · #44 · 2026-11-20

- **Tarea / Issue**: `T002` / [#44](https://github.com/CodeMonkReaper/gandalf-wizard/issues/44) — implementar evaluador determinista `gates`, redactor de secretos y veredicto de objetivo en `internal/workflow/gates`.
- **Estado**: `complete`.
- **Estado previo (RED)**: el coordinador creó [redact.go](../../../../internal/workflow/gates/redact.go), [gates.go](../../../../internal/workflow/gates/gates.go), [objective.go](../../../../internal/workflow/gates/objective.go) (stubs) y [gates_test.go](../../../../internal/workflow/gates/gates_test.go); `go test -count=1 -v ./internal/workflow/gates -run '^TestWorkflowGates$'` falló como se esperaba en detección/redacción, las 6 familias de reglas, `no_secrets` y evaluación de objetivo (`Q2`).
- **Delegación a Vibe (intento 1)**:
  - **Fecha**: 2026-11-20
  - **Task ID / Modelo**: `workflow-t002-gates` · solicitado/efectivo `zai-glm-5-3-flash`
  - **Run ID / Estado**: `58c853d3-af6f-47fe-8c66-97aeef73eac4` · `DONE_WITH_CONCERNS` (`exitCode: 3`, `stopReason: limit`, 62.4 s)
  - **Trabajo realizado por Vibe**: implementó `redact.go` y `gates.go` (sin alcanzar `objective.go` en el tope de 6 turnos).
  - **Verificaciones ejecutadas por el coordinador**: `git status --short internal/workflow/gates` y `go test -count=1 -v ./internal/workflow/gates -run '^TestWorkflowGates$'` (28/28 subtests de redacción/reglas/`no_secrets` en verde; faltó `objective.go`).
  - **Limitaciones y siguiente paso**: reintento acotado `workflow-t002-gates-r2` focalizado únicamente en `objective.go` y warnings cuando `spec.Rules` está vacío.
- **Delegación a Vibe (intento 2)**:
  - **Fecha**: 2026-11-20
  - **Task ID / Modelo**: `workflow-t002-gates-r2` · solicitado/efectivo `zai-glm-5-3-flash`
  - **Run ID / Estado**: `f584caf0-1a68-4f18-92fe-4537b078afde` · `DONE_WITH_CONCERNS` (`exitCode: 3`, `stopReason: limit`, 31.8 s)
  - **Trabajo realizado por Vibe**: implementó `internal/workflow/gates/objective.go` completo.
  - **Verificaciones ejecutadas por el coordinador**: `go test -count=1 -v ./internal/workflow/gates -run '^TestWorkflowGates$'` (34/34 subtests en `PASS`); el coordinador completó `Warnings` para `len(spec.Rules) == 0 && spec.Informative` en `gates.go` antes de commitear.
  - **Limitaciones y siguiente paso**: abierto el ciclo de revisión independiente para validar casos borde de solapamiento e aserciones.
- **Implementación, verificación GREEN / triangulación y revisión (`rsc risk` + ledger)**:
  - **Riesgo real**: `rsc risk --base a2101fe --json` → `tier 1` (`classes: ["default"]`, 1 revisor, `humanApproval: false`, `policySource: origin/master`).
  - **Commit inicial**: `121095f`.
  - **Revisión ronda 1 (`121095f`)**: revisor fresco (`9294dab0`) → `3 reported → 3 unique → 0 toVerify` (`L24` nit en `dynamic_longest_first`, `L25` nit en precedencia de redacción híbrida `sk-proj-...+tail`, `L26` nit en subtest acoplado de bloqueo; `VERDICT: APPROVE WITH NITS`). El coordinador reforzó `gates_test.go` en RED (`hybrid_dynamic_with_pattern_prefix` falló con `"key=[redactado]+tail"`) y corrigió en `redact.go` (`3869018`).
  - **Revisión ronda 2 (`121095f..3869018`)**: revisor fresco (`819b27e1`) → `1 reported → 1 unique → 1 confirmed` (`L27`, `should-fix`, `sequential-replacement-overlap-and-rescan` en `redact.go:56`; verificado `CONFIRMED` por `finding-verifier` fresco `702423bc`).
  - **Revisión ronda 3 (`3869018..b506223`)**: corregido en RED→GREEN añadiendo 3 subtests de regresión en `gates_test.go` (patrón fijo con subcadena explícita, secretos solapados y subcadenas de `[redactado]` + idempotencia) e implementando fusión de intervalos en una sola pasada en `redact.go` (`b506223`). Revisor fresco (`162f76a3`) → `0` hallazgos, `VERDICT: APPROVE`; `L24`–`L27` cerrados como `fixed` en el ledger.
- **Controles ejecutados**:
  - `tests`: `go test -count=1 -v ./internal/workflow/gates -run '^TestWorkflowGates$'` (`PASS`, 41/41 subtests) y dentro de la suite completa `go test -count=1 ./...` (`18/18` paquetes en verde).
  - `build`: `gofmt -l internal/workflow/gates` (salida vacía), `go vet ./...` (`ok`) y `go build ./...` (`ok`).
  - `security`: `TestRepoSecurityGate` en verde sin literales contiguos de secretos ni fugas en errores/warnings.
  - `traceability`: cubre `AC-03`, `AC-09`, `AC-10`.
- **Skill resolution**: `used: [implement, go, review]`, `missing: []`, `fallback: []`.

## T003 · #45 · 2026-11-20

- **Tarea / Issue**: `T003` / [#45](https://github.com/CodeMonkReaper/gandalf-wizard/issues/45) — implementar almacén confinado `.gandalf/workflows/`, checkpoints atómicos y bloqueo OS `run.lock` en `internal/workflow/store`.
- **Estado**: `complete`.
- **Estado previo (RED)**: el coordinador creó [store.go](../../../../internal/workflow/store/store.go), [checkpoint.go](../../../../internal/workflow/store/checkpoint.go), [lock_windows.go](../../../../internal/workflow/store/lock_windows.go), [lock_other.go](../../../../internal/workflow/store/lock_other.go) (stubs) y [store_test.go](../../../../internal/workflow/store/store_test.go); `go test -count=1 -v ./internal/workflow/store -run '^TestWorkflowStore$'` falló como se esperaba con `not implemented`.
- **Delegación a Vibe (intento 1)**:
  - **Fecha**: 2026-11-20
  - **Task ID / Modelo**: `workflow-t003-store` · solicitado `zai-glm-5-3-flash`, efectivo `no disponible`
  - **Run ID / Estado**: `6d04253b-6782-47d4-8ace-2b542bb7d571` · `BLOCKED` (`Owner and child exited without final record` tras reinicio de entorno)
  - **Trabajo realizado por Vibe**: 0 archivos modificados.
  - **Verificaciones ejecutadas por el coordinador**: `Get-ChildItem internal/workflow/store` (marcas intactas) y confirmación de que `TestWorkflowStore` continuaba en RED.
  - **Limitaciones y siguiente paso**: reintento acotado `workflow-t003-store-r2` con contrato completo en línea.
- **Delegación a Vibe (intento 2)**:
  - **Fecha**: 2026-11-20
  - **Task ID / Modelo**: `workflow-t003-store-r2` · solicitado `zai-glm-5-3-flash`, efectivo `no disponible`
  - **Run ID / Estado**: `9966d9cd-e149-47ed-b48f-c7802f9e3ea9` · `BLOCKED` (proceso bloqueado >14 min con 0 CPU tras escribir 3 de 4 archivos; tarea cancelada y proceso huérfano terminado)
  - **Trabajo realizado por Vibe**: escribió `checkpoint.go`, `lock_windows.go` y `lock_other.go`; no alcanzó a escribir `store.go`.
  - **Verificaciones ejecutadas por el coordinador**: inspección de `store.go` (sin cambios desde el stub); alcanzado el tope de 2 intentos por tarea, el coordinador asumió el cierre directo completando `store.go`, aplicando fusión de intervalos + idempotencia `[redactado]` en `checkpoint.go` y `runtime.KeepAlive` en `lock_windows.go`/`lock_other.go`.
  - **Limitaciones y siguiente paso**: avanzar a commit aislado y revisión independiente tras verificar Windows y compilado Linux.
- **Implementación, verificación GREEN / triangulación y revisión (`rsc risk` + ledger)**:
  - **Commit inicial**: `8c4b4ee`.
  - **Riesgo real**: `rsc risk --base b506223 --json` → `tier 1` (`classes: ["default"]`, 1 revisor, `humanApproval: false`, `policySource: origin/master`).
  - **Revisión ronda 1 (`8c4b4ee`)**: revisor fresco (`68d8014f`) → `3 reported → 3 unique → 1 confirmed` (`L28`, `should-fix`, `concurrent-race` en `store.go:239` `ensureSafeDir` sin manejar `os.ErrExist` ante concurrencia, verificado `CONFIRMED` por `finding-verifier` fresco `7cfe7e4e`; más `L29` nit en `!info.Mode().IsRegular()` y `L30` nit en tope de `MAX_PATH` en test).
  - **Revisión ronda 2 (`8c4b4ee..e3f3a55`)**: corregido en RED→GREEN añadiendo `apertura_concurrente_openroot` (falló en ronda 0 con `mkdirat .gandalf: file exists` antes del fix), re-stat ante `os.ErrExist`, `!info.Mode().IsRegular()` en todos los guardias de archivo y tope exacto `<= 185` en el test `MAX_PATH` (`e3f3a55`). Revisor fresco (`eed6025c`) sobre el diff incremental → `0` hallazgos, `VERDICT: APPROVE`; `L28`–`L30` cerrados como `fixed` en el ledger.
- **Controles ejecutados**:
  - `tests`: `go test -count=1 -v ./internal/workflow/store -run '^TestWorkflowStore$'` (`PASS`, incluidos confinamiento `os.Root`, rechazo de junctions/escapes/`MAX_PATH`, `apertura_concurrente_openroot`, checkpoints SHA-256 atómicos, corrupción `ErrCorruptState`, contención real entre procesos con `LockFileEx`, liberación por kernel y `cancel.req` sin `RunLock`) y `go test -count=1 ./...` (`18/18` paquetes en verde).
  - `build`: `gofmt -l internal/workflow` (salida vacía), `go vet ./...` (`ok`), `go build ./...` (`ok`) y `GOOS=linux GOARCH=amd64 go build ./...` (`ok`).
  - `security`: `TestRepoSecurityGate` en verde tras la última edición; cero secretos en historial ni artefactos rastreados.
  - `traceability`: cubre `AC-06`, `AC-07`, `AC-08`, `AC-10`.
- **Skill resolution**: `used: [implement, go, review]`, `missing: []`, `fallback: []`.

## Fin de Corte 1 (T001–T003)

- Sub-Issues cerrados técnicamente en `status:in-review`: [#43](https://github.com/CodeMonkReaper/gandalf-wizard/issues/43) (`T001`), [#44](https://github.com/CodeMonkReaper/gandalf-wizard/issues/44) (`T002`) y [#45](https://github.com/CodeMonkReaper/gandalf-wizard/issues/45) (`T003`).
- Suite completa post-código: `go test -count=1 ./...` (`18/18` paquetes `ok`). Pendiente de autorización humana para `git push` y apertura del **PR #1 (Corte 1)**.

## T004 · #46 · 2026-11-20

- **Tarea / Issue**: `T004` / [#46](https://github.com/CodeMonkReaper/gandalf-wizard/issues/46) — motor DAG paralelo, puertos, `quality_gate` y ciclos acotados en `internal/workflow/engine`. Entregado en [engine.go](../../../../internal/workflow/engine/engine.go), [execution.go](../../../../internal/workflow/engine/execution.go), [lifecycle.go](../../../../internal/workflow/engine/lifecycle.go) y [engine_test.go](../../../../internal/workflow/engine/engine_test.go); el Issue nombraba `ports.go`/`scheduler.go`/`steps.go`/`scheduler_test.go`, que se fusionaron en esos cuatro archivos.
- **Estado**: `complete`; informe de cierre [publicado en `#46`](https://github.com/CodeMonkReaper/gandalf-wizard/issues/46#issuecomment-7054876916) y etiqueta `status:in-review`. El Issue se cierra con el PR del Corte 2.
- **Commits**: `241d2ec` (motor) + `6ae0441` (correcciones de la ronda de revisión).
- **Delegación a Vibe**: no registrada en el ledger; la tarea se ejecutó en el coordinador.
- **Riesgo y revisión (`rsc risk` + ledger)**:
  - `rsc risk --base 3bb0f29 --json` → `tier 1`, 1 revisor; el Issue lleva `tier:1`.
  - Ronda 1 (`2026-11-20T20:44Z`) sobre `241d2ec`: 6 hallazgos `L31`–`L36`, cuatro bloqueantes (bypass de `no_secrets` durante la corrección, diagnósticos de gate sin redactar, colisión de nombres de artefacto, artefacto declarado de gate ausente) y dos `should-fix` (criterio bloqueado mostrado como cumplido, medición de pico que podía perder despachos).
  - Ronda 2 (`20:50Z`) sobre `6ae0441`: los seis corregidos con su caso de regresión.
  - Verificación en HEAD (`2026-11-21`, auditoría `T4`): los seis siguen corregidos y probados. El ledger no tenía `outcome` para ellos; se cerraron como `fixed` en esa auditoría.
- **Done-check**: `go test -count=1 -v ./internal/workflow/engine -run '^TestWorkflowEngineExecution$'` en verde (7/7 subtests al cierre; 14/14 tras la ronda `T4`).
- **Controles ejecutados**: `gofmt -l internal/workflow` (vacío), `go vet ./...` (`ok`), `go build ./...` (`ok`), `GOOS=linux GOARCH=amd64 go build ./...` (`ok`) y `go test -count=1 ./...` (`19/19` paquetes en verde).
- **Traceability**: `AC-01`, `AC-02`, `AC-03`, `AC-04`, `AC-09`, `AC-10`.

## T005 · #47 · 2026-11-20

- **Tarea / Issue**: `T005` / [#47](https://github.com/CodeMonkReaper/gandalf-wizard/issues/47) — parada conservadora `4A`, `Status`, `Approve`, `Resume` y `Cancel` en `internal/workflow/engine` ([lifecycle.go](../../../../internal/workflow/engine/lifecycle.go) y [lifecycle_test.go](../../../../internal/workflow/engine/lifecycle_test.go)).
- **Estado**: `complete`; informe de cierre [publicado en `#47`](https://github.com/CodeMonkReaper/gandalf-wizard/issues/47#issuecomment-7055061696) y etiqueta `status:in-review`. El Issue se cierra con el PR del Corte 2.
- **Commits**: `e25b5bf` (ciclo de vida) + `934f569` (correcciones `T5-1`–`T5-4`) + `f58e8db` (ronda `V5`).
- **Delegación a Vibe**: no registrada; ejecución en el coordinador.
- **Riesgo y revisión (`rsc risk` + ledger)**:
  - `tier 1` en el Issue; la ronda `V5` recalculó `rsc risk --base HEAD --json` → `tier 1`, 1 revisor.
  - Ronda 1 en el ledger: sin hallazgos. Los cuatro `T5-1`–`T5-4` provienen de la doble comprobación del coordinador recogida más abajo y **no se registraron en el ledger en su momento**; ese hueco se declara aquí en lugar de reescribir el historial.
  - Ronda 2 (auditoría independiente `V5`, 2026-11-21): `L43`–`L46` corregidos en `f58e8db`; `V5-5` (proceso) cerrado con este registro y el informe del Issue.
- **Done-check**: `go test -count=1 -v ./internal/workflow/engine -run '^TestWorkflowEngineLifecycle$'` en verde (13/13 subtests al cierre; 17/17 tras la ronda `V5`).
- **Controles ejecutados**: `gofmt -l internal/workflow` (vacío), `go vet ./...` (`ok`), `go build ./...` (`ok`), `GOOS=linux GOARCH=amd64 go build ./...` (`ok`) y `go test -count=1 ./...` (`19/19` paquetes).
- **Traceability**: `AC-05`, `AC-06`, `AC-07`, `AC-08` y `AC-09` (frontera `blocked`/`partial` en `Approve`).

## Corte 2 y Corte 3 — registro reconstruido (T006)

- **Nota de trazabilidad**: este archivo no recibió entrada para `T006` en su momento. Lo que sigue se reconstruye desde `git log` y la evidencia de pruebas; su ronda de revisión consta en el ledger sin hallazgos registrados.
- **T006 · #48 · `728c196`** — `render` de workflow, `.gandalf/` en `.gitignore` y guardia de repositorio. Done-check `go test -count=1 -v ./internal/render -run '^TestWorkflowRender$'` y `go test -count=1 -v ./internal/cli -run '^TestRepoSecurityGate$'` en verde.

## Ronda de corrección T005/T006 · 2026-11-20

Auditoría de doble comprobación sobre `T005` y `T006` contra su done-check y los AC. Seis hallazgos: cuatro corregidos, dos resueltos como decisión.

- **T5-1 · `should-fix` · corregido** — `Resume` no recuperaba un paso cuyo ciclo de corrección agotó `max_iterations`, contradiciendo la tabla de transiciones. `red`: `manual_resume_starts_a_fresh_bounded_correction_cycle` fallaba con `Status:failed`. `green`: `StepRecord.CorrectionCycleStart` separa el presupuesto por ciclo. Triangulación: `each_manual_resume_grants_exactly_one_more_bounded_cycle` prueba que cada `resume` concede exactamente `max_iterations` intentos y no abre un bucle.
- **T5-2 · `should-fix` · corregido** — `deps.SecretValues` nunca llegaba al store, así que un `--reason` con una credencial configurada se persistía sin redactar. `red`: `explicit_secrets_never_reach_the_persisted_decision_trail` fallaba en ambas rutas con el valor literal. `green`: `Run` y `Resume` llaman a `SetSecretValues` antes del primer checkpoint.
- **T5-3 · `question` · corregido** — una puerta opcional sin decidir se registraba como `fail`. `red`: el subtest esperaba `skipped` y obtenía `fail`. `green`: nuevo `gates.GateStatusSkipped`.
- **T5-4 · `question` · decidido sin cambio de código** — `resume` sobre `running` es recuperación de caída y la tabla de transiciones ya lo permite. Fijado con `a_run_left_running_by_a_dead_process_is_recoverable`.
- **T6-1 · `nit` · corregido** — `workflow cancel` usaba `status` para el estado de la corrida mientras `approve`/`status` usan `status: "ok"` + `run_status`. `red`: el test esperaba `ok` y obtenía `running`. `green`: `workflowAcknowledgment` compartido.
- **T6-2 · `question` · sin cambio** — `decided_by` no se proyecta en `status`. La especificación pide motivo y fecha (AC-05) y trazabilidad de enlace (AC-09), no autoría; el estado ya lo conserva.

- **Controles ejecutados**: `gofmt -l internal/workflow internal/render/workflow.go internal/render/workflow_test.go` (vacío), `go vet ./...` (`ok`), `go build ./...` (`ok`), `GOOS=linux GOARCH=amd64 go build ./...` (`ok`) y `go test -count=1 ./...` (`19/19` paquetes en verde).
- **Pendiente para `T007`**: llamar a `store.SetSecretValues` al abrir el store, porque `Approve` y `Cancel` dependen de la redacción del store y no reciben `Dependencies`.

## Ronda de corrección T004 · 2026-11-21

Auditoría de doble comprobación sobre `T004` contra su done-check, los `AC` y la especificación, con dos revisores independientes en contexto fresco además del coordinador. Siete hallazgos: seis de código corregidos con `red`→`green` y uno procedimental cerrado en este archivo.

- **T4-1 · `should-fix` · corregido** — un `quality_gate` bloqueante en `fail` no detenía a los dependientes del paso evaluado: `readiness` solo miraba estados de paso, así que un dependiente del paso con gate fallido arrancaba y gastaba cuota sobre salida ya rechazada. `red`: `blocking_gate_failure_skips_dependents_of_the_evaluated_step` observaba `proposal: passed` con dos llamadas al proveedor. `green`: `execution.blockingGateFailed` convierte el gate obligatorio fallido en barrera de rama (los dependientes quedan `skipped`; un gate informativo no bloquea dependientes).
- **T4-2 · `blocker` · corregido** — `optional: true` en un `quality_gate` convertía un fallo obligatorio en `partial` terminal, contra la especificación §115 y la decisión `Q2`. `red`: `optional_quality_gates_are_rejected_before_running` obtenía `partial` con el gate en `fail`. `green`: el preflight rechaza `optional` en `quality_gate` (`invalid_step`) y remite a `informative`, con casos nuevos en `TestWorkflowSchema`.
- **T4-3 · `should-fix` · corregido** — un nombre de artefacto que el store rechaza (`CON`, `NUL`, `COM1`…) pasaba el preflight y fallaba **después** de la llamada al proveedor, dejando la corrida `running`, sin error, con el intento `passed` en memoria frente a `running` en disco y sin forma de cerrarla salvo `cancel`. `red`: `preflight_rejects_artifact_names_the_store_cannot_write` y `mid-flight_store_failure_leaves_a_coherent_failed_checkpoint`. `green`: `schema.isWindowsReservedDevice` adelanta la regla del store, y toda parada interna del coordinador deja checkpoint `failed` con motivo redactado, intento `failed` y pasos pendientes reanudables.
- **T4-4 · `should-fix` · corregido** — un secreto en un entregable cerraba `blocked` con todos los criterios cumplidos y sin ninguna explicación. `red`: `deliverable_with_a_credential_explains_the_block` obtenía `Warnings: []`. `green`: `EvaluateObjectiveWithEvidence` añade un aviso redactado con el nombre del entregable detectado.
- **T4-5 · `nit` · corregido** — `events.jsonl` contradecía el checkpoint que debe corroborar: un reintento de corrección se registraba `step_finished status=pending` sin mensaje y un gate informativo fallido como `gate_evaluated status=passed`. `red`: `events_record_the_attempt_and_gate_verdict`. `green`: `setStepEvent`/`persistEvent` registran el veredicto real (estado del intento o del gate, diagnóstico redactado) sin alterar el estado persistido del paso; un `resume` emite `run_resumed` en vez de un segundo `run_started`.
- **T4-6 · `should-fix` · corregido** — `validate` aceptaba credenciales en metadatos que `run` rechazaba con `secret_in_metadata`. `red`: `validate_rejects_credentials_embedded_in_metadata`. `green`: `engine.Validate` aplica `checkMetadata` y `Run` lo hereda, de modo que ambas puertas coinciden.
- **T4-7 · `should-fix` (proceso) · corregido** — `T004` no tenía informe de cierre y sus seis hallazgos de revisión no constaban como cerrados: [#46](https://github.com/CodeMonkReaper/gandalf-wizard/issues/46) sigue abierto con 0 comentarios y etiqueta `status:ready`, este registro afirmaba que no hubo rondas de revisión y el ledger tenía 36 hallazgos con 30 `outcome` —los seis abiertos eran exactamente `L31`–`L36`—. Corregido: la sección `T004` reconstruye el historial real, el ledger cierra `L31`–`L36` como `fixed` y registra la ronda independiente `L37`–`L42`. **Publicado** (2026-11-21, con autorización del propietario): [informe de cierre en `#46`](https://github.com/CodeMonkReaper/gandalf-wizard/issues/46#issuecomment-7054876916) y etiqueta `status:in-review`; el Issue se cierra con el PR del Corte 2.

- **Controles ejecutados**: `gofmt -l internal/workflow` (vacío), `go vet ./...` (`ok`), `go build ./...` (`ok`), `GOOS=linux GOARCH=amd64 go build ./...` (`ok`), `go test -count=1 ./...` (`19/19` paquetes en verde) y `TestRepoSecurityGate` en verde.
- **Riesgo real (`rsc risk --base HEAD --json`)**: `tier 1` (`classes: ["default"]`, 1 revisor, `humanApproval: false`).
- **Errores del desarrollo**: la primera versión del test de `T4-6` incluía un literal con forma de clave; `TestRepoSecurityGate` lo detectó y se sustituyó por una construcción en tiempo de ejecución. Es el control haciendo su trabajo.
- **Archivos tocados**: `internal/workflow/engine/engine.go`, `execution.go`, `engine_test.go`, `lifecycle_test.go`; `internal/workflow/gates/objective.go`, `gates_test.go`; `internal/workflow/schema/validate.go`, `validate_test.go`.

## Ronda de verificación T005 · 2026-11-21

Auditoría de doble comprobación sobre `T005` contra su done-check y los `AC`, con sondas de contención de lock y de error de escritura. Cinco hallazgos: cuatro de código corregidos con `red`→`green` y uno procedimental cerrado en este archivo, más un riesgo vivo asignado a `T007`.

- **V5-1 · `should-fix` · corregido** — `Cancel` sobre una corrida `failed` terminaba en `cancelled` con el lock libre y en conflicto con el lock ocupado: el mismo estado daba dos resultados según la planificación. `red`: `cancel a failed run without depending on lock contention` observaba `ErrConflict` con el lock retenido. `green`: se elimina la restricción por estado en `Cancel`; todo estado no terminal usa la petición cooperativa (`cancel.req` escrito antes de esperar) y la ventana acotada de confirmación.
- **V5-2 · `nit` · corregido** — `Approve` devolvía el paso como `passed` cuando el artefacto de decisión no se podía escribir, mientras el disco seguía mostrando la puerta pendiente. `red`: `approve reports the persisted state when the decision artifact fails`. `green`: `persistedStateOr` devuelve el checkpoint real en los dos caminos de error de `Approve` y en `cancelLocked`.
- **V5-3 · `question` · corregido** — una corrida fallida por un gate determinista quedaba `failed` con `error` vacío y `resume` repetía en silencio, sin diagnóstico diferencial. `red`: `a blocking gate rejection explains the failed run` leía `error=""`. `green`: `closeObjective` nombra el gate bloqueante que rechazó la salida, en orden determinista, y `resume` conserva el mensaje.
- **V5-4 · `nit` · corregido** — `approval_decided` no llevaba el sentido de la decisión; solo el checkpoint lo tenía. `red`: `the approval decision reaches the event log` leía `message=""`. `green`: el evento registra `decision=approve`/`decision=reject`. El **motivo** sigue solo en `state.json` hasta que `T007` configure los secretos del store (ver `R5-1`).
- **V5-5 · `should-fix` (proceso) · corregido** — `#47` no tenía informe de cierre y su etiqueta seguía en `status:ready`; el ledger mostraba una ronda de `T005` sin hallazgos frente a los cuatro `T5-x` documentados solo aquí. Corregido: sección `T005`, [informe de cierre publicado en `#47`](https://github.com/CodeMonkReaper/gandalf-wizard/issues/47#issuecomment-7055061696), etiqueta `status:in-review`, ronda 2 en el ledger y declaración explícita del hueco histórico.
- **R5-1 · riesgo vivo verificado, asignado a `T007`** — `Approve`/`Cancel` no reciben `Dependencies` y no pueden redactar valores explícitos: con una instancia de store sin `SetSecretValues`, un `--reason` con el centinela aparece literal en `state.json`. Queda como criterio de aceptación de `T007` (configurar el store al abrir) y como razón para no volcar el motivo en `events.jsonl` todavía.

- **Controles ejecutados**: `go test -count=1 -v ./internal/workflow/engine -run '^TestWorkflowEngineLifecycle$'` (`17/17`), `... -run '^TestWorkflowEngineExecution$'` (`14/14`), `TestRepoSecurityGate` (`ok`), `gofmt -l internal/workflow` (vacío), `go vet ./...` (`ok`), `go build ./...` (`ok`), `GOOS=linux GOARCH=amd64 go build ./...` (`ok`) y `go test -count=1 ./...` (`19/19`).
- **Riesgo real (`rsc risk --base HEAD --json`)**: `tier 1` (`classes: ["default"]`, 1 revisor, `humanApproval: false`).
- **Archivos tocados**: `internal/workflow/engine/lifecycle.go`, `execution.go`, `lifecycle_test.go`.

## Ronda de auditoría T006 · 2026-11-21

Auditoría de solo lectura sobre `T006` contra su done-check y los `AC-01`, `AC-05`, `AC-07`, `AC-09`, `AC-10`, con sondas de centinela, inyección de tabla y rutas hostiles. Cuatro hallazgos: un desajuste de contrato documentado y tres detalles, todos corregidos.

- **V6-1 · `should-fix` · corregido** — el plan §3.5 mapeaba `workflow cancel` a `status: cancelled` mientras el código emitía `ok` + `run_status` desde `T6-1`, sin enmienda del plan ni decisión registrada; además la lista «Produce» de `T006` omitía `RenderWorkflowResume`. Corregido: la fila se parte en cancelación directa (`10`) y solicitud cooperativa (`0`), con la nota de convención de los comandos de reconocimiento, `RenderWorkflowResume` añadido y dos entradas en `decisions.md`.
- **V6-2 · `nit` · corregido** — `requested` con `omitempty` desaparecía en la cancelación directa, indistinguible de «campo no aplicable». `red`: el JSON no permitía leer `requested: false`. `green`: el campo pasa a `*bool` y la tabla imprime «Cancelación directa» o «Cancelación solicitada».
- **V6-3 · `nit` · corregido** — ni `.gitignore` (`.gandalf/` solo ignora directorios) ni `scanPathAndContent` (prefijo con barra final) cubrían un fichero llamado exactamente `.gandalf`. Corregido: patrón `.gandalf` antes de `.gandalf/`, comparación por nombre base y tres comprobaciones de `git check-ignore` en el done-check (directorio, fichero en raíz y fichero en subproyecto).
- **V6-4 · `nit` · corregido** — un artefacto con ruta rechazada quedaba con ubicación vacía (`- Artefacto n ·  · 1 bytes`), indistinguible de «sin ruta». Corregido: `rel_path_withheld: true` en JSON y «ruta fuera del espacio de corridas» en la tabla.
- **Cierre publicado** — [informe de cierre en `#48`](https://github.com/CodeMonkReaper/gandalf-wizard/issues/48#issuecomment-7055456123); etiqueta movida de `status:ready` a `status:in-review`.

- **Controles ejecutados**: `go test -count=1 -v ./internal/render -run '^TestWorkflowRender$'` (`4/4`), `go test -count=1 -v ./internal/cli -run '^TestRepoSecurityGate$'` (`3/3`), `gofmt -l internal/render/workflow.go internal/render/workflow_test.go` (vacío), `go vet ./...` (`ok`), `go build ./...` (`ok`), `GOOS=linux GOARCH=amd64 go build ./...` (`ok`), `go test -count=1 ./...` (`19/19`).
- **Sondas de auditoría**: `TestAuditT006` temporal (centinela en 5 comandos × tabla/JSON, inyección de filas, rutas hostiles, `steps` no nulo, envelope de `validate`), `4/4` en verde; fichero eliminado, árbol limpio.
- **Archivos tocados**: `internal/render/workflow.go`, `internal/render/workflow_test.go`, `internal/cli/repo_security_test.go`, `.gitignore`, `02-DOCS/wiki/sdd/plans/workflow-runtime-03.md`, `02-DOCS/wiki/sdd/decisions.md`.
- **Nota de entorno**: `gofmt -l` marca ficheros preexistentes por el checkout CRLF de Windows (`git ls-files --eol` → `i/lf w/crlf`); los blobs de `HEAD` son gofmt-limpios y los ficheros de `T006` están en LF.

## T007 · #49 · 2026-11-21

- **Estado:** completo. `gandalf workflow <validate|run|status|approve|resume|cancel>`, códigos `11–16` y puertos de proveedor configurables.
- **Red:** el done-check se escribió antes de cablear los subcomandos; la primera ejecución falló por la orden de flags documentada (`<run-id> --gate …`) y por expectativas de mapeo, ambas corregidas en la implementación, no relajando asertos.
- **Green:** `go test -count=1 -v ./internal/cli -run '^TestWorkflowCLI$'` — 12 subtests: uso (`2`), preflight condicional (`3`) y validate de solo lectura, run con puertos inyectados y sello de artefactos, memoria con el adaptador ROMERO real, `blocked` (`14`) frente a `failed` (`15`), puerta humana (`11`) + `approve` + `resume` + terminal (`16` con el estado en el payload y el motivo en stderr), interrupción (`12`) + `resume`, cancelación directa (`10`) frente a conflicto terminal, redacción del `--reason` (`R5-1`), corrida inexistente (`3`), router declarado pero no elegible (`3`) y los códigos literales con los puertos de producción.
- **Archivos:** `internal/cli/workflow.go` (nuevo), `internal/cli/workflow_test.go` (nuevo), `internal/cli/exitcodes.go`, `internal/cli/run.go`, `internal/render/workflow.go` (`RenderWorkflowFailure`).
- **Review:** ronda 1 con revisor de contexto fresco (`review-t007`, delegación registrada): cinco hallazgos `L51`–`L55` (tres `should-fix`, una `question`, un `nit`), todos corregidos en `e9eb44a`. Ledger: 55 hallazgos, 55 cerrados.
- **Controles:** `gofmt -l internal/cli/workflow.go internal/cli/workflow_test.go` (vacío), `go vet ./...`, `go build ./...`, `GOOS=linux GOARCH=amd64 go build ./...`, `go test -count=1 ./...` (19/19 paquetes), `TestRepoSecurityGate` en verde. Los ficheros preexistentes que `gofmt -l` marca siguen siendo el checkout CRLF (`i/lf w/crlf`).
- **Riesgo:** `rsc risk --base HEAD~1 --json` → tier 1, 1 revisor.
- **Riesgos residuales:** `R5-1` queda cerrado en el CLI (la credencial llega al store al abrirlo, antes de `approve`/`cancel`); la cancelación cooperativa con un coordinador vivo en otro proceso se demuestra en `T008` con subprocesos reales, porque el bloqueo de corrida es reentrante dentro del mismo proceso.

## T008 · #50 · 2026-11-21

- **Estado:** completo. Evidencia de aceptación del escenario de referencia y del comportamiento entre procesos reales.
- **Red:** `TestWorkflowAcceptance` se escribió contra `analisis-propuesta@1.0.0` / `OBJ-PROP-001@1.0.0` antes de tocar los helpers de proceso; la primera pasada destapó que el preflight exigía proveedores configurados aunque los puertos estuvieran inyectados (corregido en `633fd9f`, no en el test) y que la evidencia de proceso se apoyaba en un plazo fijo (corregido en `678fd1e`).
- **Green:** `go test -count=2 -v ./internal/cli -run '^TestWorkflowAcceptance$'` — 16 subtests: preflight de solo lectura sin llamadas ni estado (`AC-01`), ramas paralelas que se unen en la puerta (`AC-02`), gate bloqueante que rechaza a un productor `passed` y omite al dependiente (`AC-03`), gate informativo que avisa sin bloquear (`AC-03`), ciclo de corrección acotado con diagnóstico (`AC-04`), ciclo agotado en `max_iterations` (`AC-04`), parada 4A que no lanza ramas y reanuda tras aprobar (`AC-05`), cierre `succeeded` con aprobaciones, gates y artefactos (`AC-05`), rechazo que bloquea y prohíbe reanudar (`AC-05`), huella de checkpoint y definición intacta (`AC-06`), resume que no repite pasos concluidos (`AC-07`), conflicto con el lock ocupado (`AC-08`), cierre `partial` con traza y política dura `blocked` (`AC-09`), centinela ausente de salida, estado y disco con el proyecto intacto por digest (`AC-10`) y los tres casos de proceso real.
- **Proceso real:** se compila el CLI, se ejecuta contra un ROMERO falso local y se envía una interrupción de consola: `run` pausa (`12`), `resume` termina (`0`), un segundo proceso recibe el conflicto (`16`) y `cancel` desde otro proceso cierra la corrida (`10`) tras observar `cancel.req`, sin plazo fijo.
- **Archivos:** `internal/cli/workflow_acceptance_test.go`, `internal/cli/workflow_process_test.go`, `internal/cli/interrupt_windows_test.go`, `internal/cli/interrupt_other_test.go`, `internal/cli/workflow.go`.
- **Review:** ronda 1 con revisor de contexto fresco: doce hallazgos `L56`–`L67` (seis `should-fix`, tres `question`, tres `nit`), todos corregidos en `678fd1e`. Ledger: 67 hallazgos, 67 cerrados.
- **Controles:** `gofmt -l` de los cinco ficheros (vacío), `go vet ./...`, `go build ./...`, `GOOS=linux GOARCH=amd64 go build ./...`, `GOOS=linux GOARCH=amd64 go vet ./internal/cli`, `go test -count=1 ./...` (19/19 paquetes). Los ficheros preexistentes que `gofmt -l` marca siguen siendo el checkout CRLF (`i/lf w/crlf`).
- **Riesgo:** `rsc risk --base HEAD~1 --json` → tier 1, 1 revisor.
- **Riesgos residuales:** un gate bloqueante deja su veredicto en `gates[]` y el paso del gate queda `skipped` porque el motor nunca lo lanza; la evidencia fija ese contrato y un cambio de motor lo rompería de forma visible. Las pruebas de proceso dependen de un binario temporal y de señales de consola, por lo que fuera de Windows/Linux usan el helper inerte de `interrupt_other_test.go`.
- **Publicación:** #49 (`T007`) sigue abierto para que lo revise otro agente; el informe de #50 espera autorización humana (`P5`/`P8`).

## T009 · #51 · 2026-11-21

- **Estado:** completo. Verificación de cierre de la épica con veredicto **PASS** registrado en [workflow-runtime-03-closing.md](../verifications/workflow-runtime-03-closing.md).
- **Red:** la primera ronda del gate de cierre falló por un ítem real: AC-03 exige conservar la evidencia generada antes de un gate bloqueante y ningún test leía el artefacto del productor rechazado (`L68`). Se devolvió a `implement` y se cerró en `31c257e`, no relajando el criterio.
- **Green (ronda 2, `31c257ea`, árbol limpio):** T001 5/5, T002 4/4, T003 5/5, T004 14/14, T005 17/17, T006 4/4 + 3/3, T007 12/12 y T008 16/16 subtests; `gofmt -l` acotado al plan vacío; `go vet ./...`, `go build ./...`, `GOOS=linux GOARCH=amd64 go build ./...` y `go vet` cruzado limpios; `go test -count=1 ./...` 19/19 paquetes, `TestRepoSecurityGate` incluido.
- **Recorrido:** las diez claves `AC-01`–`AC-10` quedan mapeadas a prueba concreta en el registro de verificación, con el caso de referencia cerrando `succeeded` y las trazas de gate, aprobación, artefactos y veredicto.
- **Rollback:** rama no integrada (38 commits sobre `6b467282`), cambio aditivo (29 ficheros nuevos, 9 modificados, 0 borrados, `+10686`/`−12`), estado de corridas solo en `.gandalf/` ignorado por Git, sin base de datos, migraciones ni feature flags.
- **Archivos:** `02-DOCS/wiki/sdd/verifications/workflow-runtime-03-closing.md` (nuevo), `02-DOCS/wiki/index.md`, `internal/cli/workflow_acceptance_test.go`.
- **Capas ausentes:** `staticcheck`, `golangci-lint`, `govulncheck` y `go test -race` (sin cgo ni `gcc`); la cobertura se reporta sin umbral por decisión de la constitución. Declaradas en el registro como no ejecutadas, nunca contadas como verdes.
- **Ledger:** 68 hallazgos, 68 cerrados.
- **Publicación:** #50 y #51 no se publican sin autorización humana (`P5`/`P8`); #49 sigue abierto para que lo revise otro agente.

## Decision queue

- **Resuelto · #46 y #47.** Se relee el checkpoint bajo lock y se conserva la traza de gates evaluados. Las pruebas fijan la intercalación de cancelación y cubren corrección agotada, gate hermano y salida de aprobación.
- **Resuelto · #49.** Preflight por paso de Router, selección de trabajo restante al reanudar, artefacto estructurado de ROMERO y almacén de solo lectura para `status`.
- **Verificado · #50, #51 y #40.** La [nueva verificación](../verifications/workflow-runtime-03-2026-11-21-r2.md) terminó en PASS y la [segunda revisión independiente](../reviews/workflow-runtime-03.md#revisión-de-las-correcciones) en APPROVE. El cierre definitivo espera la integración de los cortes pendientes.
