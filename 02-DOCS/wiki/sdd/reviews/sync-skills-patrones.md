---
type: review
title: Revisión de cierre ASDD — sync-skills-patrones
description: Revisión adversarial de rama completa contra especificación, plan, constitución y REVIEW.md.
tags: [sdd, review, sync-skills-patrones]
timestamp: 2026-11-19T21:45:00Z
topic: sdd
slug: sync-skills-patrones
issue: https://github.com/CodeMonkReaper/gandalf-wizard/issues/28
---

# Revisión de cierre — `sync-skills-patrones`

## Ronda 1 (`2026-11-19`)

- **Rama y commit**: `feat/sync-skills-patrones` en `a164a7cfcddbd0c1a228035bed055f7d7e071ef5` (diff contra `master`, 36 archivos).
- **Nivel de riesgo (`rsc risk --base master`)**: `tier 1` (`classes: default, docs`, `reviewers: 1`, sello desactivado).
- **Modelo de revisión y verificación**: Gemini 4 Neon (High) en contexto fresco independiente (1 revisor adversarial + 5 `finding-verifier` independientes en paralelo).
- **Conteo consolidado**: **6 hallazgos reportados → 6 únicos → 5 enviados a `finding-verifier` → 5 `CONFIRMED`, 0 `REFUTED`, 0 `UNPROVEN`** (`1 blocker`, `4 should-fix`, `1 nit`).
- **Veredicto**: **`CHANGES REQUESTED`**.

### Lista de ataque verificada (pases 1–6)

- **AC-01 (Catálogo y selección)**: verificados los 3 tipos, inmutabilidad, origen, re-registro idempotente, conflicto por contenido divergente, bytes fijados tras mutar origen, dependencias y detección de manipulación. Hallazgo en acceso privado/Git (`L31`).
- **AC-02 (Vista previa de solo lectura)**: verificadas cero mutaciones, `View.ID` determinista y selección vacía. Limpio.
- **AC-03 (Reproducción y descubrimiento nativo)**: verificada convergencia byte a byte, patrones como referencia y aceptación nativa Codex/Vibe. Hallazgo en versión fija de Vibe en `TestSyncNativeProbe` (`L32`).
- **AC-04 (Autorización y repetición)**: verificado enlace de `View.ID`, preservación de `AGENTS.md` ajeno y repetición `noop`. Limpio.
- **AC-05 (Conflictos, compatibilidad, confinamiento y exclusión)**: verificados NTFS, anclaje de raíz/padre, rechazo de reparse/hardlink/ADS/DOS/UTF-16, mutex nominal y diagnósticos. Limpio.
- **AC-06 y AC-07 (Respaldo, interrupción y recuperación)**: detectados 3 defectos reales en la integración `syncfs` → `projectsync` → `cli` (`L28`, `L29`, `L30`).
- **AC-08 (Privacidad del informe y acceso restringido)**: verificada ausencia de contenido de archivos y secretos en salidas/errores. Hallazgo en protección de `.gandalf/sync` al registrar en catálogo (`L31`).
- **AC-09 (Compatibilidad CLI)**: verificado que `doctor`, `config`, `memory`, `models`, `route`, `run`, códigos `0–10` y `TestRepoSecurityGate` se conservan intactos. Limpio.
- **Seguridad y constitución**: sin código fuente de RSC, sin credenciales, reglas estrictamente consultivas, `RestrictDir` sin TOCTOU. Observación documental menor (`L33`).

### Hallazgos consolidados (ledger `sync-skills-patrones/closing`, ronda 1/3)

| ID ledger | Severidad | Verificador | Archivo y línea | Criterio | Estado | Resumen y corrección requerida |
| --- | --- | --- | --- | --- | --- | --- |
| `L28` | `blocker` | `CONFIRMED` | `internal/projectsync/restore.go:256` + `internal/syncfs/filesystem_windows.go:238-279` | `AC-07` (`AC-06`, Q2) | Pendiente | Si `Apply` cae entre `unlinkHandle` y `renameHandle`, `AGENTS.md` queda ausente y `.gandalf-sync-<opID>-*.tmp` queda en disco, pero `planRestore` salta `EntryPending` cuando `!matchesAfterState`, marca `PhaseRestored`, borra `pending.json` y pierde el archivo preexistente además de dejar el `.tmp` huérfano. **Fix**: exponer inspección/limpieza confinada de `.gandalf-sync-<op>-*.tmp` en `syncfs.ScopedProject`; en `planRestore` restaurar desde `BackupPath` cuando `BeforeExists && !cur.Exists` y exista `.tmp` atribuido en el directorio padre; limpiar `.tmp` atribuidos en `Restore`; añadir regresión. |
| `L29` | `should-fix` | `CONFIRMED` | `internal/cli/sync.go:378,418,477-495,94-103` | `AC-07` | Pendiente | `runSyncApply`/`runSyncRestore` ignoran `result.Status`/`result.Reason` en errores y `failSync` los sobrescribe a `"rejected"` / `err.Error()`; en texto plano `fail()` omite `operation_id` y `diagnostics`. **Fix**: preservar `result.Status`/`result.Reason` cuando estén presentes y mostrar `operation_id`, cambios aplicados y diagnósticos en modo texto. |
| `L30` | `should-fix` | `CONFIRMED` | `internal/projectsync/apply.go:485,494` | `AC-07` | Pendiente | `Apply` preconstruye `summaries` con todos los cambios planeados y los devuelve íntegros en `recoveryResult` ante fallo/interrupción, reportando archivos no escritos como aplicados. **Fix**: acumular `appliedSummaries` solo tras cada `project.Change` exitoso y añadir aserciones en `TestSyncApply`. |
| `L31` | `should-fix` | `CONFIRMED` | `internal/synccatalog/catalog.go:388` | `AC-08` | Pendiente | `Catalog.Register` crea `.gandalf/sync/catalog` sin llamar a `RestrictDir` ni crear `.gandalf/sync/.gitignore` (solo `Apply` lo hace y solo si `len(view.Changes) > 0`). **Fix**: aplicar `RestrictDir` y `.gitignore` al registrar en catálogo y cubrirlo en `TestSyncCatalog`. |
| `L32` | `should-fix` | `CONFIRMED` | `internal/syncnative/probe_test.go:44` | `AC-03` | Pendiente | `TestSyncNativeProbe` fija Vibe `"3.1.2"` en vez de usar `installedVibeVersion(t)` (`"3.1.3"`), rompiendo la verificación nativa de precedencia en el entorno verificado. **Fix**: usar `installedVibeVersion(t)` en `probe_test.go`, alinear perfiles/evidencia nativa con Vibe `3.1.3` y re-ejecutar `GANDALF_SYNC_NATIVE_TEST=1 go test -count=1 -v ./internal/syncnative`. |
| `L33` | `nit` | N/A | `02-DOCS/wiki/sdd/verifications/sync-skills-patrones-filesystem.md:12` | — | Pendiente | Redacción obsoleta («T002 bloqueada para integración») y enlaces de verificaciones faltantes en `02-DOCS/wiki/index.md`. **Fix**: actualizar redacción y enlazar verificaciones y esta revisión desde el índice. |
