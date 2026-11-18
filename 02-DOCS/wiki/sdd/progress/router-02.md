---
type: progress
title: Progreso — Router 0.2
description: Registro incremental de tareas, cola de decisiones, delegaciones y revisiones de la épica #15 (spec router-02).
tags: [sdd, progress, router]
timestamp: 2026-11-18T12:30:00Z
topic: sdd
slug: router-02
branch: feat/router-02
issue: https://github.com/CodeMonkReaper/gandalf-wizard/issues/15
---

# Progreso — Router 0.2

> Épica vinculada: [#15](https://github.com/CodeMonkReaper/gandalf-wizard/issues/15) · Spec: [../specs/router-02.md](../specs/router-02.md)

## Decision queue

- [review-after] Q1 — Salidas de referencia (catálogo, route, run JSON) redactadas en autopilot, registradas en 02-DOCS/wiki/sdd/specs/router-02.md, spec
- [review-after] Q2 — OpenCode interpretado como la pasarela OpenCode Zen (API), no el harness, registrado en 02-DOCS/wiki/sdd/specs/router-02.md, spec
- [review-after] Q3 — APIs de esta etapa OpenAI, Gemini y Mistral; el resto por configuración, registrado en 02-DOCS/wiki/sdd/specs/router-02.md, spec
- [review-after] Q4 — Ejecución solo de respuesta de texto, sin herramientas ni auto-approve, registrado en 02-DOCS/wiki/sdd/specs/router-02.md, spec
- [review-after] Q5 — Características declaradas por flags (complejidad, riesgo) con tabla fija; sin inferencia del texto, registrado en 02-DOCS/wiki/sdd/specs/router-02.md, spec
- [review-after] Q6 — Catálogo por descubrimiento cuando existe y por declaración en configuración, registrado en 02-DOCS/wiki/sdd/specs/router-02.md, spec
- [review-after] Q7 — Autorizar ejecutores y enlazar secretos a destinos desde la capa global del usuario, con perfiles ordenados por proyecto, registrado en 02-DOCS/wiki/sdd/plans/router-02.md §3, T001
- [review-after] Q8 — Asignar 8 a sin elegibles, 9 a agotados y 10 a cancelación, registrado en 02-DOCS/wiki/sdd/plans/router-02.md §3, T008

## Fases

- 2026-11-18 — `specify` + `clarify`: spec escrita, revisión en frío (4 hallazgos corregidos), `rsc spec gate` PASS. Estado `clarified`, aprobada en autopilot.
- 2026-11-18 — `plan` + `tasks`: [plan técnico](../plans/router-02.md) en la rama existente con T001–T010. Riesgo por ruta previsto con el fork fijado: nueve tareas tier 1 y una tier 0.
- 2026-11-18 — `analyze`: [informe](../analysis/router-02.md) PASS de consistencia documental, con dos hallazgos medios y uno bajo. Pendientes el registro de subissues en GitHub y la implementación. La verificación viva de los adaptadores requiere autorización de consumo y demostración del modo solo texto.

## Delegaciones a Vibe

- 2026-11-18 — Tarea `router-02-plan` (épica #15): redactar el plan técnico + tareas en `plans/router-02.md`. Tier heavy; modelo pedido y efectivo `mistral-large-4`; run ID `a0cf1bdd-706b-44e5-b06d-983ac7f864ee`; estado `DONE_WITH_CONCERNS`, `stopReason: limit` (6 turnos), exit 3. Resultado: ningún archivo escrito; el perfil `architect` del tier heavy tiene `write_file`/`edit` en `never` y la escritura se denegó por política. Verificación del coordinador: `Test-Path plans/router-02.md` → False; `git status` sin cambios en `02-DOCS`. Limitación: el tier heavy no sirve para producir artefactos escritos. Siguiente paso: el coordinador redacta el plan; no se reintenta con el mismo perfil.

## Subissues en GitHub

2026-11-18 — GitHub lista las diez tareas abiertas como hijas nativas de la [épica #15](https://github.com/CodeMonkReaper/gandalf-wizard/issues/15). Cada ficha incluye dependencias, contratos, alcance, criterios y pruebas. La primera tarea lista para iniciar es T001.

- T001 · [#17](https://github.com/CodeMonkReaper/gandalf-wizard/issues/17)
- T002 · [#18](https://github.com/CodeMonkReaper/gandalf-wizard/issues/18)
- T003 · [#19](https://github.com/CodeMonkReaper/gandalf-wizard/issues/19)
- T004 · [#23](https://github.com/CodeMonkReaper/gandalf-wizard/issues/23)
- T005 · [#20](https://github.com/CodeMonkReaper/gandalf-wizard/issues/20)
- T006 · [#21](https://github.com/CodeMonkReaper/gandalf-wizard/issues/21)
- T007 · [#22](https://github.com/CodeMonkReaper/gandalf-wizard/issues/22)
- T008 · [#24](https://github.com/CodeMonkReaper/gandalf-wizard/issues/24)
- T009 · [#25](https://github.com/CodeMonkReaper/gandalf-wizard/issues/25)
- T010 · [#26](https://github.com/CodeMonkReaper/gandalf-wizard/issues/26)

## T001 — 2026-11-18
- status: complete · issue [#17](https://github.com/CodeMonkReaper/gandalf-wizard/issues/17) cerrada con close review
- red: `go test ./internal/config -run TestRouterPolicy -count=1` falló por política vacía (índice fuera de rango: `Load` aún no poblaba `Router`)
- green: misma suite ok, 21 subtests
- files: `internal/config/router.go`, `internal/config/router_test.go`, `internal/config/load.go`, `internal/config/types.go`
- commit: `0745c24` (base `146428a`)
- risk: tier 1 (`rsc risk` sobre el diff real) — 1 revisor
- review: 1 reportado → 1 único → 0 blocker/should-fix; 1 nit (L11, razón de error inexacta en URL malformada) corregido en el mismo commit
- controls: tests ✅ (`go test ./...` 6 paquetes ok) · build ✅ (`go vet ./...`) · security ✅ (solo nombres de variables de credenciales; centinela verificado en política y errores) · traceability ✅ (AC5, AC13, AC14, AC16, AC17)
- reviewer: coordinador con diff completo como evidencia (este entorno no expone herramienta de subagentes); registrado como limitación
- errores generados: ninguno en el código final; el rojo inicial fue el esperado de TDD; `gofmt -l .` reporta el falso positivo CRLF/LF preexistente documentado en la retro de cli-gandalf

## T002 — 2026-11-18
- status: complete · issue [#18](https://github.com/CodeMonkReaper/gandalf-wizard/issues/18) cerrada con close review
- red: `go test ./internal/router -run '^TestCatalog$' -count=1` falló con catálogo vacío (índice fuera de rango: `Build` era stub)
- green: misma suite ok, 13 subtests
- files: `internal/router/types.go`, `internal/router/catalog.go`, `internal/router/catalog_test.go`
- commit: `85580d3` (base `615f55a`)
- risk: tier 1 (`rsc risk` sobre el diff real) — 1 revisor
- review: 0 hallazgos (ronda 1); revisión del coordinador contra contrato, spec y §0
- controls: tests ✅ (`go test ./...` 7 paquetes ok) · build ✅ (`go vet ./...`) · security ✅ (puerto sin cuota; descubrimiento solo autorizado+documentado; `Execute` nunca invocado por `Build`) · traceability ✅ (AC1, AC2, AC5, AC15, AC19)
- decisión de diseño: `Availability` deriva del sondeo local (instalado + autenticación lista + solo texto demostrado); `Verification` separa la evidencia del modelo (descubrimiento confirmado); `Origin` distingue declarado/descubierto; el inventario de candidatos sin perfil declarado vive en `Catalog.Inventory` ordenado por ID y queda fuera de `Profiles` para que T003 lo excluya
- errores generados: ninguno en el código final; el rojo inicial fue el esperado de TDD; un import sin uso (`errors`) detectado por el compilador y retirado antes del commit

## T003 — 2026-11-18
- status: complete · issue [#19](https://github.com/CodeMonkReaper/gandalf-wizard/issues/19) cerrada con close review
- red: `go test ./internal/router -run '^TestSelection$' -count=1` falló con decisión vacía (`Select` era stub)
- green: misma suite ok, 12 subtests
- files: `internal/router/select.go`, `internal/router/select_test.go`
- commit: `5c73405` (base `85580d3`)
- risk: tier 1 (`rsc risk` sobre el diff real) — 1 revisor
- review: 0 hallazgos (ronda 1)
- controls: tests ✅ (`go test ./...` 7 paquetes ok) · build ✅ (`go vet ./...`) · security ✅ (función pura sin entorno/reloj/azar) · traceability ✅ (AC3–AC5, AC9, AC10, AC12, AC19)
- diseño: `Assessment.Reason` vacío para elegibles (la explicación de elegibles vive en `Choice.Explanation`); el inventario se registra en `Considered` como descartado «sin posición declarada»; el modo manual no suma perfiles ni recorre inventario
- errores generados: ninguno en el código final; el rojo inicial fue el esperado de TDD; un helper `itoa` artesanal se sustituyó por `strconv.Itoa` antes del commit

## T004 — 2026-11-18
- status: complete · issue [#23](https://github.com/CodeMonkReaper/gandalf-wizard/issues/23) cerrada con close review
- red: `go test ./internal/router -run '^TestRun$' -count=1` falló por aserción en los 14 casos (`Run` era stub que devolvía `Result{}`); sin pánico tras añadir guardas de longitud
- green: misma suite ok, 14 subtests
- files: `internal/router/run.go`, `internal/router/run_test.go`
- commit: `89bd302` (base `5a34ef4`)
- risk: tier 1 (`rsc risk` sobre el diff real) — 1 revisor
- review: 1 reportado → 1 único → 0 blocker/should-fix; 1 nit (L12, la explicación en éxito no pasaba por redacción) corregido en el mismo commit (amend)
- controls: tests ✅ (`go test ./...` 7 paquetes ok) · build ✅ (`go vet ./...`) · security ✅ (errores crudos colapsan a motivo genérico; centinela verificado ausente en salida, motivos y explicación; sin goroutines en `Run`) · traceability ✅ (AC5–AC13, AC18)
- decisión de diseño: `Run` toma `getenv func(string) string` como sexto parámetro (registrada en [decisions.md](../decisions.md)); el esfuerzo nativo se resuelve como `profile.Efforts[choice.Effort]` (el orquestador traduce key→nativo); la cancelación del padre gana sobre el plazo del hijo y sobre el éxito; el dedup por ID de perfil y el tope manual de 1 intento aplican antes de ejecutar
- reviewer: coordinador con diff completo como evidencia (este entorno no expone herramienta de subagentes); registrado como limitación
- errores generados: ninguno en el código final; el rojo inicial fue el esperado de TDD; un error de compilación del test (`a1` fuera de ámbito en `check`) y un colgado del RED (`<-started` sin plazo con el stub) se corrigieron antes del verde

## T005 — 2026-11-18
- status: complete · issue [#20](https://github.com/CodeMonkReaper/gandalf-wizard/issues/20) cerrada con close review
- red: `go test ./internal/router/antigravity -run '^TestAdapter$' -count=1` falló por aserción en 21 de 23 subtests (los 2 restantes esperan `unproven`, el default del stub); sin pánico tras añadir guardas de `nil`
- green: misma suite ok, 23 subtests (incluye cancelación y plazo con proceso real compilado en la prueba)
- files: `internal/router/antigravity/adapter.go`, `internal/router/antigravity/adapter_test.go`
- commit: `5fff5e9` (base `f05aa62`)
- risk: tier 1 (`rsc risk` sobre el diff real) — 1 revisor
- review: 1 reportado → 1 único → 0 blocker/should-fix; 1 nit (L13, `--print-timeout` redondeaba a 0s = sin límite en el margen sub-segundo) corregido en el mismo commit (amend)
- controls: tests ✅ (`go test ./...` 8 paquetes ok) · build ✅ (`go vet ./...`) · security ✅ (sin autoaprobación; entorno del hijo sin credenciales de otros proveedores ni ROMERO; stderr y diagnósticos crudos nunca salen; sesión nativa intacta) · traceability ✅ (AC2, AC5, AC6, AC11–AC13, AC15, AC18)
- decisiones de diseño: registradas en [decisions.md](../decisions.md) — indicador de sesión por presencia de `~/.gemini/google_accounts.json`, receta de solo texto con `--settings` de denegación + `--disable-slash-commands` + `--sandbox`, `VerifyTextOnly` centinela para T010, `--effort` solo con esfuerzo nativo declarado, exclusión de credenciales por patrones, `--print-timeout` reflejando el plazo
- investigación documental: ayuda del binario 1.5.10 contrastada con la documentación oficial (headless, permisos CLI); `--settings` valida claves sin cuota; `agy models` emite TSV `slug→Nombre (Esfuerzo)`
- reviewer: coordinador con diff completo como evidencia (este entorno no expone herramienta de subagentes); registrado como limitación
- errores generados: ninguno en el código final; el rojo inicial fue el esperado de TDD; un error de compilación (`exec.LookPath` devuelve `(string, error)`, no `(string, bool)`) y un pánico del rojo (`f.last()` nil con el stub) se corrigieron antes del verde

## T006 — 2026-11-18
- status: complete · issue [#21](https://github.com/CodeMonkReaper/gandalf-wizard/issues/21) cerrada con close review
- red: `go test ./internal/router/vibe -run '^TestAdapter$' -count=1` falló por aserción en 19 de 21 subtests (los 2 restantes esperan `unproven`, el default del stub); sin pánico tras añadir guardas de `nil`
- green: misma suite ok, 22 subtests (incluye cancelación, plazo y exit 3 con procesos reales compilados en la prueba)
- files: `internal/router/vibe/adapter.go`, `internal/router/vibe/adapter_test.go`
- commit: `7e5d413` (base `65d3b10`)
- risk: tier 1 (`rsc risk` sobre el diff real) — 1 revisor
- review: 2 reportados → 2 únicos → 0 blocker/should-fix; 1 minor (L15, exit 3 documentado clasificado como executor-error en vez de timeout) y 1 nit (L14, `strconvItoa` artesanal) corregidos en el mismo commit (amend)
- controls: tests ✅ (`go test ./...` 9 paquetes ok) · build ✅ (`go vet ./...`) · security ✅ (sin autoaprobación ni `--trust`; entorno del hijo sin credenciales ajenas, `ROMERO_` ni contexto `VIBE_`; stderr y diagnósticos crudos nunca salen; sesión nativa intacta; `Execute` bloqueado sin solo texto demostrado) · traceability ✅ (AC2, AC5, AC6, AC11–AC13, AC15, AC18)
- decisiones de diseño: registradas en [decisions.md](../decisions.md) — indicador de sesión por presencia de `~/.vibe/whoami_cache.json` (ausente → `unknown`), `Discover` siempre `unavailable` (sin descubrimiento documentado sin cuota), esfuerzo omitido (Vibe 3.1.2 no declara flag), receta `--agent ask` + `--enabled-tools` sin coincidencias + `--max-turns 1` + `--output json` + `--output-dir` aislado, exclusión de `VIBE_*` del entorno del hijo, exit 3 → `FailureTimeout`, `--time-limit` reflejando el plazo (mínimo 1 s)
- investigación documental: ayuda del binario 3.1.2 contrastada con la documentación oficial (modo programático, códigos de salida 0–3, `--enabled-tools` desactiva el resto, `--agent ask` deniega herramientas en headless); forma del JSON de sesión verificada localmente
- límite registrado: la corrida de investigación de la forma JSON hizo una llamada parcial al modelo (exit 3 con guardas `--max-turns 0 --max-price 0 --time-limit 20`); consumió una cantidad mínima de cuota; la evidencia viva del centinela queda para T010 con autorización
- reviewer: coordinador con diff completo como evidencia (este entorno no expone herramienta de subagentes); registrado como limitación
- errores generados: ninguno en el código final; el rojo inicial fue el esperado de TDD; un bug del helper de pruebas (`sessionMessages` interpolaba un `\n` real dentro del JSON y lo invalidaba) se detectó con un test de depuración temporal y se corrigió escapando con `json.Marshal` antes del verde
