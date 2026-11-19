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

## T003 · 2026-11-19

- **Tarea / Issue**: T003 / [#31](https://github.com/CodeMonkReaper/gandalf-wizard/issues/31) — implementar catálogo inmutable y lectura de selección en `internal/synccatalog`.
- **Estado previo (RED)**: el coordinador creó el stub [catalog.go](../../../../internal/synccatalog/catalog.go) y el test inicial [catalog_test.go](../../../../internal/synccatalog/catalog_test.go). Ejecutó `go test -count=1 -v ./internal/synccatalog -run '^TestSyncCatalog$'` y verificó el fallo esperado (`not implemented` en los 3 subtests).
- **Delegación a Vibe (intento 1)**:
  - **Fecha**: 2026-11-19
  - **Task ID / Tier / Modelo**: `sync-t003-catalog` · tier `balanced` · modelo configurado `zai-glm-5-3-flash` (efectivo: no disponible).
  - **Run ID / Estado**: no disponible · `BLOCKED`.
  - **Motivo**: `rsc-vibe-bridge init` omitió `.vibe/agents/developer.toml` porque ya existía la versión de RSC en el repo; al despachar, el bridge bloqueó el envío porque `.vibe/agents/developer.toml` difiere de la política de seguridad del bridge.
  - **Trabajo realizado por Vibe**: ninguno (bloqueado antes de ejecutar).
  - **Verificaciones ejecutadas por el coordinador**: `node <bridge> init`, `node <bridge> doctor` (9 checks ok), `go test -count=1 -v ./internal/synccatalog -run '^TestSyncCatalog$'` (RED confirmado).
  - **Limitaciones y siguiente paso**: requiere decisión del propietario para respaldar `.vibe/agents/developer.toml` e instalar la plantilla segura del bridge (o usar un perfil alternativo del bridge) y reintentar el despacho a Vibe.
- **Delegación a Vibe (intento 2, tras instalar plantilla segura del bridge)**:
  - **Fecha**: 2026-11-19
  - **Task ID / Tier / Modelo**: `sync-t003-catalog` · tier `balanced` · solicitado/efectivo `zai-glm-5-3-flash`.
  - **Run ID / Estado**: `eee4addd-8ead-4df3-b675-fe79972033f5` · `DONE_WITH_CONCERNS` (`stopReason: limit`, 23.7s).
  - **Trabajo realizado por Vibe**: ningún archivo modificado; agotó los 6 turnos leyendo archivos del repo e intentando `bash` (deshabilitado).
  - **Verificaciones ejecutadas por el coordinador**: `git diff --stat internal/synccatalog` (sin cambios), `go test -count=1 -v ./internal/synccatalog -run '^TestSyncCatalog$'` (sigue en RED con `not implemented`), `go vet ./...` y `gofmt -l internal/synccatalog`.
  - **Siguiente paso**: reintento acotado (`sync-t003-catalog-r2`, `balanced`, 6 turnos) entregando la firma de `syncfs` en el prompt e instruyendo escribir `catalog.go` y `catalog_test.go` directamente.
- **Delegación a Vibe (intento 3 / reintento acotado)**:
  - **Fecha**: 2026-11-19
  - **Task ID / Tier / Modelo**: `sync-t003-catalog-r2` · tier `balanced` · solicitado/efectivo `zai-glm-5-3-flash`.
  - **Run ID / Estado**: `d6c2d950-c59a-4aee-a1ac-85ac24c81233` · `DONE_WITH_CONCERNS` (exit 0, 78.6 s; worker sin shell).
  - **Trabajo realizado por Vibe**: implementó [catalog.go](../../../../internal/synccatalog/catalog.go) y añadió 6 subpruebas en [catalog_test.go](../../../../internal/synccatalog/catalog_test.go).
  - **Verificación y ajustes del coordinador**: `go test -count=1 -v ./internal/synccatalog -run '^TestSyncCatalog$'` detectó 3 detalles (rechazo de directorios en `collectSkillResources`, `..` en `validateID` y falta de actualización de `index.json` en `writeEntry`). Tras corregirlos y añadir rechazo de tokens finales con `io.EOF`, pasaron 9/9 subpruebas y los 14 paquetes (`3abc7d5`).
- **Cierre y revisión de T003**:
  - **Estado**: completo. Nivel de revisión 1 (`rsc risk --base 8aa4f5c --json`: tier 1, 1 revisor, sin aprobación humana).
  - **Revisión**: ronda 1 (`3abc7d5`, revisor en contexto fresco) → 6 hallazgos (`L9`–`L14`: 2 blocker, 3 should-fix, 1 nit; `changes-needed`). Todos corregidos con pruebas de regresión en `af9c828` y marcados `fixed` en el ledger. Ronda 2 (`af9c828`, revisor en contexto fresco) → `VERDICT: pass` (0 hallazgos).
  - **Controles ejecutados**: `gofmt -l internal/synccatalog` (limpio), `go vet ./...` (ok), `go build ./...` (ok), `go test -count=1 -v ./internal/synccatalog -run '^TestSyncCatalog$'` (12/12 PASS), `go test -count=1 ./...` (14/14 paquetes en verde). Trazabilidad AC-01, AC-02, AC-05.

## T004 · 2026-11-19

- **Tarea / Issue**: T004 / [#32](https://github.com/CodeMonkReaper/gandalf-wizard/issues/32) — calcular vista previa y proyección sin escribir en `internal/projectsync`.
- **Estado previo (RED)**: el coordinador creó el stub [preview.go](../../../../internal/projectsync/preview.go) y el test inicial [preview_test.go](../../../../internal/projectsync/preview_test.go). Ejecutó `go test -count=1 -v ./internal/projectsync -run '^TestSyncPreview$'` y confirmó el fallo esperado (`not implemented` en 4/4 subtests).
- **Delegación a Vibe (intento 1)**:
  - **Fecha**: 2026-11-19
  - **Task ID / Tier / Modelo**: `sync-t004-preview` · tier `balanced` · solicitado/efectivo `mistral-large-4`.
  - **Run ID / Estado**: `a3cdd524-1cb2-415e-9a41-0da129142ff5` · `BLOCKED` (exit 1, 7.9 s).
  - **Motivo**: la sesión hija no heredó la variable `MISTRAL_API_KEY` recién definida y Vibe usó una clave antigua del keyring.
  - **Trabajo realizado por Vibe**: ninguno.
  - **Siguiente paso**: reintento `sync-t004-preview-r2` cargando `MISTRAL_API_KEY` desde el entorno de usuario de Windows.
- **Delegación a Vibe (intento 2)**:
  - **Fecha**: 2026-11-19
  - **Task ID / Tier / Modelo**: `sync-t004-preview-r2` · tier `balanced` · solicitado/efectivo `mistral-large-4`.
  - **Run ID / Estado**: `d4d37f0f-555e-4ccf-8ef6-bdd22c8ebea8` · `BLOCKED` (exit 2, 7.0 s).
  - **Motivo**: `MISTRAL_API_KEY` en el entorno de usuario contiene un salto de línea final (`\n`); `vibe` lo rechaza en la cabecera HTTP (`Illegal header value`).
  - **Trabajo realizado por Vibe**: ninguno.
  - **Siguiente paso**: limpiar el salto de línea final de `MISTRAL_API_KEY` (y considerar rotar la clave si procede, ya que `vibe` registró la cabecera inválida en su `stderr.log` local) antes de reintentar T004 con Vibe.
- **Delegación a Vibe (intento 3, tras limpiar `MISTRAL_API_KEY` y borrar `stderr.log` anterior)**:
  - **Fecha**: 2026-11-19
  - **Task ID / Tier / Modelo**: `sync-t004-preview-r3` · tier `balanced` (perfil `developer`) · solicitado/efectivo `mistral-large-4`.
  - **Run ID / Estado**: `ac4c6356-bda0-4ad6-9783-fdb1700611a2` · `DONE_WITH_CONCERNS` (exit 0, 157.2 s; worker sin shell).
  - **Trabajo realizado por Vibe**: implementó [preview.go](../../../../internal/projectsync/preview.go) y añadió 4 subpruebas de borde en [preview_test.go](../../../../internal/projectsync/preview_test.go).
  - **Verificación y ajustes del coordinador**: ajustó el mapeo de `synccatalog.Diagnostic` (`Code`/`Message`), el salto de línea al reemplazar `[BlockStart..BlockEnd]` en `AGENTS.md` y el setup de dos subpruebas (`d718fee`).
- **Cierre y revisión de T004**:
  - **Estado**: completo. Nivel de revisión 1 (`rsc risk --base ede5c25 --json`: tier 1, 1 revisor, sin aprobación humana).
  - **Revisión**: ronda 1 (`d718fee`, revisor en contexto fresco) → 4 hallazgos `should-fix` (`L15`–`L18`, 0 blocker; `changes-needed`). Todos corregidos con pruebas de regresión en `6a38ffa` y marcados `fixed` en el ledger. Ronda 2 (`6a38ffa`, revisor en contexto fresco) → `VERDICT: pass` (0 hallazgos).
  - **Controles ejecutados**: `gofmt -l internal/projectsync` (limpio), `go vet ./...` (ok), `go build ./...` (ok), `go test -count=1 -v ./internal/projectsync -run '^TestSyncPreview$'` (9/9 PASS), `go test -count=1 ./...` (15/15 paquetes en verde). Trazabilidad AC-02, AC-03, AC-04, AC-05.
