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

**GATE: PASS** (0 críticos, 0 altos, 0 medios abiertos — `A01`–`A03` resueltos, 1 bajo informativo `A04`; `A05` sincronizado en GitHub).

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
| `AC-07` | Checkpoints, `Ctrl+C` → `paused`, `failed` recuperable, `resume` idempotente y corrida terminal | §3.3, §3.4, §3.5, §5 | `T003`, `T005`, `T006`, `T007`, `T008`, `T009` | Cubierto (`A01` resuelto) |
| `AC-08` | Cancelación cooperativa (`running`/`paused`/`awaiting_approval`) y exclusión mutua por corrida | §2, §3.3, §3.4, §5 | `T003`, `T005`, `T007`, `T008`, `T009` | Cubierto |
| `AC-09` | Evaluación contra objetivo (`GW-05`), escenario `analisis-propuesta@1.0.0`, `partial` vs `failed` | §3.2, §4, §5 | `T002`, `T004`, `T005`, `T006`, `T007`, `T008`, `T009` | Cubierto |
| `AC-10` | Aislamiento en `.gandalf/workflows/`, cero fuga de secretos y no regresión de `gandalf` | §0, §3.2, §3.5, §5 | `T002`, `T003`, `T004`, `T006`, `T007`, `T008`, `T009` | Cubierto |

- **Cumplimiento constitucional (`P1`–`P10`):** 0 violaciones. Se preservan separación de repositorios (`P1`), decisiones explícitas (`P2`), ASDD previo al código (`P3`), evidencia observable (`P4`), mínimo privilegio solo lectura/solo texto y exclusión Git de `.gandalf/` (`P5`), controles deterministas diferenciados de reglas procedimentales (`P6`), Issues y PR vinculados (`P7`), autoría humana (`P8`), continuidad documental (`P9`) y fuente única de skills (`P10`).
- **Deriva de alcance (`plan/tasks → spec`):** 0 tareas ni bloques sin respaldo en `AC-01`–`AC-10` o `P5`/`P6`. Todo lo declarado en `spec §Fuera de alcance` queda excluido explícitamente en `plan §1`.
- **Completitud de contratos para implementadores aislados:** existe `§0 Restricciones globales` con valores literales y bloque `### Interfaces por tarea` fijado como contrato Go canónico para `T001`–`T009` (`A03` resuelto).

## Hallazgos

| ID | Severidad | Tipo | Artefacto A (ubicación) | Artefacto B (ubicación) | Conflicto / laguna | Estado y resolución |
| --- | --- | --- | --- | --- | --- | --- |
| `A01` | **MEDIA** | Contradicción interna | `plans/workflow-runtime-03.md` §3.5 | `plans/workflow-runtime-03.md` §5 `AC-07` | §3.5 asignaba `16` (`conflict`) a `resume` sobre corrida terminal mientras §5 decía «código de estado sin reejecutar». | **Resuelto en `plan`:** fijado código `16` (`conflict`) en §3.5 y §5 `AC-07`, devolviendo el estado terminal en el payload JSON y diagnóstico. |
| `A02` | **MEDIA** | Ambigüedad | `specs/workflow-runtime-03.md` §Rutas de error y `plans/workflow-runtime-03.md` §3.3 | `plans/workflow-runtime-03.md` §3.5 | `ErrRunNotFound` (`run-id` inexistente) estaba en `store` (`T003`), sin fila explícita en la tabla de salida de §3.5. | **Resuelto en `plan`:** mapeado `ErrRunNotFound` al código `3` (`not_found`) en §0 y §3.5. |
| `A03` | **MEDIA** | Ambigüedad / Carrier completeness | `plans/workflow-runtime-03.md` §3.2–§3.4 | `plans/workflow-runtime-03.md` `### Interfaces por tarea` (`T001`–`T004`) | Diferencias menores entre §3 conceptual y las firmas Go de `T002`–`T004`, más definición de `CriterionSpec` y arista de corrección en `T001`. | **Resuelto en `plan`/`tasks`:** `### Interfaces por tarea` declarada firma Go canónica; §3.2–§3.4 alineadas; `CriterionSpec{ID, Deliverable, Rules}` y arista `router_task.correction.gate_step_id` fijadas en §4 y `T001`. |
| `A04` | **BAJA** | Consistencia operativa | `plans/workflow-runtime-03.md` §0 y `config.yaml` | `plans/workflow-runtime-03.md` `T009` | §0 cita `gofmt -l .` global, mientras `T009` acota `gofmt -l` a los archivos nuevos/modificados de workflow debido a `core.autocrlf=true` en Windows (63 `.go` preexistentes). | **Informativo (aceptado):** mantener alcance acotado en `T009` y registrar la nota en `verifications/`. |
| `A05` | **BAJA** | Sincronización documental | [Issue #40](https://github.com/CodeMonkReaper/gandalf-wizard/issues/40) (`Estado ASDD`) | `specs/workflow-runtime-03.md` y `plans/workflow-runtime-03.md` | El checklist de `#40` estaba sin actualizar y faltaban los sub-Issues de `T001`–`T009`. | **Resuelto en GitHub:** `#40` actualizado con el estado ASDD y creados los sub-Issues [#43](https://github.com/CodeMonkReaper/gandalf-wizard/issues/43)–[#51](https://github.com/CodeMonkReaper/gandalf-wizard/issues/51). |

## Enrutamiento recomendado

1. **Gate listo para `implement` (`0` críticos, `0` altos, `0` medios abiertos, `1` bajo informativo `A04`):** `A01`–`A03` y `A05` quedaron cerrados con aprobación del propietario.
2. **Siguiente paso:** ejecutar `implement` comenzando por `T001` ([#43](https://github.com/CodeMonkReaper/gandalf-wizard/issues/43), `internal/workflow/schema`) en TDD (`red → green`), con delegación en Vibe (`--model zai-glm-5-3`) y Puerta de Cierre Técnico por Issue.

## Evidencia de comprobación

- `rsc spec gate 02-DOCS/wiki/sdd/specs/workflow-runtime-03.md` → `PASS workflow-runtime-03.md — 9 open point(s), 9 typed` (`0` afirmaciones contradichas).
- `rsc risk --json` contra `origin/master` (registrado en `plans/workflow-runtime-03.md`) → `T001`–`T008` `tier 1`, `T009` `tier 0`.
- `rsc-vibe-bridge doctor` → `vibe 3.1.3`, 9/9 comprobaciones locales `ok` (sin inferencia ni consumo de cuota).
- Límite explícito (`P6`): las comprobaciones automáticas no sustituyen la revisión semántica cruzada ni prueban aún comportamiento en runtime (no existe código en `internal/workflow/`).
