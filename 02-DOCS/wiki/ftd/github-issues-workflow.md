---
author: Luis Haroldo Salamanca Fernandéz
branch: chore/github-issues-workflow
status: in-progress
---
# Flujo, etiquetas y plantillas de GitHub Issues con cierre técnico obligatorio

Issue vinculado: [#41](https://github.com/CodeMonkReaper/gandalf-wizard/issues/41) · Guía operativa: [github-issues-workflow.md](../harness/github-issues-workflow.md)

## Intent

Estandarizar la taxonomía de etiquetas, las plantillas de Issues y Pull Requests en `.github/` y la **Puerta de Cierre Técnico obligatoria** antes de pasar de un Issue al siguiente, incorporando revisión técnica detallada y registro explícito de errores del desarrollo y soluciones aplicadas.

## Scope

- **Incluido:** creación de las 15 etiquetas en GitHub (`type:*`, `lane:*`, `tier:*`, `status:*`), etiquetado inicial de la épica [#40](https://github.com/CodeMonkReaper/gandalf-wizard/issues/40), apertura del Issue [#41](https://github.com/CodeMonkReaper/gandalf-wizard/issues/41), plantillas `.github/ISSUE_TEMPLATE/*.yml` + `.github/PULL_REQUEST_TEMPLATE.md`, guía en [02-DOCS/wiki/harness/github-issues-workflow.md](../harness/github-issues-workflow.md), enlace en [02-DOCS/wiki/index.md](../index.md) y registro en [02-DOCS/wiki/sdd/decisions.md](../sdd/decisions.md).
- **Fuera de alcance:** GitHub Actions / CI workflows, edición retroactiva de Issues cerrados (`#1`–`#39`), cambios en código fuente Go, creación de la rama `feat/workflow-runtime-03` o `push`/PR sin autorización humana explícita.

## Checklist

- [x] Etiquetas (`type:*`, `lane:*`, `tier:*`, `status:*`) creadas en GitHub, [#40](https://github.com/CodeMonkReaper/gandalf-wizard/issues/40) etiquetada e Issue FTD [#41](https://github.com/CodeMonkReaper/gandalf-wizard/issues/41) abierto — *criterio:* salida de `gh label list` e inspección de `#40`/`#41`.
- [x] Plantillas `.github/ISSUE_TEMPLATE/` (`01-epic-asdd.yml`, `02-task-asdd.yml`, `03-ftd-change.yml`, `04-bug-debug.yml`, `config.yml`) y `.github/PULL_REQUEST_TEMPLATE.md` creadas y validadas — *criterio:* parseo con `yaml.safe_load`, validación estructural y coincidencia 1:1 con `gh label list`.
- [x] Guía operativa en `02-DOCS/wiki/harness/github-issues-workflow.md` redactada, enlazada en `02-DOCS/wiki/index.md` y regla registrada en `02-DOCS/wiki/sdd/decisions.md` — *criterio:* enlaces relativos verificados (`LINKS_OK`) y `git diff --check` limpio.
- [x] Riesgo clasificado (`rsc risk`), revisión fresca de contexto frío ejecutada (`tier 1`) y hallazgos atendidos — *criterio:* salida de `rsc risk` (`tier 1`) y dictamen `APPROVE` del revisor fresco con los 3 `nit` corregidos.
- [x] Controles Go obligatorios (`go vet ./...`, `go test -count=1 ./...` incluyendo `TestRepoSecurityGate`) ejecutados en verde tras la última edición de plantillas/guía — *criterio:* 15/15 paquetes Go en `ok` (ver nota sobre `gofmt -l .` y `core.autocrlf=true` en `Evidence`).
- [ ] Commit local en `chore/github-issues-workflow`, re-verificación de `TestRepoSecurityGate` sobre `origin/master..HEAD`, publicación del Informe de Cierre Técnico en [#41](https://github.com/CodeMonkReaper/gandalf-wizard/issues/41) y actualización a `status:in-review`.

## Evidence

- **Etiquetas e Issues en GitHub (`gh`):**
  - `gh label list --repo CodeMonkReaper/gandalf-wizard --limit 50` confirma las 15 etiquetas (`type:epic`, `type:task`, `type:ftd`, `type:bug`, `type:docs`, `lane:asdd`, `lane:ftd`, `tier:0`, `tier:1`, `tier:2`, `status:spec`, `status:ready`, `status:in-progress`, `status:blocked`, `status:in-review`).
  - `gh issue view 40 --json number,labels` confirma `#40` con `type:epic`, `lane:asdd`, `status:spec`.
  - `gh issue view 41 --json number,labels` confirma `#41` con `type:ftd`, `lane:ftd`, `tier:1`, `status:in-progress`.
- **Validación YAML, enlaces e higiene Git:**
  - Script Python (`yaml.safe_load`) sobre `.github/ISSUE_TEMPLATE/*.yml`: `YAML_OK` en `01-epic-asdd.yml` (9 elementos), `02-task-asdd.yml` (8 elementos), `03-ftd-change.yml` (6 elementos), `04-bug-debug.yml` (6 elementos) y `config.yml`, comprobando IDs únicos, `checkboxes` sin bloqueo de creación y coincidencia 1:1 de etiquetas contra GitHub.
  - Verificación de enlaces Markdown relativos (`LINKS_OK`) en `02-DOCS/wiki/harness/github-issues-workflow.md`, `02-DOCS/wiki/ftd/github-issues-workflow.md`, `02-DOCS/wiki/index.md` y `02-DOCS/wiki/sdd/decisions.md`.
  - `git diff --check`: limpio (`DIFF_CHECK_OK`).
- **Clasificación de riesgo y revisión fresca (`tier 1`):**
  - `npm exec --yes --package=github:CodeMonkReaper/rsc-harness-vibe#2b37e98870eaaca9980bf903381d5ae5aacd5280 -- rsc risk --base origin/master` → `tier 1` (`classes: default, docs`, `reviewers: 1`, `human approval: not required`).
  - Revisión fresca en contexto frío (`research` subagent `0ea24462-65f6-4a05-b3c2-25f3b3bd98ae`) sobre `REVIEW.md`, `constitution.md`, `decisions.md` y los 10 archivos: **3 hallazgos (`0 blocker`, `0 should-fix`, `3 nit`), veredicto `APPROVE`**. Los 3 `nit` se corrigieron antes del commit (condicionalidad de revisión fresca en `03-ftd-change.yml:54`, apertura de `type:docs` y nota de límite consultivo P6 en `harness/github-issues-workflow.md`). Límite reportado por el sandbox del revisor: acceso remoto a `gh`/TLS bloqueado dentro del subagente, verificado directamente por el coordinador arriba.
- **Controles Go tras las correcciones `nit`:**
  - `gofmt -l .` listó 63 archivos `.go` preexistentes únicamente por saltos CRLF de `core.autocrlf=true` en Windows (`gofmt -d` confirmó solo cambios de fin de línea en archivos no tocados por esta rama).
  - `go vet ./...` → `GOVET_OK`.
  - `go test -count=1 ./...` → `ok` en los 15 paquetes (`cmd/gandalf`, `internal/cli` incluido `TestRepoSecurityGate`, `internal/config`, `internal/doctor`, `internal/harness`, `internal/projectsync`, `internal/render`, `internal/romero`, `internal/router`, `internal/router/antigravity`, `internal/router/direct`, `internal/router/vibe`, `internal/synccatalog`, `internal/syncfs`, `internal/syncnative`).

## Next

Ejecutar `git commit` local en `chore/github-issues-workflow`, validar `TestRepoSecurityGate` sobre `origin/master..HEAD`, publicar el Informe de Cierre Técnico en [#41](https://github.com/CodeMonkReaper/gandalf-wizard/issues/41) con etiqueta `status:in-review` y solicitar autorización humana para `git push` y apertura del Pull Request hacia `master` (`Closes #41`).
