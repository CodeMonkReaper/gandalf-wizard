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

## Decision queue

*(Vacía — sin entradas `[blocking]` ni `[review-after]` abiertas.)*
