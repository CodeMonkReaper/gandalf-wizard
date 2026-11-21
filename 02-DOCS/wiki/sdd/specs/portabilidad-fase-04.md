---
type: spec
title: Fase 04 — Prueba de portabilidad entre Codex y OpenCode
slug: portabilidad-fase-04
status: approved
clarified: 2026-11-21
scope_approved: 2026-11-21
topic: sdd
---

# Prueba de portabilidad entre Codex y OpenCode

## Objetivo

El operador debe poder ejecutar el análisis de una propuesta técnica en dos entornos reales y auditar si ambos conservan el objetivo, las restricciones y los controles humanos. El resultado permitirá decidir si la fase 04 cumple AC-04. Rigen la [constitución ratificada](../constitution.md) y el [escenario B.1 de la especificación maestra](../../../../Gandalf_Wizard_v1.1.0/docs/GANDALF_WIZARD_ESPECIFICACION_MAESTRA.md).

## Caso de referencia

Usar una única definición versionada de `analisis-propuesta` 1.0.0 y su objetivo `OBJ-PROP-001`. El flujo consulta contexto en ROMERO, identifica riesgos, sintetiza una recomendación y solicita revisión humana. Los dos destinos parten del mismo commit, entrada, contexto de prueba y versiones de metodología, skill y reglas. El alcance comprende el flujo de solo texto y sus pruebas de contrato. Otros ejecutores, interfaces y comparaciones de rendimiento quedan para fases posteriores. [Definición de referencia](../../../../internal/portability/reference.go)

## Criterios de aceptación aprobados

| ID | Condición observable en cada ejecutor |
| --- | --- |
| P1 | Antes de ejecutar, el registro identifica la versión real del ejecutor, la revisión de la definición y el catálogo fijado. El manifiesto de cada destino permite comprobar dependencias, entradas, límites y permisos equivalentes. Toda diferencia está explicada. |
| P2 | La comprobación previa contrasta cada capacidad requerida con evidencia por versión. Una prueba con una capacidad obligatoria ausente detiene la ejecución e identifica el paso afectado. |
| P3 | En un entorno aislado, la síntesis recibe el contexto y los riesgos. Los riesgos tienen contenido y la recomendación es JSON válido con las claves `decision` y `riesgos`. Ambas salidas respetan las restricciones de texto y confidencialidad. Los mismos validadores evalúan ambas corridas. |
| P4 | Una síntesis inválida activa la corrección permitida por la definición. Si persiste el defecto tras agotar el límite de dos iteraciones, la corrida registra el fallo antes de solicitar aprobación. |
| P5 | Con una síntesis válida, el flujo se detiene para revisión humana. Al interrumpir y reanudar en esa espera, conserva los artefactos, la decisión pendiente y una sola ejecución de cada paso ya completado. Solo una aprobación explícita permite el cierre satisfactorio. Un rechazo queda registrado e impide ese cierre. |
| P6 | El informe vincula objetivo, pasos, artefactos, controles y decisión humana. Incluye eventos, accesos y consumo observado o estimado con su procedencia. Si falta una medición, declara la ausencia. La evidencia compartida protege secretos. |

## Veredicto

Registrar un resultado por criterio y por ejecutor con evidencia de la corrida real. La portabilidad queda demostrada cuando ambos cumplen todos los criterios con la misma definición. Un incumplimiento observado se registra como fallo. Una capacidad o un acceso ausente se registra como impedimento con versión, criterio afectado y evidencia. Una ejecución simulada sirve para probar el contrato local y carece de valor probatorio sobre el ejecutor real.

## Decisiones aprobadas

El propietario aprobó el alcance y los criterios P1–P6 el 21 de noviembre de 2026. Para la consulta «decisiones previas propuesta», ambos ejecutores recibirán la misma copia fija y versionada de la respuesta de ROMERO. Cada corrida registrará la versión utilizada.

El plan usará dos directorios locales aislados, uno para Codex y otro para OpenCode. Ambos partirán del mismo commit y tendrán permisos equivalentes. Las autorizaciones de servicios externos siguen el procedimiento del proyecto.
