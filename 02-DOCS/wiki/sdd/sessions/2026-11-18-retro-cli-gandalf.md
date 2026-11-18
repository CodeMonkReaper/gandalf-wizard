---
type: checkpoint
title: Retro post-merge — cli-gandalf, Foundation 0.1 cerrada
description: Cierre tras mergear PR #14. Hallazgos de entorno del ciclo cli-gandalf.
tags: [sdd, checkpoint, retro, cli-gandalf, foundation]
timestamp: 2026-11-18T12:45:00Z
topic: sdd
---

# Checkpoint — Retro post-merge cli-gandalf (2026-11-18)

- Estado: `master` en `c81a536` (merge del [PR #14](https://github.com/CodeMonkReaper/gandalf-wizard/pull/14)). Épica [#5](https://github.com/CodeMonkReaper/gandalf-wizard/issues/5) y sub-issues #6-#13 cerrados; rama `feat/cli-gandalf` borrada en remoto y local.
- Gate en master: `go vet ./...` PASS, `go test ./...` 6 paquetes ok. `gofmt -l` sigue reportando falso positivo CRLF/LF en Windows (confirmado: el diff es solo conversión de finales de línea; los bytes mergeados estaban limpios).
- Bundle de archivo: [archive/cli-gandalf/](../archive/cli-gandalf/) (final-report, state.yaml, punteros de progreso/verificación/revisión). Spec marcada `status: shipped`.

## Hallazgos que cambian el entorno (no el código)

1. **Handlers bloqueados en tests:** la cancelación del cliente HTTP no siempre propaga hasta `r.Context()` en el lado servidor y `httptest.Server.Close` colgaba el cleanup. Patrón: bloquear en `select` sobre un canal `release` cerrado por `t.Cleanup` registrado DESPUÉS de `server.Close` (LIFO: corre antes).
2. **Palabras clave de cierre en el PR:** mencionar la épica en prosa («#5 se cierra con esta épica») no cierra el issue; GitHub exige `Closes #N`/`Fixes #N` en el cuerpo. La épica quedó abierta tras el merge y hubo que cerrarla a mano. Regla: el PR que cierra una épica lleva `Closes #<épica>` explícito.
3. **write_file a /tmp en Windows:** la herramienta falla con `WinError 5` (resuelve `C:\tmp`). Patrón seguro ya conocido: heredoc vía bash para ficheros temporales.
4. **CLI del ledger:** el orden es `rsc review ledger record <findings.json> --task <id>`; la forma `--task <id> --from <file>` imprime ayuda y no registra.
5. **Suite de aceptación como evidencia de cierre:** probar los 13 ACs en proceso con `httptest` + centinela de fuga + snapshot de árbol resultó barata y cerró la verificación sin humo manual; replicar el patrón en futuras épicas con ACs observables.

## Siguiente acción

Foundation 0.1 está cerrada con el alcance entregado. La sincronización reproducible de skills y patrones (prevista en 0.1 por el roadmap orientativo) queda **diferida a la siguiente etapa** por decisión del propietario ([decisiones](../decisions.md)); incluirla al priorizar el próximo `specify`. El contexto maestro (`gandalf-wizard-contexto-maestro.md`) tiene propuestas pendientes de spec; al retomar, abrir issue nuevo y empezar con specify. Q1-Q4 (review-after) quedaron listados en el PR #14 para consideración del propietario.

- Antes de relevar: este checkpoint queda enlazado desde [wiki/index.md](../index.md).
