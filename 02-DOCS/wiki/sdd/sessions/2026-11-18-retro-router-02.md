---
type: checkpoint
title: Retro post-merge — router-02, épica #15 cerrada y apertura de #28
description: Cierre tras mergear PR #27 (abf5901). Hallazgos de entorno y proceso del ciclo router-02.
tags: [sdd, checkpoint, retro, router-02]
timestamp: 2026-11-18T16:33:00Z
topic: sdd
---

# Checkpoint — Retro post-merge router-02 (2026-11-18)

- Estado: `master` en `abf5901` (merge del [PR #27](https://github.com/CodeMonkReaper/gandalf-wizard/pull/27)). Épica [#15](https://github.com/CodeMonkReaper/gandalf-wizard/issues/15) y sub-issues #17–#26 cerrados; ramas `feat/router-02` y `docs/diferir-sync-y-epica-15` borradas en remoto y local.
- Bundle de archivo: [archive/router-02/](../archive/router-02/) (`final-report`, `state.yaml`, punteros de progreso/verificación/revisión). Spec marcada `status: shipped`.
- Nueva épica abierta: [#28](https://github.com/CodeMonkReaper/gandalf-wizard/issues/28) (`epic(foundation): sincronización reproducible de skills y patrones entre harnesses (sync-skills-patrones)`).

## Hallazgos del ciclo que conviene recordar

1. **Vibe `VerifyTextOnly` y herencia de plazo (`--time-limit`):** en `--agent ask --max-turns 1`, el intento bloqueado consume hasta el `--time-limit` que recibe del contexto. Pasar el contexto padre al Paso 1 agotaba el margen total antes de llegar al Paso 2; acotar el Paso 1 con un subcontexto (`20s`) resuelve la sonda en ~39 s.
2. **Puentes inyectables vs cableado de producción:** `routerRegistryFor` construye adaptadores con defaults de producción (`direct.Adapter{Getenv: deps.getenv}` y `TextOnlyEnforced` una vez demostrada la sonda viva). Conviene probar explícitamente al menos una corrida con `RouterRegistry == nil` antes de cerrar la integración.
3. **Recorrido de claves en arreglos JSON (`walkKeys`):** `case []any:` pasaba `nil` en lugar de `childKnown`, dejando muertos `executorKnownKeys` y `profileKnownKeys`; las validaciones de esquema recursivas necesitan pruebas de aviso sobre arreglos además de objetos.
4. **Higiene de claves y falsos positivos de carpeta:** `.vibe/hooks.toml` es archivo rastreado del entorno RSC multi-target, mientras `.vibe/config.toml`, `.vibe/sessions/`, `01-TOOLS/vibe-bridge/` y archivos `*whoami_cache*.json`/`*google_accounts*.json` son locales. `TestRepoSecurityGate` y `.git/hooks/pre-commit`/`pre-push` quedan probados con casos reales de paso y bloqueo.
5. **Revisión de cierre con contexto fresco (`research` subagent):** la ronda 1 en contexto frío detectó 4 defectos reales (`Probe` ejecutando `--version` en no autorizados, `walkKeys`, `IsForbiddenCredentialKey` literal, `AdapterFor` con `nil`) que habían pasado los tests unitarios por tarea.

## Siguiente acción

Iniciar la etapa `specify` para [sync-skills-patrones (Épica #28)](https://github.com/CodeMonkReaper/gandalf-wizard/issues/28): definir la frontera con RSC (constitución P10), los harnesses objetivo y el contrato de sincronización reproducible de skills, reglas y patrones. Las suposiciones `Q1`–`Q8` de `router-02` quedan en [progress/router-02.md](../progress/router-02.md) para consideración del propietario.

- Antes de relevar: este checkpoint queda enlazado desde [wiki/index.md](../../index.md).
