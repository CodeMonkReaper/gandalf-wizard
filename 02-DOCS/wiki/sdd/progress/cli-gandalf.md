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
