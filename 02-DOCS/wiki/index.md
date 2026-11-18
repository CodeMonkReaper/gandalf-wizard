---
type: index
title: Conocimiento de Gandalf Wizard
description: Punto de entrada compartido por Vibe, Codex y Antigravity.
tags: [harness, knowledge, sdd]
timestamp: 2026-11-18T00:03:05Z
topic: harness
---

# Conocimiento del proyecto

Al retomar, leer el [checkpoint vigente](sdd/sessions/2026-11-18-post-merge.md) (post-merge; anterior: [2026-11-17](sdd/sessions/2026-11-17-entorno-rsc.md)), después los documentos necesarios para la tarea.

- Producto: [contexto maestro](../../gandalf-wizard-contexto-maestro.md), inventario conceptual, no especificación aprobada.
- Harness: [instalación y capacidades](harness/installation.md), [plan generado](harness/installation-plan.md), [decisiones](harness/decisions.md).
- ASDD: [constitución ratificada v1.2.0](sdd/constitution.md), [configuración SDD](sdd/config.yaml), [convenciones Go](stack/go.md), [decisiones](sdd/decisions.md), [criterios de revisión](../../REVIEW.md). Repositorio público: [CodeMonkReaper/gandalf-wizard](https://github.com/CodeMonkReaper/gandalf-wizard) ([Issue #1](https://github.com/CodeMonkReaper/gandalf-wizard/issues/1): entorno RSC/ASDD).
- Foundation 0.1 ([Issue #2](https://github.com/CodeMonkReaper/gandalf-wizard/issues/2)) cuenta con la [especificación aprobada de consulta de memoria en ROMERO](sdd/specs/consulta-memoria-romero.md), su [plan técnico aprobado con tareas](sdd/plans/consulta-memoria-romero.md), su [informe de consistencia (`analyze`: PASS, 0 hallazgos abiertos)](sdd/analysis/consulta-memoria-romero.md) y el [registro de progreso](sdd/progress/consulta-memoria-romero.md) (`T001`–`T004` completadas en `feat/consulta-memoria-romero`). Siguiente tarea: `T005`.
- Evidencia: [verificación del entorno](sdd/verifications/2026-11-17-entorno-rsc.json), [ratificación y doctor posterior](sdd/verifications/2026-11-17-constitution-ratification.json).

Stack inicial (Go) y controles de calidad fijados en `v1.1.0`; la instancia autorizada de ROMERO y decisiones de fases futuras siguen abiertas.
