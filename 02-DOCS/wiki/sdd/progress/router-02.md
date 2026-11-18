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

## T007 — 2026-11-18
- status: complete · issue [#22](https://github.com/CodeMonkReaper/gandalf-wizard/issues/22) cerrada con close review
- red: `go test ./internal/router/direct -run '^TestAdapter$' -count=1` falló por aserción en 18 de 24 subtests (los 6 restantes esperan el default del stub: unavailable/ErrDiscoveryUnavailable); sin pánico
- green: misma suite ok, 25 subtests (tras añadir el subtest del origen no https en la revisión)
- files: `internal/router/direct/adapter.go`, `internal/router/direct/adapter_test.go`
- commit: `b2f105f` (base `e9bd09b`)
- risk: tier 1 (`rsc risk` sobre el diff real) — 1 revisor
- review: 2 reportados → 2 únicos → 0 blocker; 1 should-fix (L16, el adaptador confiaba en la validación https de config y no verificaba el esquema antes de adjuntar la credencial) y 1 nit (L17, handlers bloqueantes sin `defer close` colgaban la limpieza en la ruta de fallo) corregidos en el mismo commit (amend)
- controls: tests ✅ (`go test ./...` 10 paquetes ok) · build ✅ (`go vet ./...`) · security ✅ (credencial solo al origen https verificado por el adaptador; sin redirects a nivel de transporte; sin reintentos; sin herramientas ni streaming; motivos de cadena fija sin cuerpo, URL ni centinela) · traceability ✅ (AC1, AC2, AC5, AC6, AC11–AC13, AC15, AC16, AC18)
- decisiones de diseño: registradas en [decisions.md](../decisions.md) — un adaptador por protocolo con extensión por configuración, rutas derivadas por protocolo con modelo escapado en gemini, Bearer/x-goog-api-key, esfuerzo solo traducible (reasoning.effort / thinkingBudget entero / nunca en mistral), RoundTrip sin redirects, Probe local con presencia≠validez, Discover siempre unavailable, extracción por protocolo descartando razonamiento
- investigación documental: formas de Responses (output/output_text), Chat Completions (choices/message/content) y GenerateContent (candidates/content/parts con thought) contrastadas con la documentación enlazada del issue
- reviewer: coordinador con diff completo como evidencia (este entorno no expone herramienta de subagentes); registrado como limitación
- errores generados: ninguno en el código final; el rojo inicial fue el esperado de TDD; un error de compilación del rojo (`srv.Certificate` es método en Go 1.27, no campo) se corrigió antes de la primera corrida

## T008 — 2026-11-18
- status: complete · issue [#24](https://github.com/CodeMonkReaper/gandalf-wizard/issues/24) cerrada con close review
- red: `go test ./internal/cli ./internal/doctor ./cmd/gandalf -run 'TestRouterCLI|TestRouterDoctor|TestRouterInterrupt' -count=1` falló por aserción en 19 de 22 subtests de TestRouterCLI y 2 de 3 de TestRouterDoctor (guardas de índice añadidas); TestRouterInterrupt compiló y esperaba al stub
- green: misma suite ok — 22 subtests TestRouterCLI, 3 TestRouterDoctor, TestRouterInterrupt con proceso real interrumpido
- files: `internal/cli/router.go`, `internal/cli/router_test.go`, `internal/cli/exitcodes.go`, `internal/cli/run.go`, `internal/render/router.go`, `internal/render/types.go`, `internal/render/render.go`, `internal/doctor/types.go`, `internal/doctor/doctor.go`, `internal/doctor/router_test.go`, `cmd/gandalf/main.go`, `cmd/gandalf/main_test.go`, `cmd/gandalf/interrupt_windows_test.go`, `cmd/gandalf/interrupt_other_test.go`
- commit: `716e54c` (base `f8c51d8`)
- risk: tier 1 (`rsc risk` sobre el diff real) — 1 revisor
- review: 3 reportados → 3 únicos → 0 blocker; 1 should-fix (L18, constantes syscall de Windows sin build tags: el paquete no compilaba en Linux) y 2 nits (L19 condición muerta en la espera de la prueba; L20 ayuda desactualizada sobre los flags de anulación) corregidos en el mismo commit (amend); `GOOS=linux go vet` verificado limpio
- controls: tests ✅ (`go test ./...` 11 paquetes ok) · build ✅ (`go vet ./...` y `GOOS=linux go vet` en los paquetes tocados) · security ✅ (route nunca ejecuta; run redacta credenciales verificadas con centinela; doctor solo sondeos locales sin cuota; sin goroutines nuevas en el CLI) · traceability ✅ (AC1–AC12, AC15, AC17–AC19)
- decisiones de diseño: registradas en [decisions.md](../decisions.md) — códigos 8/9/10, defaults media/medio, doctor informativo sin sumar fallos, registry de producción por ID, Ctrl+C = os.Interrupt con build tags por plataforma
- reviewer: coordinador con diff completo como evidencia (este entorno no expone herramienta de subagentes); registrado como limitación
- errores generados: ninguno en el código final; el rojo inicial fue el esperado de TDD; corregidos antes del verde: pánico de índice en `routeView` con lista vacía (`Ordered[1:]`), `schema_version`/`command` vacíos en los JSON de route y run, subtest de doctor que comparaba fallos absolutos en vez de contra la línea base; `syscall.SIGBREAK` y `syscall.GenerateConsoleCtrlEvent` no existen en Go 1.27 (resuelto: el runtime mapea ambos eventos de consola a os.Interrupt y el evento de la prueba se envía con LazyDLL); el primer commit arrastró por accidente `gandalf.exe` y configs locales de `.vibe/` (corregido con amend y `.gitignore`)

## T009 — 2026-11-18
- status: complete · issue [#25](https://github.com/CodeMonkReaper/gandalf-wizard/issues/25) cerrada con close review
- red: `go test ./internal/cli -run '^TestRouterAcceptance$' -count=1` no existía (cero coincidencias); tras escribirla, AC2 falló por aserción: el motivo de indisponibilidad mostraba la descripción estática del protocolo del adaptador directo en vez de «falta la credencial»
- green: misma suite ok — 19 subtests identificados por AC (AC1–AC19), verificado con `-v`
- regresión documentada y remitida a su tarea propietaria (T007): el `Probe` del adaptador directo ponía una descripción de protocolo en `Reason` y el catálogo prefiere el motivo del adaptador, enmascarando la causa específica; corregido dejando el `Reason` vacío en el sondeo sano (el catálogo rellena «falta la credencial» / «autenticación desconocida») y actualizando la aserción de T007 a la semántica corregida; el contrato no cambió
- files: `internal/cli/router_acceptance_test.go`, `internal/router/direct/adapter.go`, `internal/router/direct/adapter_test.go`
- commit: `13fae84` (base `5df557c`)
- risk: tier 1 (`rsc risk` sobre el diff real) — 1 revisor
- review: 2 reportados → 2 únicos → 0 blocker; 2 nits (L21, cancelación de AC11 por sondeo con plazo mágico en vez de señal determinista; L22, comentario de `countingAdapter` impreciso sobre el alcance del conteo) corregidos en el mismo commit (amend)
- controls: tests ✅ (`go test ./...` 11 paquetes ok; `TestRouterAcceptance` 19 subtests) · build ✅ (`go vet ./...`) · security ✅ (centinela buscado en stdout, stderr y todos los archivos del entorno temporal tras éxito y fallo en models list, route, run y doctor; config con credencial rechazada con 3 sin revelar el valor; sin cuota: solo simuladores y transporte inyectado) · traceability ✅ (AC1–AC19)
- decisiones de diseño: registradas en [decisions.md](../decisions.md) — suite de aceptación por AC con ejecutores falsos y clientes inyectados; `Reason` del sondeo solo explica problemas
- reviewer: coordinador con diff completo como evidencia (este entorno no expone herramienta de subagentes); registrado como limitación
- errores generados: ninguno en el código final; la regresión de AC2 (motivo enmascarado) se corrigió en la tarea propietaria antes del commit; el rojo inicial fue el esperado de TDD

## T010 — 2026-11-18
- status: complete · issue [#26](https://github.com/CodeMonkReaper/gandalf-wizard/issues/26) cerrada tras completar las correcciones finales y la verificación viva completa
- autorización: el propietario aprobó la Opción A (corregir los hallazgos habilitando Antigravity y Vibe tras sus sondas) y aportó la clave de API directa solo en memoria para la verificación viva de `api-mistral`
- correcciones previas al cierre:
  - `T005`/`T006`: `Probe.Reason` solo incluye la versión del binario cuando `AuthReady` y `TextOnlyEnforced`, evitando enmascarar los motivos de indisponibilidad en el catálogo; `New()` adopta `TextOnlyEnforced` por defecto tras demostrar cada sonda viva (`antigravity` y `vibe`)
  - `T006`: `VerifyTextOnly` de Vibe ejecuta Paso 1 con subcontexto acotado a `20s` (observa 0 archivos creados ante intento bloqueado) y Paso 2 con el contexto padre para confirmar respuesta de solo texto
  - `T008`: `gandalf models list` activa `DiscoveryLocalAndDocumentedFree` combinando perfiles declarados e inventario descubierto; `routerRegistryFor` y `runRun` usan `deps.getenv` para honrar claves inyectadas y redactar secretos también en producción
- controles locales: `gofmt -l .` señala solo los 6 archivos CRLF preexistentes de Foundation · `go vet ./...` vacío · `go test -count=1 ./...` 11 paquetes ok (sin caché)
- sondas de bloqueo de herramientas: antigravity **enforced** (centinela vivo, cero efectos) · vibe **enforced** (sonda en dos pasos `38.653s`: Paso 1 bloqueó sin crear archivos, Paso 2 respondió texto limpio)
- cableado de producción (`cli.Run` con `RouterRegistry` nil y binario compilado `./cmd/gandalf`): `models list --json` exit 0 con descubrimiento activo (`origin: descubierto`), `route --json` exit 0, `doctor --json` exit 7 en entorno aislado con los 3 ejecutores `enforced`
- éxitos vivos (3/3): antigravity ✅ (`exit 0`, `status ok`, `1` intento, `37.487s`) · vibe ✅ (`exit 0`, `status ok`, `1` intento, `11.306s`; binario compilado también `exit 0`) · api-mistral ✅ (`exit 0`, `status ok`, `1` intento, `1.269s`, sin fuga de credencial en `stdout`/`stderr`)
- fallback en vivo: ✅ demostrado — primer perfil con modelo inexistente rechazado por el ejecutor (`failed`), segundo sano `ok`, `exit 0`, `2` intentos
- evidencia: [verifications/2026-11-18-router-02-live-verification.json](../verifications/2026-11-18-router-02-live-verification.json) (depurada: sin respuestas íntegras, sin claves, sin diagnósticos crudos)
- límites remitidos: los protocolos directos `openai` y `gemini` quedan verificados en local con `httptest` (la spec pide al menos una API directa en vivo); siguen pendientes la verificación/revisión de cierre de `router-02` y la segunda spec (`sync-skills-patrones`) de la épica #15
- errores generados: en el primer intento de sonda viva de Vibe con margen total de 90 s, Paso 1 consumió el deadline del contexto padre al traducirse en `--time-limit`; corregido acotando Paso 1 a un subcontexto de 20 s (`step1Ctx`)
