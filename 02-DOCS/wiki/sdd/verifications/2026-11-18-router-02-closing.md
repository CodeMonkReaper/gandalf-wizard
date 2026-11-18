---
type: verification
title: Verification — router-02 — 2026-11-18
description: Veredicto con evidencia para router-02 (épica #15) — gate Go, done-checks T001–T010, criterios AC1–AC19, evidencia viva e higiene de secretos.
tags: [sdd, verification, router]
timestamp: 2026-11-18T16:19:00Z
topic: sdd
slug: router-02
source_state: cef75ff (clean)
---

# Verification — router-02 — 2026-11-18

Cierre de la primera especificación (`router-02`) de la épica [#15](https://github.com/CodeMonkReaper/gandalf-wizard/issues/15) en `feat/router-02` (25 commits sobre `master`, 0 detrás, 42 archivos modificados en `master...HEAD`, `8960` inserciones / `21` borrados). Gate genérico de `02-DOCS/wiki/sdd/config.yaml` (no existe `scripts/verify.sh` en el repositorio).

## Stack gate (ejecutado fresco en `cef75ff`)

- `gofmt -l .` → PASS en todos los archivos de `router-02` (únicamente señala los 6 archivos CRLF preexistentes de Foundation: `internal/harness/*`, `internal/romero/status_*`)
- `go vet ./...` → PASS (salida vacía, exit 0)
- `go build ./...` → PASS (compila el binario `cmd/gandalf` y todos los paquetes)
- `go test -count=1 -cover ./...` → PASS: 11 paquetes ok (sin caché):
  - `cmd/gandalf`: `0.0%` (`TestRouterInterrupt` recompila y ejecuta el binario como subproceso externo)
  - `internal/cli`: `84.2%`
  - `internal/config`: `91.8%`
  - `internal/doctor`: `98.1%`
  - `internal/harness`: `100.0%`
  - `internal/render`: `38.9%`
  - `internal/romero`: `91.8%`
  - `internal/router`: `89.9%`
  - `internal/router/antigravity`: `84.7%`
  - `internal/router/direct`: `89.6%`
  - `internal/router/vibe`: `82.4%`

## Task done-checks (ejecutados literales)

- [x] T001 `go test ./internal/config -run TestRouterPolicy -count=1` → ok (1.714s en `cef75ff`)
- [x] T002 `go test ./internal/router -run TestCatalog -count=1` → ok (1.145s en `34c5237`, más `go test -count=1 ./internal/router` ok en `cef75ff`)
- [x] T003 `go test ./internal/router -run TestSelection -count=1` → ok (1.192s en `34c5237`, más `go test -count=1 ./internal/router` ok en `cef75ff`)
- [x] T004 `go test ./internal/router -run TestRun -count=1` → ok (1.265s en `34c5237`, más `go test -count=1 ./internal/router` ok en `cef75ff`)
- [x] T005 `go test ./internal/router/antigravity -run TestAdapter -count=1` → ok (3.277s en `cef75ff`)
- [x] T006 `go test ./internal/router/vibe -run TestAdapter -count=1` → ok (6.160s en `cef75ff`)
- [x] T007 `go test ./internal/router/direct -run TestAdapter -count=1` → ok (3.523s en `34c5237`, más `go test -count=1 ./internal/router/direct` ok en `cef75ff`)
- [x] T008 `go test ./internal/cli ./internal/doctor ./cmd/gandalf -run 'TestRouterCLI|TestRouterDoctor|TestRouterInterrupt' -count=1` → 3 paquetes ok (7.259s / 4.123s / 8.355s en `34c5237`, más `go test -count=1` de los 3 paquetes ok en `cef75ff`)
- [x] T009 `go test -v ./internal/cli -run TestRouterAcceptance -count=1` → PASS (0.31s / 4.276s total paquete en `cef75ff`; 19/19 subtests `AC1`–`AC19`)
- [x] T010 Verificación viva + higiene de secretos → [2026-11-18-router-02-live-verification.json](2026-11-18-router-02-live-verification.json) (sondas `enforced` en Antigravity y Vibe, 3 éxitos vivos en producción `antigravity`/`vibe`/`api-mistral`, fallback vivo en 2 intentos) y `go test -v ./internal/cli -run '^TestRepoSecurityGate$' -count=1` → PASS (0.19s / 3.995s en `cef75ff`)

## Acceptance criteria (spec)

Evidencia observable: subtests de `TestRouterAcceptance` en `internal/cli/router_acceptance_test.go`, más evidencia viva en [2026-11-18-router-02-live-verification.json](2026-11-18-router-02-live-verification.json):

- [x] AC1 catálogo con autorizados y no autorizados — PASS `AC1_catalogo_con_autorizados_y_no_autorizados` (más descubrimiento vivo `origin=descubierto` en `models list`)
- [x] AC2 binario y credencial ausentes — PASS `AC2_binario_y_credencial_ausentes` (motivos específicos sin enmascarar por versión)
- [x] AC3 determinismo de `route` — PASS `AC3_misma_entrada_produce_la_misma_salida_de_route`
- [x] AC4 explicación de criterio, alternativas y fallback — PASS `AC4_route_explica_criterio_alternativas_y_fallback`
- [x] AC5 no autorizado nunca elegido ni invocado — PASS `AC5_no_autorizado_nunca_elegido_ni_invocado` + pruebas unitarias `probe no autorizado no lanza version`
- [x] AC6 `run` ok con un intento — PASS `AC6_run_ok_con_un_solo_intento` + 3 éxitos vivos (`antigravity`, `vibe`, `api-mistral`)
- [x] AC7 fallback al segundo perfil — PASS `AC7_fallback_al_segundo_perfil` + fallback vivo en 2 intentos
- [x] AC8 agotamiento con código 9 — PASS `AC8_agotamiento_con_codigo_9`
- [x] AC9 sin elegibles código 8 sin invocar — PASS `AC9_sin_elegibles_codigo_8_sin_invocar`
- [x] AC10 manual inelegible sin sustitución — PASS `AC10_manual_inelegible_sin_sustitucion`
- [x] AC11 cancelación código 10 sin fallback — PASS `AC11_cancelacion_codigo_10_sin_fallback` + `TestRouterInterrupt` con `CTRL_BREAK_EVENT` en proceso real
- [x] AC12 esfuerzo omitido y explicado — PASS `AC12_esfuerzo_omitido_y_explicado`
- [x] AC13 centinela ausente en salidas y archivos — PASS `AC13_centinela_ausente_en_salidas_y_archivos` + `TestRepoSecurityGate` + verificación viva sin fugas
- [x] AC14 configuración con credencial da 3 — PASS `AC14_configuracion_con_credencial_da_3` + `TestRouterPolicy` (incluidas claves específicas como `mistral_api_key`)
- [x] AC15 `doctor` informa sin cuota ni escrituras — PASS `AC15_doctor_informa_sin_cuota_ni_escrituras`
- [x] AC16 proveedor compatible solo por configuración — PASS `AC16_proveedor_compatible_solo_por_configuracion`
- [x] AC17 Foundation y códigos 0–7 intactos — PASS `AC17_foundation_y_codigos_0-7_en_verde` + avisos de claves desconocidas en `executors`/`profiles` + suite `TestAcceptance`
- [x] AC18 timeout corta primer intento antes del segundo — PASS `AC18_timeout_corta_el_primer_intento_antes_del_segundo`
- [x] AC19 riesgo alto descarta no verificados — PASS `AC19_riesgo_alto_descarta_el_no_verificado`

## Rollback plan

- Pre-merge: la rama `feat/router-02` no está fusionada en `master`; revertir es no hacer PR/merge o descartar la rama (`git branch -D feat/router-02`).
- Post-merge: `git revert -m 1 <sha-de-merge>` del PR; sin migraciones ni cambios incompatibles sobre los comandos o códigos `0`–`7` de Foundation. Los archivos globales/locales del usuario no se alteran automáticamente.

## Capas no ejecutadas

- **SUSTITUIDA** — cobertura: `go test -count=1 -cover ./...` informa porcentajes sin umbral de fallo (`GAP` del runner Go; la constitución difiere umbrales al habilitar remoto).
- **HERRAMIENTA-AUSENTE** — `staticcheck`, `golangci-lint`, `govulncheck` y `gitleaks`: no instalados en el entorno; la higiene de secretos usa `TestRepoSecurityGate` + hooks `.git/hooks/pre-commit`/`pre-push` (ambos probados en bloqueo y paso), y `go.mod` tiene cero dependencias externas (solo stdlib).
- **HERRAMIENTA-AUSENTE** — `go test -race`: no ejecutado (no figura en `sdd/config.yaml`).
- **ALCANCE-DE-PRUEBA-VIVA** — protocolos `openai` y `gemini` en `direct.Adapter`: cubiertos en unidad con `httptest`; la prueba viva cubrió `api-mistral` según el requisito de la spec («al menos una API directa»).

## Hallazgos descartados

- Ledger `router-02` (`T001`–`T009`): 12 hallazgos únicos (`L11`–`L22`), todos resueltos como `fixed` en su tarea, 0 repetidos, 0 abiertos, media 1.00 ronda por tarea.
- Ronda 1 de revisión de cierre (`421c7b8e`): 4 hallazgos (`3 should-fix`, `1 nit`) — todos corregidos en `cef75ff` y re-verificados en verde.

## Verdict: PASS

Gate Go en verde en los 11 paquetes (`cef75ff`), 10/10 done-checks con evidencia, 19/19 criterios `AC1`–`AC19` en PASS, 3 ejecutores y fallback verificados en vivo, rollback documentado. Listo para la ronda 2 de revisión de cierre (`review`).
