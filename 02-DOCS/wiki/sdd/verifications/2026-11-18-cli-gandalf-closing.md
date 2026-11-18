---
type: verification
title: Verification — cli-gandalf — 2026-11-18
description: Evidence-backed verdict for cli-gandalf — stack gate, task done-checks, acceptance criteria.
tags: [sdd, verification]
timestamp: 2026-11-18T00:45:00Z
topic: sdd
slug: cli-gandalf
source_state: 8990035 (clean)
---

# Verification — cli-gandalf — 2026-11-18

Cierre de la épica [#5](https://github.com/CodeMonkReaper/gandalf-wizard/issues/5) en `feat/cli-gandalf` (17 commits sobre `master`, sin divergencia). Gate genérico de `sdd/config.yaml` (no existe `scripts/verify.sh` del stack Go en este repositorio).

## Stack gate (ejecutado fresco, post-último-commit)

- `gofmt -l .` → PASS (salida vacía, exit 0)
- `go vet ./...` → PASS (sin hallazgos, exit 0)
- `go test -count=1 -cover ./...` → PASS: 6 paquetes ok (`cli` 85.3%, `config` 88.0%, `doctor` 97.9%, `harness` 100%, `render` 93.9%, `romero` 91.8%; `cmd/gandalf` sin archivos de test)
- `go build ./...` → PASS (binario `cmd/gandalf` compila)

## Task done-checks (ejecutados literales)

- [x] T001 `go test -v ./internal/...` → 6 paquetes ok
- [x] T002 `go test -v ./internal/config -run TestLoadConfig` → ok (1.96s)
- [x] T003 `go test -v ./internal/config -run TestInitProjectConfig` → ok (1.91s)
- [x] T004 `go test -v ./internal/render -run TestRender` → ok (2.42s)
- [x] T005 `go test -v ./internal/harness -run TestDetectHarnesses` → ok (1.40s)
- [x] T006 `go test -v ./internal/romero -run TestRomeroStatusClient` → ok (4.30s)
- [x] T007 `go test -v ./internal/doctor -run TestDoctorChecks` → ok (2.02s)
- [x] T008 `go test -v ./internal/cli -run TestCLIDispatch` → ok (4.21s)
- [x] T009 `go test -v ./internal/cli -run TestAcceptance_AC1_to_AC13` → ok (5.39s)
- [x] T010 `gofmt -l .` vacío AND `go vet ./...` AND `go test ./...` → los tres en verde (arriba)

## Acceptance criteria (spec)

Evidencia observable: subtests de `TestAcceptance_AC1_to_AC13` en `internal/cli/acceptance_test.go`, todos PASS en la corrida fresca:

- [x] AC1 enrutamiento/ayuda/0-2 — PASS `AC1_enrutamiento,_ayuda_y_códigos_0/2`
- [x] AC2 precedencia flag→entorno→proyecto→global→predeterminado — PASS (4 subtests + casos sin proyecto ni global)
- [x] AC3 centinela ausente de toda salida — PASS (13 ejecuciones éxito/error incluida denegación 401; `config show` informa «presente»)
- [x] AC4 config init exclusivo y validado — PASS (0/3/3, archivo sin modificar, id inválido no crea nada)
- [x] AC5 config inválida → 3, cero llamadas ROMERO — PASS (4 casos + aviso de clave desconocida sin bloqueo)
- [x] AC6 doctor 6×OK → 0 — PASS (`strings.Count(out, "[OK]") == 6`, `fallos=0 avisos=0`)
- [x] AC7 doctor con fallos → 7, los 6 checks con remedio — PASS
- [x] AC8 doctor detecta harnesses sin escribir nada — PASS (snapshot del árbol idéntico antes/después)
- [x] AC9 tabla referencia 1 byte a byte — PASS (comparación exacta de cadena)
- [x] AC10 JSON referencia 2, `entries` siempre lista — PASS (payload decodificado campo a campo; vacío serializa `[]`)
- [x] AC11 estados con etiquetas y códigos aprobados — PASS (ok/empty vía httptest; partial/incomplete vía `RomeroFakeAdapter`; access_denied vía 401; scope_required vía `Present()` — la CLI no puede producirlo porque el guard de proyecto va antes, cubierto por AC12)
- [x] AC12 sin proyecto: 0 peticiones, salida 3 — PASS (contador de peticiones del servidor simulado)
- [x] AC13 timeout 1s < 3s con código 6 — PASS (1.00s medido, «Consulta incompleta»)

## Rollback plan

- Pre-merge: la rama no está en `master`; revertir es no fusionar (o `git branch -D feat/cli-gandalf` si se descarta).
- Post-merge: `git revert <sha-de-merge>`; sin migraciones ni flags; el cambio es aditivo (paquetes nuevos `internal/*` y binario `cmd/gandalf`), nada de `master` previo depende de él. Los `.gandalf.json` creados por usuarios son datos suyos y no los toca el revert.

## Capas no ejecutadas

- **SUSTITUIDA** — cobertura: `go test -cover` corre sin umbral (GAP). Un reporte, no un gate; no detecta regresión de cobertura. La constitución difiere los umbrales «al habilitar remoto». Números frescos arriba por paquete.
- **HERRAMIENTA-AUSENTE** — staticcheck/golangci-lint/govulncheck: no instalados; el gate constituido (gofmt/vet/test) no los exige y nada corrió en su lugar.
- **HERRAMIENTA-AUSENTE** — `go test -race`: no ejecutado (el gate constituido no lo incluye); la CLI es secuencial y no lanza goroutines propias.
- **HERRAMIENTA-AUSENTE** — auditoría de seguridad dedicada (`security-audit`): no ejecutada como pase completo; la superficie de credenciales quedó cubierta por AC3 (centinela en 13 salidas) y los controles por tarea, pero no hubo escaneo de vulnerabilidades de dependencias (solo stdlib, `go.mod` sin dependencias externas).

## Hallazgos descartados

- Ledger de revisión: 10 hallazgos únicos, 0 abiertos — 5 fixed (L1, L5, L6, L8, L9), 5 intentional con racional registrado (L2, L3, L4, L7, L10); 8 tareas revisadas, media 1.00 ronda. Falsos positivos: 0.
- «el handler bloqueado puede colgar `server.Close`» — resuelto: canal `release` cerrado en cleanup LIFO antes de `Close` (T009, decisión en progreso).

## Verdict: PASS

Gate en verde, 10/10 done-checks con evidencia, 13/13 ACs con prueba observable, rollback escrito. Cola de decisiones Q1-Q4 (review-after) viaja al humano con el cierre; no bloquean. Entrada a `review` de cierre de rama.
