---
type: archive
title: Archivo — cli-gandalf — Foundation 0.1 cerrada
description: Bundle final de la épica #5: qué se entregó, decisión de aterrizaje y enlaces de evidencia.
tags: [sdd, archive, foundation, cli]
timestamp: 2026-11-18T12:40:00Z
topic: sdd
slug: cli-gandalf
---

# Archivo — cli-gandalf (2026-11-18)

## Qué se entregó

CLI `gandalf` 0.1.0 en Go (solo stdlib), épica [#5](https://github.com/CodeMonkReaper/gandalf-wizard/issues/5):

- `internal/config` — configuración en 5 capas (flag → entorno → proyecto → global → predeterminado), descubrimiento ascendente de `.gandalf.json`, rechazo de claves de credencial, `config init` exclusivo y validado.
- `internal/render` — tabla Markdown y JSON estable `schema_version: "1"` con etiquetas canónicas en español.
- `internal/harness` — detector solo lectura de harnesses (codex, vibe, antigravity) y `.rsc.json`.
- `internal/romero` — adaptador REST de lectura (`Search`, `GetDetail`) y sonda `CheckStatus` sobre el contrato `pkg/romeroclient` rev `3f46f9a`.
- `internal/doctor` — 6 diagnósticos sin cortocircuito con remedios.
- `internal/cli` + `cmd/gandalf` — despachador `Run` inyectable, códigos 0-7, credencial solo por `ROMERO_API_KEY` nunca en salida.

## Por qué

Cierre de Foundation 0.1: vía determinista para que agentes y personas consulten la memoria de proyecto en ROMERO sin filtrar credenciales y con salidas estables para consumo automático.

## Decisión de aterrizaje

PR (opción 2 de `ship`), autorizado por el propietario: [PR #14](https://github.com/CodeMonkReaper/gandalf-wizard/pull/14) mergeado a `master` en `c81a536` (2026-11-18). Sub-issues #6-#13 cerrados vía `Closes #N`; épica #5 cerrada manualmente tras el merge. Rama borrada en remoto y local.

## Evidencia

- Progreso y cola de decisiones: [progress/cli-gandalf.md](../../progress/cli-gandalf.md)
- Verificación de cierre (PASS): [verifications/2026-11-18-cli-gandalf-closing.md](../../verifications/2026-11-18-cli-gandalf-closing.md)
- Revisión de cierre (APPROVE): [reviews/cli-gandalf.md](../../reviews/cli-gandalf.md)
- Spec: [specs/cli-gandalf.md](../../specs/cli-gandalf.md) · Plan: [plans/cli-gandalf.md](../../plans/cli-gandalf.md) · Análisis: [analysis/cli-gandalf.md](../../analysis/cli-gandalf.md)
- Ledger de revisión del ciclo: 10 hallazgos, 0 abiertos (5 fixed, 5 intentional), 8 tareas, media 1.00 ronda.
