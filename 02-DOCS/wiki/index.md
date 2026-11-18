---
type: index
title: Conocimiento de Gandalf Wizard
description: Punto de entrada compartido por Vibe, Codex y Antigravity.
tags: [harness, knowledge, sdd]
timestamp: 2026-11-18T01:50:00Z
topic: harness
---

# Conocimiento del proyecto

Al retomar, leer el [checkpoint vigente](sdd/sessions/2026-11-18-retro-cli-gandalf.md) (retro cli-gandalf; anteriores: [cierre](sdd/sessions/2026-11-18-cierre-cli-gandalf.md), [post-merge](sdd/sessions/2026-11-18-post-merge.md), [2026-11-17](sdd/sessions/2026-11-17-entorno-rsc.md)), después los documentos necesarios para la tarea.

- Producto: [contexto maestro](../../gandalf-wizard-contexto-maestro.md), inventario conceptual, no especificación aprobada.
- Harness: [instalación y capacidades](harness/installation.md), [plan generado](harness/installation-plan.md), [decisiones](harness/decisions.md).
- ASDD: [constitución ratificada v1.2.0](sdd/constitution.md), [configuración SDD](sdd/config.yaml), [convenciones Go](stack/go.md), [decisiones](sdd/decisions.md), [criterios de revisión](../../REVIEW.md). Repositorio público: [CodeMonkReaper/gandalf-wizard](https://github.com/CodeMonkReaper/gandalf-wizard) ([Issue #1](https://github.com/CodeMonkReaper/gandalf-wizard/issues/1): entorno RSC/ASDD, cerrado en PR #4).
- Foundation 0.1 ([Issue #2](https://github.com/CodeMonkReaper/gandalf-wizard/issues/2), cerrado en PR #3) cuenta con la [especificación aprobada de consulta de memoria en ROMERO](sdd/specs/consulta-memoria-romero.md), su [plan técnico aprobado con tareas](sdd/plans/consulta-memoria-romero.md), su [informe de consistencia (`analyze`: PASS, 0 hallazgos abiertos)](sdd/analysis/consulta-memoria-romero.md) y el [registro de progreso](sdd/progress/consulta-memoria-romero.md) (`T001`–`T009` completadas e integradas en `master` mediante `30bd22c`).
- Épica CLI Gandalf ([Issue #5](https://github.com/CodeMonkReaper/gandalf-wizard/issues/5), cerrada en el [PR #14](https://github.com/CodeMonkReaper/gandalf-wizard/pull/14), merge `c81a536`): [especificación entregada](sdd/specs/cli-gandalf.md), [plan técnico `T001`–`T010`](sdd/plans/cli-gandalf.md), [informe de consistencia (`analyze`: PASS)](sdd/analysis/cli-gandalf.md), [registro de progreso y cola de decisiones](sdd/progress/cli-gandalf.md), [verificación de cierre (PASS)](sdd/verifications/2026-11-18-cli-gandalf-closing.md), [revisión de cierre (APPROVE)](sdd/reviews/cli-gandalf.md) y [bundle de archivo](sdd/archive/cli-gandalf/final-report.md).
- Épica Router 0.2 ([Issue #15](https://github.com/CodeMonkReaper/gandalf-wizard/issues/15), abierta): adaptadores Antigravity + Mistral Vibe, APIs directas, catálogo de modelos/esfuerzos, selección explicable y fallback; incluye la sincronización de skills y patrones diferida de 0.1. Dos specs (`router-02`, luego `sync-skills-patrones`) en autopilot; decisiones de frontera en [decisions.md](sdd/decisions.md).
  - Router 0.2 tiene [especificación aclarada](sdd/specs/router-02.md), [plan técnico con tareas](sdd/plans/router-02.md), [informe de consistencia](sdd/analysis/router-02.md) y [registro de avance](sdd/progress/router-02.md).
- FTD: [limpieza post-merge y alineación documental de Foundation 0.1](ftd/limpieza-post-merge-foundation-01.md).
- Evidencia: [verificación del entorno](sdd/verifications/2026-11-17-entorno-rsc.json), [ratificación y doctor posterior](sdd/verifications/2026-11-17-constitution-ratification.json), [integración viva con ROMERO (`rev 3f46f9a`)](sdd/verifications/2026-11-18-romero-live-integration.json), [verificación de cierre de cli-gandalf (PASS)](sdd/verifications/2026-11-18-cli-gandalf-closing.md).

Stack inicial (Go) y controles de calidad fijados en `v1.1.0`; Foundation 0.1 entregada y verificada en vivo contra ROMERO; la sincronización de skills y patrones queda diferida a la siguiente etapa ([decisiones](sdd/decisions.md)); las decisiones de fases futuras del contexto maestro siguen abiertas.
