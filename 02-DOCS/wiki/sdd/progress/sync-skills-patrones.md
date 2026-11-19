---
type: progress
title: Avance de la sincronización reproducible
timestamp: 2026-11-19T16:29:17Z
topic: sdd
slug: sync-skills-patrones
---

# Avance de la sincronización reproducible

[Épica #28](https://github.com/CodeMonkReaper/gandalf-wizard/issues/28) · [Especificación](../specs/sync-skills-patrones.md) · [Plan y tareas](../plans/sync-skills-patrones.md)

## Inicio del lote T001–T002

- Rama de trabajo `feat/sync-skills-patrones`. Base `6ca99cb`.
- `go test -count=1 ./...` pasó en los once paquetes existentes antes del cambio.
- T001 trabaja sobre el ensayo nativo de Codex y Vibe en `internal/syncnative`. T002 trabaja sobre la escritura confinada de Windows en `internal/syncfs`. Cada ensayo usa sus propios directorios temporales.
- Se aplican las guías `implement`, `parallel` y `go`. Primero se observa el fallo que debe detectar la prueba. Después se prueba el resultado en los procesos y archivos reales que exige el plan.
- El perfil dedicado de implementación no se pudo iniciar con esta cuenta. Ambos ensayos se delegaron a agentes con un modelo disponible.

## T001 · 2026-11-19

- Estado completo. Nivel de revisión 1. Cambios en 6adc4f0 y 8347c9e.
- Las aserciones fallaron con la captura vacía. Después, el ensayo con ambos CLI reales pasó seis casos, incluidos los controles negativos.
- Ronda 1 encontró acceso potencial al almacén de Windows durante el inicio de Vibe. Se comprobó el código del paquete instalado. La prueba de entorno falló sin el bloqueo y pasó tras añadirlo. Con el bloqueo, los seis casos volvieron a pasar y el proxy registró cero peticiones auxiliares. La primera ejecución carece de evidencia sobre acceso efectivo al almacén.
- Ronda 2 aprobó la corrección. Trazabilidad AC-03 y AC-05. La prueba completa después de aplicar la selección sigue en T008. [Evidencia de descubrimiento](../verifications/sync-skills-patrones-native.md)

## T002 · 2026-11-19

- Estado bloqueado antes de integrar. Nivel de revisión 1. Ensayo local en 8d6dfb6 y correcciones en 75cd2bd.
- Pasaron los casos de confinamiento, anclaje de directorios, exclusión entre procesos, recuperación tras terminar al titular y estados antes y después de sustituir. Las regresiones de longitud y flujos NTFS fallaron primero y pasaron tras corregirse.
- Ronda 1 identificó cuatro hallazgos. Dos se corrigieron con pruebas. Los otros dos afectan la publicación condicionada a la identidad verificada y la trazabilidad de temporales que pueden sobrevivir.
- El plan aprobado separa la primitiva de archivos de la recuperación completa. La alternativa en evaluación acopla ese tramo al registro de operaciones para identificar temporales antes de publicar. El reemplazo necesita un protocolo que preserve una edición externa concurrente. [Evidencia de Windows](../verifications/sync-skills-patrones-filesystem.md)


## T002 · cierre · 2026-11-19

- Estado completo. Nivel de revisión 1 (`rsc risk --base 6ca99cb`: tier 1, 1 revisor, sin aprobación humana).
- Q1 y Q2 decididos por el propietario (precondición de identidad; acoplamiento al registro de operaciones) y registrados en [decisions.md](../decisions.md). Implementación en `53de71b` (lease con operación, temporales `.gandalf-sync-<op>-<id>.tmp`, guard de identidad) y `c525306` (publicación sin reemplazo: el enlace verificado se elimina por handle con semántica POSIX y el renombrado no usa bandera de reemplazo).
- red: `precondicion_de_identidad_antes_de_publicar/reemplazo_tras_la_verificacion` devolvía `changed=true` sobrescribiendo la edición externa; `TryExclusive("")` se aceptaba. green: `go test -count=1 -v ./internal/syncfs -run '^TestSyncFilesystem$'` pasa con los seams antes y después de la verificación, el modo `armed` (proceso pausado dentro de `Change` tras `keepStagedFile`) y exclusión con ids de operación distintos.
- revisión: ronda 1 (4 hallazgos, 2 corregidos) → ronda 2 (3 hallazgos: L6 blocker TOCTOU, L7/L8 cobertura; veredicto changes-needed) → ronda 3 (veredicto pass, 0 hallazgos, revisor Antigravity en contexto fresco). L6–L8 registrados como `fixed`. La advertencia del ledger sobre AC-07 (objeciones distintas en rondas 1 y 2) quedó atendida con las pruebas de interrupción reforzadas de la ronda 3.
- controles: `go test -count=1 ./...` en verde (13 paquetes), `go vet ./...`, `go build ./...`, compilación cruzada Linux amd64 y `gofmt -l internal/syncfs` limpio. Trazabilidad AC-05, AC-06, AC-07.
- decisión: Q1 y Q2 cerrados en la cola de decisiones (ver arriba). El plan se enmendó con la interfaz `TryExclusive(operation)` y la publicación sin reemplazo.

## Controles del lote

- La suite `go test -count=1 ./...` pasó en los 13 paquetes con el ensayo nativo habilitado.
- `go vet ./...` y `go build ./...` pasaron en Windows. Pasó la compilación cruzada a Linux amd64.
- `TestRepoSecurityGate` pasó dentro de la suite. Los archivos Go nuevos pasaron `gofmt -l internal/syncnative internal/syncfs`. El chequeo global también enumera archivos preexistentes con finales CRLF.
- Los controles respaldan los casos ejecutados. Q1 y Q2 bloquean la integración de la escritura para T002.

## Decision queue

- [closed 2026-11-19, propietario] Q1 — T002, proteger un archivo que otro programa reemplaza entre la comprobación y la publicación. Decisión: precondición de identidad — reverificar junto al reemplazo, eliminar el enlace verificado por handle y publicar sin reemplazo; ante identidad distinta, `ErrConflict` conservando la edición externa. Casos AC-05 y AC-07. Implementado en `c525306`.
- [closed 2026-11-19, propietario] Q2 — T002, atribuir los temporales a una operación antes de que puedan sobrevivir al proceso. Decisión: el lease lleva el identificador de operación (`TryExclusive(operation)`) y los temporales se nombran `.gandalf-sync-<operación>-<id>.tmp`. Implementado en `53de71b` y probado desde `Change` en `c525306`. Caso AC-07.
