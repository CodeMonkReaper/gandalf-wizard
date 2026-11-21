---
type: analysis
title: Consistencia de la sincronización reproducible
description: Cobertura y condiciones de implementación para los nueve criterios de la épica 28.
tags: [sdd, analysis, sync-skills-patrones]
timestamp: 2026-11-19T16:12:36Z
topic: sdd
slug: sync-skills-patrones
---

# Consistencia de la sincronización reproducible

**GATE: PASS.** 0 críticos, 0 altos, 0 medios, 0 bajos en los documentos. Quedan pendientes los ensayos técnicos de T001 y T002.

Se cruzaron la [constitución](../constitution.md), la [especificación aprobada](../specs/sync-skills-patrones.md) y el [plan aprobado con tareas](../plans/sync-skills-patrones.md). La revisión cubrió cumplimiento, trazabilidad, alcance, contradicciones, duplicación y ambigüedad.

## Cobertura

| Criterio | Requisito | Plan | Tareas principales |
| --- | --- | --- | --- |
| AC-01 | Catálogo, versión inmutable, procedencia y referencia del patrón | §3 Catálogo, §4 Selección | T003, T007, T008 |
| AC-02 | Vista previa sin escrituras y selección vacía | §3 Vista previa, §5 Pruebas | T003, T004, T007, T008 |
| AC-03 | Reproducción en dos copias y conformidad nativa de dos CLI | §4 Proyección, §5 Pruebas | T001, T004, T005, T008 |
| AC-04 | Autorización vinculada, preservación de texto y repetición | §3 Aplicar, §4 Flujo | T004, T005, T007, T008 |
| AC-05 | Compatibilidad, conflictos, confinamiento y exclusión | §3 Interfaces, §4 Flujo, §7 Riesgos | T001–T006, T008 |
| AC-06 | Respaldo y restauración de altas y actualizaciones | §4 Aplicar y recuperar, §5 Pruebas | T002, T005, T006, T008 |
| AC-07 | Interrupciones, reconciliación y reintento | §4 Aplicar y recuperar, §5 Pruebas | T002, T005, T006, T008 |
| AC-08 | Privacidad de salidas, evidencias y respaldos | §0 Restricciones, §4 Estado local, §5 Pruebas | T005, T007, T008 |
| AC-09 | Regresión de la CLI y controles de Go | §0 Restricciones, §5 Pruebas | T007, T009 |

T009 vuelve a ejecutar las comprobaciones del conjunto y recoge el resultado de cierre.

## Hallazgos

| Severidad | Artefactos | Resultado | Fase |
| --- | --- | --- | --- |
| — | Constitución P3–P8, spec y plan §0–§7 | El plan conserva Go, TDD, autorización, evidencia y aprobación humana para publicación. | — |
| — | Spec AC-01–AC-09 y plan §5 y Tasks | Los nueve criterios tienen comprobaciones previstas. Cada tarea tiene criterio, dependencia, interfaz y comprobación. T001 caracteriza descubrimiento y T008 prueba el flujo completo con Gandalf. | — |
| — | Spec «Fuera de alcance», plan §4 y Tasks | El catálogo vincula los patrones por versión. La escritura se limita a skills e instrucciones declarativas en destinos verificados. El plan conserva archivos y fragmentos ajenos. | — |

La comprobación automatizada confirmó nueve identificadores consecutivos, nueve criterios cubiertos, referencias a dependencias anteriores y bloques de interfaz para todas las tareas. T001 y T002 son las únicas tareas iniciales marcadas como independientes.

## Condiciones para implementar

T001 debe demostrar descubrimiento de la skill y carga de la regla en ambos CLI reales. T002 debe comprobar reemplazo recuperable, confinamiento y exclusión con procesos reales en Windows. La evidencia determinará qué combinaciones se habilitan. T008 debe repetir la comprobación después de pasar por la CLI de Gandalf. Este veredicto acredita la consistencia de los documentos.
