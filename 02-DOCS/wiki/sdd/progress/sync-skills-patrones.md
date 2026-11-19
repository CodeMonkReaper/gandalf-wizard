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

## Controles del lote

- La suite `go test -count=1 ./...` pasó en los 13 paquetes con el ensayo nativo habilitado.
- `go vet ./...` y `go build ./...` pasaron en Windows. Pasó la compilación cruzada a Linux amd64.
- `TestRepoSecurityGate` pasó dentro de la suite. Los archivos Go nuevos pasaron `gofmt -l internal/syncnative internal/syncfs`. El chequeo global también enumera archivos preexistentes con finales CRLF.
- Los controles respaldan los casos ejecutados. Q1 y Q2 bloquean la integración de la escritura para T002.

## Decision queue

- [blocking] Q1 — T002, proteger un archivo que otro programa reemplaza entre la comprobación y la publicación. El plan revisado debe demostrar publicación sin sobrescritura de una entrada nueva o una precondición de identidad equivalente. Casos AC-05 y AC-07.
- [blocking] Q2 — T002, atribuir los temporales a una operación antes de que puedan sobrevivir al proceso. Coordinar la interfaz de escritura con el registro de T005 y probar interrupciones en ese punto. Caso AC-07.
