---
type: checkpoint
title: Retro post-merge — Foundation 0.1 shipped
description: Cierre de sesión tras mergear PR #4 y PR #3 a master. Hallazgos que mejoran el entorno.
tags: [sdd, checkpoint, retro, foundation]
timestamp: 2026-11-18T01:35:00Z
topic: sdd
---

# Checkpoint — Retro post-merge (2026-11-18)

- Estado: `master` en `30bd22c`. PR #4 (harness, Issue #1) y PR #3 (Foundation 0.1, Issue #2) mergeados con merge commit; ramas borradas en remoto; issues #1 y #2 cerrados.
- Gate en master: `go vet ./...` PASS, `go test ./...` 16/16 PASS, cobertura 90.8%. `gofmt -l` reporta archivos por CRLF/LF en Windows (falso positivo de line-endings; el gate en el worktree con `gofmt -w` previo estaba limpio).
- Integración viva: ROMERO rev `3f46f9a` corriendo local (perfil lite, solo `romero-postgres` en Docker, API como proceso Go en `127.0.0.1:8420`). Evidencia en [verifications/2026-11-18-romero-live-integration.json](../verifications/2026-11-18-romero-live-integration.json).

## Hallazgos que cambian el entorno (no el código)

1. **Contrato ROMERO real:** el scope no es `project` sino `project_shared` (válido: agent_private, task_shared, project_shared, org_shared, global_public). El plan lo corregía en `rest_client.go`; queda registrado como desviación aprobada en la evidencia de integración.
2. **Seam T006/T007:** los errores de transporte en `GetDetail` deben envolver como `ErrDetailInterrupted` (no `ErrServiceInterrupted`), o el orchestrator hace `continue` en vez de `break` y pierde el corte limpio de "Consulta parcial".
3. **Tests vacuosos:** un subtest puede pasar sin ejercitar la rama que afirma (T005 finding 1). El review con mutation testing lo detectó. Recomendación: en reviews tier-1 de seguridad, mutar la guarda y exigir que el test falle.
4. **Audit trail en denial:** devolver `AuditTrail` en un path de `ErrAccessDenied` filtra el conteo de hits. Regla: en outcomes de denial, cero entries y cero audit trail.
5. **Docker ROMERO:** solo `romero-postgres` es obligatorio; embeddings/reranker/NLI/SLM son opcionales (profiles balanced/ultra, requieren GPU). La API corre con `go run ./cmd/romero serve` en perfil lite.
6. **Orden de PRs:** mergear el PR base (harness) antes que el de feature para que el diff del feature quede limpio.
7. **write_file en Windows:** rutas `/c/...` pueden escribir en `C:\c\...`. Patrón seguro: escribir en scratchpad y copiar con `cp` vía bash.

## Siguiente acción

Contexto maestro (`gandalf-wizard-contexto-maestro.md`) tiene propuestas pendientes de spec. Abrir nuevo issue y empezar con grilling + specify cuando se retome.

- Antes de relevar: este checkpoint queda enlazado desde [wiki/index.md](../../index.md).
