---
type: archive
title: Archivo — sync-skills-patrones — Épica #28 cerrada
description: Bundle final de la épica #28: qué se entregó, decisión de aterrizaje y enlaces de evidencia.
tags: [sdd, archive, sync-skills-patrones]
timestamp: 2026-11-20T03:11:00Z
topic: sdd
slug: sync-skills-patrones
---

# Archivo — sync-skills-patrones (2026-11-20)

## Qué se entregó

Sincronización reproducible local de skills, reglas consultivas y referencias de patrones en Go (solo stdlib), épica [#28](https://github.com/CodeMonkReaper/gandalf-wizard/issues/28):

- `internal/syncfs` — confinamiento NTFS en Windows, anclaje de directorios padre, rechazo de enlaces/reparse/hardlinks/ADS/nombres DOS/UTF-16 largos, precondición de identidad (`Q1`), nombrado de temporales con ID de operación `.gandalf-sync-<op>-*.tmp` (`Q2`), exclusión entre procesos (`TryExclusive`), ACL restringida al propietario (`RestrictDir`) y descubrimiento/limpieza confinada de temporales (`HasOperationStage` / `CleanupOperationStages`), con stubs `!windows` para compilación cruzada `linux/amd64`.
- `internal/synccatalog` — catálogo local inmutable para skills, reglas y patrones por tipo/id/versión con procedencia, huella canónica SHA-256, validación de dependencias, detección de manipulación y restricción inmediata de `.gandalf/sync` (ACL + `.gitignore`).
- `internal/projectsync` — vista previa determinista de solo lectura vinculada a `View.ID`, aplicación autorizada con respaldo previo y diario recuperable, y restauración reanudable que conserva ediciones externas y recupera archivos preexistentes tras cortes entre `unlinkHandle` y `renameHandle`.
- `internal/cli` — comandos `gandalf catalog register|list` y `gandalf sync preview|apply|restore` (JSON estable + texto sin volcar contenido de archivos ni secretos, preservando `status`, `reason`, `operation_id` y `diagnostics`).
- `internal/syncnative` — pruebas nativas reales contra Codex CLI `0.169.0` y Mistral Vibe CLI `3.1.3` (`TestSyncNativeProbe` + `TestSyncNativeAcceptance`, `proxy_denials=0` antes y después de restaurar).

## Por qué

Completar la sincronización reproducible diferida desde Foundation 0.1 para compartir versiones exactas de skills y reglas entre Codex CLI y Mistral Vibe CLI sin copias manuales ni sobrescritura silenciosa de contenido ajeno.

## Decisión de aterrizaje

PR (opción 2 de `ship`), autorizado por el propietario: [PR #38](https://github.com/CodeMonkReaper/gandalf-wizard/pull/38) mergeado a `master` en `7316d4a` (2026-11-20). La épica `#28` y los nueve subissues `#29`–`#37` (`T001`–`T009`) se cerraron automáticamente con `Closes #28`–`#37`. Rama `feat/sync-skills-patrones` borrada en local y en remoto; worktree desregistrado.

## Evidencia

- Progreso y delegaciones Vibe: [progress/sync-skills-patrones.md](../../progress/sync-skills-patrones.md)
- Verificación de sistema de archivos (T002): [verifications/sync-skills-patrones-filesystem.md](../../verifications/sync-skills-patrones-filesystem.md)
- Verificación nativa real (Codex `0.169.0` + Vibe `3.1.3`): [verifications/sync-skills-patrones-native.md](../../verifications/sync-skills-patrones-native.md)
- Verificación de cierre (PASS, 15/15 paquetes + nativo): [verifications/sync-skills-patrones-closing.md](../../verifications/sync-skills-patrones-closing.md)
- Revisión de cierre (APPROVE en ronda 2/3): [reviews/sync-skills-patrones.md](../../reviews/sync-skills-patrones.md)
- Spec: [specs/sync-skills-patrones.md](../../specs/sync-skills-patrones.md) · Plan: [plans/sync-skills-patrones.md](../../plans/sync-skills-patrones.md) · Análisis: [analysis/sync-skills-patrones.md](../../analysis/sync-skills-patrones.md)
- Revisión de cierre: ronda 1 (`a164a7c`) detectó 6 hallazgos (`1 blocker`, `4 should-fix`, `1 nit`; 5/5 confirmados por `finding-verifier` independientes), corregidos en `46a9510` y aprobados en ronda 2 (`APPROVE`, 0 hallazgos).
