---
type: analysis
title: Consistencia — Workflow Runtime 0.3
description: Puerta pre-implementación (analyze) cruzando constitución v1.2.0, spec aprobada, plan aprobado y tareas T001–T009.
tags: [sdd, analysis, workflow-runtime, odd, gates]
timestamp: 2026-11-20T18:01:00Z
topic: sdd
slug: workflow-runtime-03
---

# Consistencia — Workflow Runtime 0.3

**GATE: PASS** (0 críticos, 0 altos, 3 medios, 2 bajos).

Se cruzaron la [constitución v1.2.0](../constitution.md), la [especificación aprobada](../specs/workflow-runtime-03.md), el [plan técnico aprobado con tareas T001–T009](../plans/workflow-runtime-03.md), la [configuración SDD](../config.yaml), los [criterios de revisión](../../../../REVIEW.md) y la [épica #40](https://github.com/CodeMonkReaper/gandalf-wizard/issues/40). Se ejecutaron los seis análisis obligatorios: cumplimiento constitucional, cobertura hacia adelante (`spec → plan → tasks`), deriva de alcance (`plan/tasks → spec`), contradicciones, duplicación y ambigüedad / completitud de contratos para implementadores aislados.

## Mapa de cobertura

| Criterio | Requisito de la spec | Plan | Tareas | Estado |
| --- | --- | --- | --- | --- |
| `AC-01` | Preflight solo lectura, `S6` condicional y rechazo de definiciones inválidas | §3.1, §3.4, §5 | `T001`, `T004`, `T006`, `T007`, `T008`, `T009` | Cubierto |
| `AC-02` | Paralelismo real en ramas independientes y barrera de dependencias | §2, §3.4, §5 | `T004`, `T008`, `T009` | Cubierto |
| `AC-03` | 6 reglas deterministas, modo informativo, `no_secrets` → `blocked` y dependientes `skipped` | §0, §3.2, §5 | `T001`, `T002`, `T004`, `T008`, `T009` | Cubierto |
| `AC-04` | Ciclo acotado `max_iterations` (recuperación en intento 2 y parada exacta al agotar) | §3.1, §3.4, §4, §5 | `T001`, `T004`, `T008`, `T009` | Cubierto |
| `AC-05` | Puerta humana `human_approval`, parada conservadora `4A`, `--checkpoint` y rechazo obligatorio/opcional | §0, §3.4, §3.5, §5 | `T005`, `T006`, `T007`, `T008`, `T009` | Cubierto |
| `AC-06` | Integridad `definition_sha256` y alcance estricto de aprobaciones y huellas | §3.1, §3.3, §3.4, §5 | `T001`, `T003`, `T005`, `T007`, `T008`, `T009` | Cubierto |
| `AC-07` | Checkpoints, `Ctrl+C` → `paused`, `failed` recuperable, `resume` idempotente y corrida terminal | §3.3, §3.4, §3.5, §5 | `T003`, `T005`, `T006`, `T007`, `T008`, `T009` | Cubierto (ver `A01`) |
| `AC-08` | Cancelación cooperativa (`running`/`paused`/`awaiting_approval`) y exclusión mutua por corrida | §2, §3.3, §3.4, §5 | `T003`, `T005`, `T007`, `T008`, `T009` | Cubierto |
| `AC-09` | Evaluación contra objetivo (`GW-05`), escenario `analisis-propuesta@1.0.0`, `partial` vs `failed` | §3.2, §4, §5 | `T002`, `T004`, `T005`, `T006`, `T007`, `T008`, `T009` | Cubierto |
| `AC-10` | Aislamiento en `.gandalf/workflows/`, cero fuga de secretos y no regresión de `gandalf` | §0, §3.2, §3.5, §5 | `T002`, `T003`, `T004`, `T006`, `T007`, `T008`, `T009` | Cubierto |

- **Cumplimiento constitucional (`P1`–`P10`):** 0 violaciones. Se preservan separación de repositorios (`P1`), decisiones explícitas (`P2`), ASDD previo al código (`P3`), evidencia observable (`P4`), mínimo privilegio solo lectura/solo texto y exclusión Git de `.gandalf/` (`P5`), controles deterministas diferenciados de reglas procedimentales (`P6`), Issues y PR vinculados (`P7`), autoría humana (`P8`), continuidad documental (`P9`) y fuente única de skills (`P10`).
- **Deriva de alcance (`plan/tasks → spec`):** 0 tareas ni bloques sin respaldo en `AC-01`–`AC-10` o `P5`/`P6`. Todo lo declarado en `spec §Fuera de alcance` queda excluido explícitamente en `plan §1`.
- **Completitud de contratos para implementadores aislados:** existe `§0 Restricciones globales` con valores literales y bloque `### Interfaces por tarea` para `T001`–`T009`; los desajustes menores de firma entre `§3` y `Interfaces` se registran en `A03`.

## Hallazgos

| ID | Severidad | Tipo | Artefacto A (ubicación) | Artefacto B (ubicación) | Conflicto / laguna | Resolver en |
| --- | --- | --- | --- | --- | --- | --- |
| `A01` | **MEDIA** | Contradicción interna | `plans/workflow-runtime-03.md` §3.5 (L153) | `plans/workflow-runtime-03.md` §5 `AC-07` (L187) | §3.5 asigna código `16` (`conflict`) a `resume` sobre corrida terminal, mientras §5 dice «devuelve código de estado sin reejecutar» (lo que implicaría `0`/`10`/`13`/`14`). Si no se fija uno solo, `T007` y `T008` pueden chocar al validar el exit code. | `plan` (alinear §5 a `16` o §3.5 al código terminal) |
| `A02` | **MEDIA** | Ambigüedad | `specs/workflow-runtime-03.md` §Rutas de error (L151) y `plans/workflow-runtime-03.md` §3.3 (L111) | `plans/workflow-runtime-03.md` §3.5 (L141–L153) | `ErrRunNotFound` (`run-id` inexistente en `status`/`approve`/`resume`/`cancel`) está definido en `store` (`T003`), pero falta su fila en la tabla de códigos de salida de §3.5; un worker aislado en `T007` tendría que adivinar si devuelve `2`, `3` o `16`. | `plan` (añadir `ErrRunNotFound` explícitamente en §3.5) |
| `A03` | **MEDIA** | Ambigüedad / Carrier completeness | `plans/workflow-runtime-03.md` §3.2–§3.4 (L96, L107, L113, L124) | `plans/workflow-runtime-03.md` `### Interfaces por tarea` (`T001`–`T004`, L249–L252) | Hay diferencias menores de firma entre §3 conceptual (`StepRecord`/`ArtifactIndex`, `*RunHandle`, `HasCancelRequest -> (bool, string)`, `RouterPort.Execute` posicional) y las firmas Go concretas de `T002`–`T004` (`StepOutcome`/`map[string]string`, `string` runID, `HasCancelRequest` con `error`, `RouterRequest`/`RouterResult`). Además, en `T001` conviene explicitar los campos de `CriterionSpec` (reglas compartidas con `GateSpec` sobre `deliverable`) y la regla DAG de arista de corrección permitida (`router_task.correction.gate_step_id` → `quality_gate` bloqueante dependiente). | `plan` / `tasks` (declarar `### Interfaces por tarea` como firma Go canónica y precisar `CriterionSpec` + arista de corrección) |
| `A04` | **BAJA** | Consistencia operativa | `plans/workflow-runtime-03.md` §0 (L20) y `config.yaml` (L28) | `plans/workflow-runtime-03.md` `T009` (L243) | §0 cita `gofmt -l .` global, mientras `T009` acota `gofmt -l` a los archivos nuevos/modificados de workflow debido al comportamiento ya conocido de `core.autocrlf=true` en Windows (63 archivos `.go` preexistentes sin tocar). Coherente con la evidencia de `#41` y `sync-skills-patrones`. | Informativo (mantener alcance acotado en `T009` y registrar la nota en verificación) |
| `A05` | **BAJA** | Sincronización documental | [Issue #40](https://github.com/CodeMonkReaper/gandalf-wizard/issues/40) (`Estado ASDD`) | `specs/workflow-runtime-03.md` y `plans/workflow-runtime-03.md` | En GitHub el checklist de `#40` conserva los hitos sin marcar y aún no existen sub-Issues, mientras en local ya están aprobados spec, plan y `tasks` (`d7115a6`). | Sincronización autorizada de GitHub Issues al abrir `T001`–`T009` |

## Enrutamiento recomendado

1. **Sin bloqueadores (`0` críticos, `0` altos):** el gate abre paso a `implement` una vez resueltos o aceptados conscientemente `A01`–`A03`.
2. **Ajuste recomendado antes de `implement` (1 edición mínima en `plan`/`tasks`):**
   - `A01`: confirmar código `16` (`conflict`) para `resume` sobre corrida terminal (incluyendo el estado terminal en el payload JSON y mensaje) y alinear §5 `AC-07`.
   - `A02`: mapear `ErrRunNotFound` a código `3` (recurso/estado no encontrado, consistente con configuración/archivo inválido) en §3.5.
   - `A03`: fijar `### Interfaces por tarea` como contrato Go canónico sobre §3 conceptual y detallar `CriterionSpec` + validación de arista declarada `correction.gate_step_id` en `T001`.
3. **Tras ese ajuste (o aceptación explícita):** actualizar `#40` y crear sub-Issues `T001`–`T009` en GitHub con el modelo ejecutor de Vibe elegido por el propietario.

## Evidencia de comprobación

- `rsc spec gate 02-DOCS/wiki/sdd/specs/workflow-runtime-03.md` → `PASS workflow-runtime-03.md — 9 open point(s), 9 typed` (`0` afirmaciones contradichas).
- `rsc risk --json` contra `origin/master` (registrado en `plans/workflow-runtime-03.md`) → `T001`–`T008` `tier 1`, `T009` `tier 0`.
- `rsc-vibe-bridge doctor` → `vibe 3.1.3`, 9/9 comprobaciones locales `ok` (sin inferencia ni consumo de cuota).
- Límite explícito (`P6`): las comprobaciones automáticas no sustituyen la revisión semántica cruzada ni prueban aún comportamiento en runtime (no existe código en `internal/workflow/`).
