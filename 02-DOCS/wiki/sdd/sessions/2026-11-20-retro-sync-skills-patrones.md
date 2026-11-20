---
type: checkpoint
title: Retro post-merge — sync-skills-patrones, épica #28 cerrada
description: Cierre tras mergear PR #38 (7316d4a). Hallazgos de entorno, delegación Vibe y revisión de cierre.
tags: [sdd, checkpoint, retro, sync-skills-patrones]
timestamp: 2026-11-20T03:11:00Z
topic: sdd
---

# Checkpoint — Retro post-merge sync-skills-patrones (2026-11-20)

- Estado: `master` en `7316d4a` (merge del [PR #38](https://github.com/CodeMonkReaper/gandalf-wizard/pull/38)). Épica [#28](https://github.com/CodeMonkReaper/gandalf-wizard/issues/28) y subissues `#29`–`#37` (`T001`–`T009`) cerrados automáticamente en GitHub; rama `feat/sync-skills-patrones` borrada en local y remoto; worktree desregistrado.
- Bundle de archivo: [archive/sync-skills-patrones/final-report.md](../archive/sync-skills-patrones/final-report.md). Spec marcada `status: shipped`.

## Hallazgos del ciclo que conviene recordar

1. **Valor de la revisión de cierre de rama completa + `finding-verifier`:** las revisiones por tarea (`T001`–`T008`) pasaron en verde, pero la revisión adversarial de rama completa en contexto fresco (`Gemini 4 Neon`) detectó 5 defectos reales de integración entre paquetes (`L28`–`L32`), incluido un `blocker` de pérdida de datos si `Apply` cae entre `unlinkHandle` y `renameHandle` y `Restore` omitía `EntryPending` y los `.gandalf-sync-<op>-*.tmp`.
2. **Delegación a Vibe con presupuesto de 6 turnos (`rsc-vibe-bridge`):** despachar 7 archivos en una sola tarea agotó los 6 turnos leyendo archivos (`stopReason: limit`). Dividir en subtareas focalizadas de 3–4 archivos con lectura y edición en paralelo permitió completar cada despacho en 20–33 s con `exitCode: 0`.
3. **Windows `MAX_PATH` con Vibe `3.1.3`:** usar `t.TempDir()` para `home` en pruebas nativas generó rutas de chunks de sesión > 260 caracteres en Windows; `os.MkdirTemp("", "g")` con `t.Cleanup` en `capture_test.go` resolvió el fallo de forma estable.
4. **Comentarios de GitHub desde PowerShell:** usar here-strings con comillas dobles (`@"..."@`) corrompe los backticks de Markdown; usar siempre here-strings con comillas simples (`@'...'@`) enviados por pipe a `gh api -F body=@-`.
5. **Hook `pre-push` en worktrees enlazados:** al ejecutar `git push` desde un worktree enlazado, `GIT_DIR` puede afectar a `TestRepoSecurityGate` dentro del hook; ejecutar `git push` desde la raíz del repositorio principal mantiene el hook `pre-push` activo y en verde sin omitirlo.

## Siguiente acción

Definir y priorizar la siguiente épica del roadmap de Gandalf Wizard (con Foundation 0.1, CLI Gandalf, Router 0.2 y `sync-skills-patrones` ya integrados en `master`).

- Antes de relevar: este checkpoint queda enlazado desde [wiki/index.md](../../index.md).
