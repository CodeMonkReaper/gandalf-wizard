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
- Modelo ejecutor en Vibe seleccionado por el propietario: `zai-glm-5-3` (`T001`–`T008`; `T009` en coordinador directo).
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

## Decision queue

*(Vacía — sin entradas `[blocking]` ni `[review-after]` abiertas.)*
