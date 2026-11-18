---
type: analysis
title: Consistencia — Router 0.2
description: Cobertura y condiciones de implementación para la primera especificación de la épica 15.
tags: [sdd, analysis, router]
timestamp: 2026-11-18T12:40:00Z
topic: sdd
slug: router-02
---

# Consistencia — Router 0.2

**GATE: PASS** del diseño documentado. 0 críticos, 0 altos, 2 medios y 1 bajo. La evidencia viva y la autorización para consumir cuota están pendientes.

Se cruzaron la [constitución](../constitution.md), la [especificación](../specs/router-02.md), las secciones de diseño del [plan](../plans/router-02.md) y su tabla de tareas. Se revisaron cumplimiento, cobertura, alcance, contradicciones, duplicación, ambigüedad y contratos para implementar de forma aislada.

## Mapa de cobertura

| Criterio | Resultado exigido | Plan | Tareas principales | Estado |
| --- | --- | --- | --- | --- |
| AC1 | Catálogo y visibilidad de no autorizados | §3 Catálogo y CLI | T002, T008, T009 | Cubierto |
| AC2 | Ejecutable o credencial ausentes | §3 Catálogo y adaptadores | T002, T005–T007, T009 | Cubierto |
| AC3 | Repetibilidad | §3 Selección | T003, T008, T009 | Cubierto |
| AC4 | Criterio, alternativas y fallback explicados | §3 Selección y CLI | T003, T008, T009 | Cubierto |
| AC5 | Exclusión de ejecutores no autorizados | §3 Configuración, catálogo, selección y ejecución | T001–T009 | Cubierto |
| AC6 | Éxito normalizado con un intento | §3 Ejecución y CLI | T004–T009 | Cubierto |
| AC7 | Fallback ante error y trazas | §3 Ejecución | T004, T008, T009 | Cubierto |
| AC8 | Agotamiento y código propio | §3 Ejecución y CLI | T004, T008, T009 | Cubierto |
| AC9 | Ausencia de elegibles sin ejecución | §3 Selección y CLI | T003, T004, T008, T009 | Cubierto |
| AC10 | Perfil manual sin sustitución | §3 Selección | T003, T004, T008, T009 | Cubierto |
| AC11 | Cancelación real sin fallback | §3 Ejecución y adaptadores, §5 | T004–T009 | Cubierto |
| AC12 | Esfuerzo omitido y explicado | §3 Selección y adaptadores | T003–T009 | Cubierto |
| AC13 | Secretos ausentes de salidas y archivos | §3 Configuración, ejecución y adaptadores, §5 | T001, T004–T007, T009 | Cubierto |
| AC14 | Credencial en configuración rechazada | §3 Configuración | T001, T009 | Cubierto |
| AC15 | Doctor local por ejecutor sin cuota | §3 Catálogo y CLI | T002, T005–T009 | Cubierto |
| AC16 | Proveedor compatible solo por configuración | §3 Configuración y HTTP | T001, T007, T009 | Cubierto |
| AC17 | Regresión de CLI y códigos 0–7 | §0, §3 CLI, §5 | T001, T008–T010 | Cubierto |
| AC18 | Timeout detiene intento y permite fallback | §3 Ejecución y adaptadores, §5 | T004–T009 | Cubierto |
| AC19 | Riesgo alto excluye no verificados | §3 Catálogo y selección | T002, T003, T008, T009 | Cubierto |
| Evidencia transversal | Tres ejecutores vivos, aislamiento y fallback | §5, §7 | T010 | Planificada. Requiere autorización |

## Hallazgos

| ID | Severidad y tipo | Ubicaciones | Hallazgo y efecto | Fase que lo resuelve |
| --- | --- | --- | --- | --- |
| A01 | Media · factibilidad externa | Spec «Fuera de alcance» y «Cómo lo demuestra» · plan §3 y §7 | Las opciones documentadas permiten investigar el aislamiento, pero la evidencia actual no demuestra que los binarios cierren todas las herramientas ni que ambos expongan un sondeo local de sesión. El plan descarta el perfil si falta cualquiera de las dos comprobaciones. Una versión incompatible deja pendiente ese adaptador y el cierre de la spec. | Implementación T005/T006 y verificación T010. Si no se logra, resolver alcance con el propietario |
| A02 | Media · ambigüedad de requisitos | Spec «Qué resultado obtiene» y «Comportamiento» · plan §3 Catálogo | La introducción a los ejemplos habla de mostrar solo modelos verificados, mientras el comportamiento y AC19 necesitan representar perfiles no verificados. El plan reserva «no verificado» para declaraciones o capacidades sin evidencia y evita inventar modelos. Queda pendiente confirmar esa lectura editorial. | Aclaración de la spec |
| A03 | Baja · documentación desactualizada | Issue #15 «Estado ASDD» · spec «Clarifications» y registro de progreso | GitHub conserva el checklist inicial. El avance de la especificación está asentado en la rama local. | Actualización de la issue al publicar las subtareas |

## Dictamen y límites

El plan conserva Go y los contratos de Foundation. Mantiene las dos especificaciones separadas y vincula cada tarea a criterios aprobados. Las coincidencias entre tareas de adaptador, CLI y aceptación corresponden a pruebas en niveles distintos. La configuración global como fuente de autorización y vínculo de credenciales limita el alcance de la configuración de proyectos.

`rsc spec gate` pasó con ocho puntos abiertos tipificados. Se usó `rsc risk` del fork fijado para prever el riesgo de las rutas. Ninguno de esos comandos sustituye la revisión cruzada ni demuestra el funcionamiento de los proveedores. Los done-checks de Go son contratos para la implementación futura.
