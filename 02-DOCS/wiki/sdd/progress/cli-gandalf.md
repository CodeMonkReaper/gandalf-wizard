---
type: progress
title: Progreso — CLI gandalf, configuración y doctor
description: Registro incremental de tareas TDD, contratos base, cola de decisiones y revisiones para la épica #5 (cierre Foundation 0.1).
tags: [sdd, progress, foundation, cli, config, doctor]
timestamp: 2026-11-18T02:22:00Z
topic: sdd
slug: cli-gandalf
branch: feat/cli-gandalf
issue: https://github.com/CodeMonkReaper/gandalf-wizard/issues/5
---

# Progreso — CLI gandalf, configuración y doctor

> Épica vinculada: [#5](https://github.com/CodeMonkReaper/gandalf-wizard/issues/5) · Spec: [../specs/cli-gandalf.md](../specs/cli-gandalf.md) · Plan: [../plans/cli-gandalf.md](../plans/cli-gandalf.md) · Analyze: [../analysis/cli-gandalf.md](../analysis/cli-gandalf.md)

## Decision queue

- [review-after] Q1 — Salidas de referencia (tablas, JSON y doctor) redactadas en autopilot, registradas en 02-DOCS/wiki/sdd/specs/cli-gandalf.md, T004/T009
- [review-after] Q2 — Subcomandos en inglés (config, doctor, memory query) y salidas en español, registradas en 02-DOCS/wiki/sdd/specs/cli-gandalf.md, T008
- [review-after] Q3 — Fallback de caller al usuario del sistema operativo cuando no está configurado, registrado en 02-DOCS/wiki/sdd/specs/cli-gandalf.md, T002
- [review-after] Q4 — Validación multiplataforma por diseño verificada en Windows en este ciclo, registrada en 02-DOCS/wiki/sdd/specs/cli-gandalf.md, T002/T005

## T001 — 2026-11-18
- status: complete (contratos-primero en `feat/cli-gandalf`)
- files: `internal/config/types.go`, `internal/render/types.go`, `internal/harness/types.go`, `internal/romero/status_port.go`, `internal/doctor/types.go`, `internal/cli/exitcodes.go`, `internal/cli/contracts_test.go`
- risk: tier 1 (`rsc risk --json`, `classes: ["default"]`, `reviewers: 1`)
- controls:
  - tests: ✅ (`go test -v ./...` y `go test -count=1 -cover ./...` en `PASS`, `internal/cli` `TestContracts_T001` + `internal/romero` `90.8%`)
  - build: ✅ (`gofmt -l .` vacío; `go vet ./...` en `PASS`)
  - security: ✅ (`go.mod` con 0 dependencias externas; `EffectiveConfig` no almacena el valor de `ROMERO_API_KEY`; `IsForbiddenCredentialKey` bloquea claves de secreto en configuración)
  - traceability: ✅ (`AC1, AC2, AC3, AC6, AC9, AC10, AC11`)
- decision: esqueleto contratos-primero integrado para habilitar implementación delegada por sub-Issues en `T002`–`T009`.

## Sub-Issues publicados (cola delegable)

| Tarea | Sub-Issue | Paquete / alcance | Dependencias | Tier |
| --- | --- | --- | --- | --- |
| `T002` `[P]` | [#6](https://github.com/CodeMonkReaper/gandalf-wizard/issues/6) | `internal/config/load.go` + tests | `T001` | 1 |
| `T003` `[P]` | [#7](https://github.com/CodeMonkReaper/gandalf-wizard/issues/7) | `internal/config/init.go` + tests | `T001` | 1 |
| `T004` `[P]` | [#8](https://github.com/CodeMonkReaper/gandalf-wizard/issues/8) | `internal/render/render.go` + tests | `T001` | 1 |
| `T005` `[P]` | [#9](https://github.com/CodeMonkReaper/gandalf-wizard/issues/9) | `internal/harness/detect.go` + tests | `T001` | 1 |
| `T006` `[P]` | [#10](https://github.com/CodeMonkReaper/gandalf-wizard/issues/10) | `internal/romero/status_client.go` + tests | `T001` | 1 |
| `T007` | [#11](https://github.com/CodeMonkReaper/gandalf-wizard/issues/11) | `internal/doctor/doctor.go` + tests | `T002` (#6), `T005` (#9), `T006` (#10) | 1 |
| `T008` | [#12](https://github.com/CodeMonkReaper/gandalf-wizard/issues/12) | `internal/cli/run.go` + `cmd/gandalf/main.go` | `T002` (#6), `T003` (#7), `T004` (#8), `T007` (#11) | 1 |
| `T009` | [#13](https://github.com/CodeMonkReaper/gandalf-wizard/issues/13) | `internal/cli/acceptance_test.go` (`AC1`–`AC13`) | `T008` (#12) | 1 |
| `T010` | `#5` | Gate final Go (`gofmt -l .`, `go vet ./...`, `go test ./...`) + cierre | `T001`–`T009` | — |

## T002 — 2026-11-18
- status: complete (commit `6c022fd`, sub-Issue #6)
- red: `go test -v ./internal/config -run TestLoadConfig` falló en aserciones con stub de `Load` (compilaba, no importaba mal)
- green: mismo comando en PASS; `go test -count=1 -cover ./internal/config` 88.8%
- files: `internal/config/load.go`, `internal/config/load_test.go`
- risk: tier 1 (`rsc risk --base a1e7132 --json`: `classes: ["default","docs"]`, `reviewers: 1`, `humanApproval: false`)
- review: ronda 1/3, 3 nits → L1 fixed (bloque duplicado en test), L2/L3 intentional; VERDICT: pass
- controls: tests ✅ (`go test ./...` completo en PASS) · build ✅ (`gofmt -l` vacío, `go vet ./...` PASS) · security ✅ (solo stdlib; errores con motivo fijo sin texto de parseo; `EffectiveConfig` sin valor de `ROMERO_API_KEY`) · traceability ✅ (AC2, AC3, AC5)
- decision: motivos de error sanitizados con cadenas fijas (p. ej. «JSON inválido») sin eco del error de parseo, para que ningún carácter del archivo llegue a stdout/stderr (constitución P5); registrado en decisions.md

## T003 — 2026-11-18
- status: complete (commit `a3716f1`, sub-Issue #7)
- red: `go test -v ./internal/config -run TestInitProjectConfig` falló en aserciones con stub
- green: mismo comando en PASS; `go test -count=1 -cover ./internal/config` 88.1%
- files: `internal/config/init.go`, `internal/config/init_test.go`
- risk: tier 1 (`rsc risk --base 48e5093 --json`: `classes: ["default","docs"]`, `reviewers: 1`)
- review: ronda 1/3, 1 nit → L4 intentional (archivo parcial si Write falla; el error cita la ruta); VERDICT: pass
- controls: tests ✅ (`go test ./...` 3 paquetes ok) · build ✅ (`gofmt -l` vacío, `go vet ./...` PASS) · security ✅ (O_EXCL sin sobrescritura; 0600 en POSIX; aserción de permisos consciente de plataforma por Q4) · traceability ✅ (AC4)
- decision: aserción de permisos 0600 solo en POSIX (Windows ignora bits POSIX en OpenFile); la exclusividad O_EXCL sí se verifica en Windows

## T004 — 2026-11-18
- status: complete (commit `7e44505`, sub-Issue #8)
- red: `go test -v ./internal/render -run TestRender` falló en 10 aserciones con stubs compilando
- green: mismo comando en PASS; `go test -count=1 -cover ./internal/render` 93.9%
- files: `internal/render/render.go`, `internal/render/render_test.go`
- risk: tier 1 (`rsc risk --base 5a41747 --json`: `classes: ["default","docs"]`, `reviewers: 1`)
- review: ronda 1/3, 1 should-fix → L5 fixed (degradation_notice del JSON ahora usa el aviso resuelto del presentador; triangulación con Degraded=true y aviso vacío); VERDICT: pass
- controls: tests ✅ (`go test ./...` 4 paquetes ok) · build ✅ (`gofmt -l` vacío, `go vet ./...` PASS) · security ✅ (render no accede a entorno ni secretos; denegación sin resúmenes ni IDs verificado) · traceability ✅ (AC1, AC2, AC3, AC6, AC7, AC9, AC10, AC11)
- decision: salidas humanas y JSON comparten el aviso de degradación resuelto por el presentador para que agente y persona lean lo mismo; referencias 1-4 de la spec verificadas byte a byte

## T005 — 2026-11-18
- status: complete (commit `9c2e575`, sub-Issue #9)
- red: `go test -v ./internal/harness -run TestDetectHarnesses` falló (stub con report vacío)
- green: mismo comando en PASS; `go test -count=1 -cover ./internal/harness` 100%
- files: `internal/harness/detect.go`, `internal/harness/detect_test.go`
- risk: tier 1 (`rsc risk --base b1e62c8 --json`: `classes: ["default","docs"]`, `reviewers: 1`)
- review: ronda 1/3, 0 hallazgos; VERDICT: pass (solo lectura probada por snapshot de árbol, fallback agy, nil-safe)
- controls: tests ✅ (`go test ./...` 5 paquetes ok) · build ✅ (`gofmt -l` vacío, `go vet ./...` PASS) · security ✅ (solo lookPath inyectable y os.Stat; cero escrituras) · traceability ✅ (AC8)

## T006 — 2026-11-18
- status: complete (commit `c651e08`, sub-Issue #10)
- red: `go test -v ./internal/romero -run TestRomeroStatusClient` falló en 7 aserciones (stub + doble compilando)
- green: mismo comando en PASS; `go test -count=1 -cover ./internal/romero` 91.8%
- files: `internal/romero/status_client.go`, `internal/romero/status_client_test.go`
- risk: tier 1 (`rsc risk --base 1ab53ee --json`: `classes: ["default","docs"]`, `reviewers: 1`)
- review: ronda 1/3, 1 nit → L6 fixed (mensaje «checkStatus status N» en vez de «status status N»); VERDICT: pass
- controls: tests ✅ (`go test ./...` 5 paquetes ok) · build ✅ (`gofmt -l` vacío, `go vet ./...` PASS) · security ✅ (token solo en encabezado; errores sin eco del secreto; decodeCapped con tope 4 MiB) · traceability ✅ (AC6, AC7)
- decision: los errores de transporte los envuelve ya `do` con ErrServiceInterrupted (igual que Search); CheckStatus los devuelve directos

## T007 — 2026-11-18
- status: complete (commit `433d9f4`, sub-Issue #11)
- red: `go test -v ./internal/doctor -run TestDoctorChecks` falló (stub con report vacío)
- green: mismo comando en PASS; `go test -count=1 -cover ./internal/doctor` 97.9%
- files: `internal/doctor/doctor.go`, `internal/doctor/doctor_test.go`
- risk: tier 1 (`rsc risk --base 5a35907 --json`: `classes: ["default","docs"]`, `reviewers: 1`)
- review: ronda 1/3, 0 hallazgos nuevos; VERDICT: pass. Corrección durante el rojo: el test esperaba harness FAIL sin detecciones, pero la referencia 4 de la spec muestra [OK] informativo — la spec manda
- controls: tests ✅ (`go test ./...` 6 paquetes ok) · build ✅ (`gofmt -l` vacío, `go vet ./...` PASS) · security ✅ (credencial solo como presencia; detalle de configuración saneado por ConfigError) · traceability ✅ (AC3, AC6, AC7, AC8)
- decision: check de harness es informativo (nunca FAIL) según referencia 4; romero.conexión no se sondea sin URL y falla con remedio explícito
