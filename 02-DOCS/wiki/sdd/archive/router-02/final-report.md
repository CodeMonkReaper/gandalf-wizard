---
type: archive
title: Archivo — router-02 — Etapa 0.2 Router cerrada
description: Bundle final de la épica #15: qué se entregó, decisión de aterrizaje y enlaces de evidencia.
tags: [sdd, archive, router]
timestamp: 2026-11-18T16:32:00Z
topic: sdd
slug: router-02
---

# Archivo — router-02 (2026-11-18)

## Qué se entregó

Router 0.2 en Go (solo stdlib), épica [#15](https://github.com/CodeMonkReaper/gandalf-wizard/issues/15):

- `internal/config` — sección `router` en configuración por capas: ejecutores y autorización solo en capa global, perfiles ordenados y plazo por intento sustituibles por proyecto, rechazo de claves de credencial por proveedor (`IsForbiddenCredentialKey` preservando `credential_env`) y avisos de claves desconocidas en `executors` y `profiles`.
- `internal/router` — catálogo (`Build` separando disponibilidad, verificación y origen), selector determinista explicable (`Select`, con complejidad/riesgo, omisión explicada de esfuerzo y modo manual `--profile`) y orquestador (`Run` con fallback secuencial, timeout por intento, cancelación por contexto y redacción de secretos).
- `internal/router/antigravity`, `internal/router/vibe`, `internal/router/direct` — adaptadores en modo solo texto verificado en vivo (sin auto-approve, aislamiento de directorio y entorno) y cliente HTTP directo sin redirects ni reintentos para `openai`, `mistral` (y compatibles por configuración) y `gemini`.
- `internal/render`, `internal/doctor`, `internal/cli`, `cmd/gandalf` — comandos `models list` (con descubrimiento sin cuota), `route`, `run`, sección informativa `executors` en `doctor`, códigos de salida `8`–`10` (conservando `0`–`7`), propagación de `Ctrl+C` (`signal.NotifyContext`) y gate `TestRepoSecurityGate` contra fuga de claves API y sesiones locales.

## Por qué

Primera especificación de la etapa 0.2 Router: convertir la elección manual de ejecutor, modelo y esfuerzo en un contrato determinista, explicable y con fallback seguro en modo solo texto.

## Decisión de aterrizaje

PR (opción 1 de `ship`), autorizado por el propietario: [PR #27](https://github.com/CodeMonkReaper/gandalf-wizard/pull/27) mergeado a `master` en `abf5901` (2026-11-18). Sub-issues `T001`–`T010` (#17–#26) cerrados antes del merge; épica #15 actualizada y cerrada tras separar `sync-skills-patrones` en la nueva épica [#28](https://github.com/CodeMonkReaper/gandalf-wizard/issues/28). Rama `feat/router-02` borrada en remoto y local.

## Evidencia

- Progreso y cola de decisiones (`Q1`–`Q8`): [progress/router-02.md](../../progress/router-02.md)
- Verificación viva (sondas, 3 ejecutores y fallback): [verifications/2026-11-18-router-02-live-verification.json](../../verifications/2026-11-18-router-02-live-verification.json)
- Verificación de cierre (PASS): [verifications/2026-11-18-router-02-closing.md](../../verifications/2026-11-18-router-02-closing.md)
- Revisión de cierre (APPROVE): [reviews/router-02.md](../../reviews/router-02.md)
- Spec: [specs/router-02.md](../../specs/router-02.md) · Plan: [plans/router-02.md](../../plans/router-02.md) · Análisis: [analysis/router-02.md](../../analysis/router-02.md)
- Ledger de revisión de tareas `T001`–`T009`: 12 hallazgos únicos (`L11`–`L22`), 12 `fixed`, 0 abiertos, media 1.00 ronda; revisión de cierre ronda 1 (`3 should-fix`, `1 nit`) resuelta en `cef75ff`, ronda 2 (`APPROVE`, `0` hallazgos nuevos).
