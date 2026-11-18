---
type: analysis
title: Analyze — CLI gandalf, configuración y doctor
description: Auditoría de consistencia pre-implementación entre constitución v1.2.0, especificación aprobada en autopilot, plan técnico y tareas T001..T010.
tags: [sdd, analyze, foundation, cli, config, doctor]
timestamp: 2026-11-18T02:22:00Z
topic: sdd
slug: cli-gandalf
verdict: pass
---

# Analyze — CLI gandalf, configuración y doctor

> **GATE: PASS** — `0 CRITICAL` · `0 HIGH` · `0 MEDIUM` · `0 LOW`
> Artefactos contrastados: [constitution.md](../constitution.md) (`v1.2.0`) · [specs/cli-gandalf.md](../specs/cli-gandalf.md) (`approved-autopilot`) · [plans/cli-gandalf.md](../plans/cli-gandalf.md) (`approved-autopilot` + `## Tasks T001..T010`) · [REVIEW.md](../../../../REVIEW.md)

## 1. Requirement Coverage Map (spec → plan → tasks)

| REQ-ID | Spec requirement (short) | Plan section | Task(s) | Status |
| --- | --- | --- | --- | --- |
| AC1 | `--help`, `version`, desconocido, sin args (`0`/`2`) | §0, §2, §3, §5 | T001, T004, T008, T009 | `covered` |
| AC2 | Precedencia de 5 capas (`flag > entorno > proyecto > global > predeterminado`) y `Origen` en `config show` | §0, §3, §5 | T001, T002, T004, T008, T009 | `covered` |
| AC3 | Cero fuga de `ROMERO_API_KEY` centinela en `stdout`/`stderr`; `config show` informa `presente` | §0, §3, §5, §7 | T001, T002, T004, T007, T008, T009 | `covered` |
| AC4 | `config init --project` exclusivo sin sobrescribir y rechazo de ID inválido (`0`/`3`) | §0, §3, §4, §5 | T003, T008, T009 | `covered` |
| AC5 | Archivo JSON/valor/credencial inválido detiene con archivo+motivo (`3`); clave desconocida emite aviso | §0, §3, §5 | T002, T008, T009 | `covered` |
| AC6 | `doctor` con 6 diagnósticos en `OK` cuando entorno y ROMERO están sanos (`0`) | §2, §3, §5 | T001, T004, T006, T007, T008, T009 | `covered` |
| AC7 | `doctor` con fallos emite `FAIL` + línea de remedio sin cortocircuitar y sale con `7` | §0, §3, §5 | T004, T006, T007, T008, T009 | `covered` |
| AC8 | Detección de harnesses y `.rsc.json` en solo lectura sin modificar archivos | §0, §2, §3, §5 | T005, T007, T008, T009 | `covered` |
| AC9 | `memory query` en Faro (`M1`, `F1`) coincide con tabla referencia 1 y sale con `0` | §0, §4, §5 | T001, T004, T008, T009 | `covered` |
| AC10 | `memory query --json` cumple `schema_version: "1"`, `source_ref: null` si falta y `entries: []` si vacío | §0, §3, §4, §5 | T001, T004, T008, T009 | `covered` |
| AC11 | Estados `ok`, `empty`, `partial`, `incomplete`, `access_denied`, `scope_required` con etiquetas y códigos `0`/`3`/`4`/`5`/`6` y opacidad | §0, §3, §4, §5 | T001, T004, T008, T009 | `covered` |
| AC12 | `memory query` sin proyecto activo ejecuta `0` peticiones a ROMERO y sale con `3` | §0, §4, §5 | T008, T009 | `covered` |
| AC13 | Timeout de `1s` frente a ROMERO bloqueado termina en `< 3s` con «Consulta incompleta» y código `6` | §0, §4, §5, §7 | T008, T009 | `covered` |

- **Cobertura hacia adelante (spec → tasks):** `13/13` criterios cubiertos.
- **Deriva de alcance hacia atrás (tasks → spec):** `0 DRIFT` (`T001`–`T009` trazan directamente a `AC1`–`AC13`; `T010` es el gate final de módulo exigido por `tasks/SKILL.md` y `constitution v1.1.0`).
- **Cumplimiento constitucional:** `0 CRITICAL` (solo librería estándar Go en `go.mod`, aislamiento de secretos P5, solo lectura sobre artefactos de otros harnesses P10, trazabilidad por Issue #5 y sub-Issues P7, autoría humana P8).
- **Completitud de portadores (carrier completeness):** `§0 Global Constraints` fija los valores exactos (códigos `0`–`7`, esquema JSON de configuración, reglas de validación, valores predeterminados y claves prohibidas) y cada tarea `T001`–`T009` declara su bloque `Interfaces` (`Consumes` / `Produces`) respaldado por el esqueleto de contratos `T001`.

## 2. Findings Table

*(Sin hallazgos abiertos — `0 CRITICAL`, `0 HIGH`, `0 MEDIUM`, `0 LOW`.)*

## 3. Recommended Routing

- Gate superado (`GATE: PASS`). Proceder a implementar el esqueleto de contratos (`T001`), publicar los sub-Issues autocontenidos (`T002`–`T009`) vinculados a [#5](https://github.com/CodeMonkReaper/gandalf-wizard/issues/5) y habilitar la ejecución delegada por tareas.

```json result-envelope
{
  "status": "complete",
  "executive_summary": "GATE: PASS (0 CRITICAL, 0 HIGH, 0 MEDIUM, 0 LOW). Los 13 criterios AC1..AC13 están trazados sin deriva ni huecos de portadores; listo para T001 contratos-primero y sub-Issues.",
  "artifact": "02-DOCS/wiki/sdd/analysis/cli-gandalf.md",
  "next_recommended": "implement",
  "risk": "low",
  "skill_resolution": {
    "used": ["analyze"],
    "missing": [],
    "fallback": [],
    "compact_rules": [
      "Read adversarially across artifacts, not inside one.",
      "Verify carrier completeness for isolated workers."
    ]
  },
  "evidence": [
    "02-DOCS/wiki/sdd/specs/cli-gandalf.md (PASS rsc spec gate)",
    "02-DOCS/wiki/sdd/plans/cli-gandalf.md (§0..§7 + T001..T010)",
    "02-DOCS/wiki/sdd/analysis/cli-gandalf.md (13/13 ACs covered, 0 drift)"
  ]
}
```
