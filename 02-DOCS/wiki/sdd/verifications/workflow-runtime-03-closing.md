---
type: verification
title: Verificación de cierre de workflow-runtime-03
description: Evidencia consolidada de T001-T009, controles Go, recorrido de AC-01 a AC-10 y plan de rollback.
tags: [sdd, verification, closing, workflow-runtime-03]
timestamp: 2026-11-21T11:55:00Z
topic: sdd
slug: workflow-runtime-03
verdict: PASS
source_state: 31c257ea (clean)
---

# Verificación de cierre — `workflow-runtime-03` (Épica #40)

**Fecha:** 2026-11-21
**Rama:** `feat/workflow-runtime-03` (base `master` `6b467282`, HEAD verificado `31c257ea`, árbol limpio)
**Veredicto:** **PASS**
**Entorno verificado:** Windows `amd64` · Go `1.27.3` · binario `gandalf` compilado por los tests de aceptación

**Estado posterior:** la [revisión de cierre de `8197fff`](../reviews/workflow-runtime-03.md) encontró brechas fuera de los casos cubiertos por esta ejecución. El resultado de esta suite queda como registro histórico. La [nueva verificación](workflow-runtime-03-2026-11-21-r2.md) cubre las correcciones del candidato local.

## Historial del gate

| Ronda | Estado | Qué pasó |
| --- | --- | --- |
| 1 | **FAIL** (1 ítem abierto) | El recorrido de AC-03 encontró que la cláusula «se conserva la evidencia ya generada» no tenía prueba observable: ningún test leía el artefacto del productor rechazado. Registrado como `L68` y devuelto a `implement`. |
| 2 | **PASS** | `31c257e` añade el aserto en `TestWorkflowAcceptance/AC-03`: tras el cierre `failed`, el artefacto de `sintesis` sigue en disco y contiene la salida rechazada. Todas las cifras de este documento vienen de esta ronda. |

## Matriz de comprobaciones ejecutadas

| Tarea | Comprobación | Resultado | Evidencia |
| --- | --- | --- | --- |
| T001 (#43) | `go test -count=1 -v ./internal/workflow/schema -run '^TestWorkflowSchema$'` | **PASS** (5/5 subtests) | Preflight de solo lectura, `S6` condicional, `definition_sha256` canónico y rechazos tipados. |
| T002 (#44) | `go test -count=1 -v ./internal/workflow/gates -run '^TestWorkflowGates$'` | **PASS** (4/4 subtests) | Seis reglas deterministas, `no_secrets` → `blocked` no reintentable, redacción y frontera de objetivo. |
| T003 (#45) | `go test -count=1 -v ./internal/workflow/store -run '^TestWorkflowStore$'` | **PASS** (5/5 subtests) | Confinamiento `os.Root`, `checkpoint_digest`, escritura atómica, `ErrRunBusy` multiproceso y `cancel.req` sin `RunLock`. |
| T004 (#46) | `go test -count=1 -v ./internal/workflow/engine -run '^TestWorkflowEngineExecution$'` | **PASS** (14/14 subtests) | Paralelismo con barrera, corrección acotada, gates bloqueantes y checkpoint coherente tras fallo. |
| T005 (#47) | `go test -count=1 -v ./internal/workflow/engine -run '^TestWorkflowEngineLifecycle$'` | **PASS** (17/17 subtests) | Parada `4A`, aprobación ligada a corrida/puerta/definición, rechazo obligatorio y opcional, interrupción y cancelación. |
| T006 (#48) | `go test -count=1 -v ./internal/render -run '^TestWorkflowRender$'` y `go test -count=1 -v ./internal/cli -run '^TestRepoSecurityGate$'` | **PASS** (4/4 + 3/3 subtests) | Presentación tabla/`--json` redactada, `.gandalf` y `.gandalf/` en `.gitignore` y en la guardia de seguridad. |
| T007 (#49) | `go test -count=1 -v ./internal/cli -run '^TestWorkflowCLI$'` | **PASS** (12/12 subtests) | Subcomandos, códigos `10`–`16`, puertos inyectables y adaptadores de producción. |
| T008 (#50) | `go test -count=1 -v ./internal/cli -run '^TestWorkflowAcceptance$'` | **PASS** (16/16 subtests) | `AC-01`–`AC-10`, interrupción de proceso real (`12`), contención (`16`) y cancelación cooperativa (`10`). |
| T009 (#51) | `gofmt -l internal/workflow internal/cli/workflow*.go internal/render/workflow*.go` · `go vet ./...` · `go build ./...` · `GOOS=linux GOARCH=amd64 go build ./...` · `go vet` y `go build` cruzados · `go test -count=1 ./...` | **PASS** (`gofmt` vacío, `vet`/`build` limpios, 19/19 paquetes en verde) | Este documento. |

## Controles Go del proyecto

| Control | Salida |
| --- | --- |
| `gofmt -l` (ámbito del plan) | vacío (`rc=0`) |
| `gofmt -l .` (repo completo) | 62 ficheros, todos por finales de línea CRLF del checkout; `gofmt -d` muestra el mismo contenido con `\r`, y ninguno vive en `internal/workflow`, `internal/cli` ni `internal/render` |
| `go vet ./...` | sin hallazgos |
| `go build ./...` | limpio |
| `GOOS=linux GOARCH=amd64 go build ./...` · `go vet ./...` | limpio (compilación y vet cruzados) |
| `go test -count=1 ./...` | 19/19 paquetes `ok` |
| `TestRepoSecurityGate` | verde, con control en las dos direcciones: `bloquea_claves_API_y_archivos_de_sesion_simulados` demuestra que la guardia **falla** ante entradas conocidas malas y `git_ignora_el_estado_local_de_workflows` que **pasa** con las buenas |

## Cobertura de criterios de aceptación (`AC-01`–`AC-10`)

- **AC-01 (preflight de solo lectura, proveedores condicionales):** `TestWorkflowSchema` (`referencia_valida_en_solo_lectura_y_orden_topologico`, `validacion_condicional_de_proveedores_S6`, `workflow_sin_proveedores_S6_pasa_con_ambos_false`), `TestWorkflowCLI/validate_es_de_solo_lectura_y_respeta_los_proveedores_condicionales` y `TestWorkflowAcceptance/AC-01` (0 llamadas, `.gandalf` inexistente tras `validate`). Rechazos con identificador: `rechazos_tipados_de_esquema_y_grafo` y `validate_rechaza_un_router_declarado_pero_no_elegible`.
- **AC-02 (paralelismo real y dependencias):** `TestWorkflowEngineExecution/parallel_dependencies_and_limit` y `TestWorkflowAcceptance/AC-02`, que bloquea cada rama hasta que la otra llega: si el motor las serializara, el test agota el plazo y falla.
- **AC-03 (gates deterministas, bloqueo de dependientes, reglas no eludibles):** `TestWorkflowGates/reglas_deterministas_del_catalogo_cerrado` y `no_secrets_evalua_primero_y_bloquea_sin_revelar_ni_permitir_informativo`; `TestWorkflowEngineExecution/blocking_gate_failure_skips_dependents_of_the_evaluated_step`; `TestWorkflowAcceptance/AC-03` en sus dos variantes: el gate bloqueante deja `sintesis` en `passed`, `gates[]` en `fail` con motivo, el dependiente en `skipped`, la corrida en `failed` (`15`) **y el artefacto rechazado legible en disco**; el informativo registra `warnings` sin bloquear.
- **AC-04 (ciclo acotado de corrección):** `TestWorkflowEngineExecution/correction_succeeds_on_retry` y `correction_stops_at_limit`; `TestWorkflowAcceptance/AC-04` prueba la recuperación en la iteración 2 con ambos intentos conservados y la parada exacta en `max_iterations` (2 llamadas al productor).
- **AC-05 (puerta humana, huella y parada conservadora):** `TestWorkflowEngineLifecycle/approval_stops_the_parallel_frontier_and_resumes_once` (el paso ya `running` concluye y no se lanzan nuevos), `reject_stops_the_required_run_or_only_the_optional_branch` (rechazo obligatorio vs rama opcional); `TestWorkflowAcceptance/AC-05` cubre las tres variantes: la parada 4A con rama posterior que solo se ejecuta tras aprobar, el cierre `succeeded` con traza y el rechazo que bloquea la corrida y su `resume`.
- **AC-06 (integridad de definición y alcance de aprobaciones):** `TestWorkflowEngineLifecycle/approval_is_bound_to_the_current_run,_gate_and_definition` y `TestWorkflowAcceptance/AC-06` (huella ajena → `16`, definición alterada → `16`, sin ejecutar pasos).
- **AC-07 (checkpoints, interrupción y reanudación idempotente):** `interrupt_pauses_and_preserves_completed_steps`, `recoverable_failure_reruns_only_unfinished_steps`, `status_reads_the_last_checkpoint`; `TestWorkflowAcceptance/AC-07` interrumpe con `contexto_romero` ya `passed` y comprueba que `resume` conserva el `run-id`, no repite la memoria y ejecuta la síntesis una sola vez; el caso de proceso real añade `Ctrl+C` sobre el binario (`12`) y `resume` hasta `succeeded` (`0`).
- **AC-08 (cancelación cooperativa y exclusión mutua):** `cancel_a_running_workflow_cooperatively`, `cancel_an_idle_workflow_without_replay`; `TestWorkflowStore/bloqueo_exclusivo_multiproceso_y_cancel_cooperativo`; `TestWorkflowAcceptance` demuestra con dos procesos reales la contención (`16`) y la cancelación desde otro proceso (`10`) esperando `cancel.req`.
- **AC-09 (evaluación contra objetivo y trazabilidad):** `TestWorkflowAcceptance/AC-05 the reference scenario closes succeeded with its full trace` (objetivo → pasos → artefactos → gates → aprobación → veredicto), `AC-09 optional branch failure closes partial` (`partial` con `met_criteria` 2 y traza completa) y `TestWorkflowGates/evaluacion_de_objetivo_q2`, que fija `FailedCriteria` con el criterio incumplido; `TestWorkflowCLI/run_distingue_blocked_de_failed` separa `blocked` (`14`) de `failed` (`15`).
- **AC-10 (aislamiento, redacción y no regresión):** `TestWorkflowAcceptance/AC-10` barre el centinela en stdout, stderr, `--json`, checkpoint y disco, exige la marca de redacción y compara el proyecto por digest antes y después; el caso de proceso repite el barrido tras una interrupción; `TestRepoSecurityGate` vigila claves; la suite completa (19/19) cubre la no regresión de Foundation, CLI, Router y Sync.

## Plan de rollback

- **Entrega:** la rama no está integrada en `master` (`6b467282` es ancestro de `31c257ea`; 38 commits por delante). Descartarla es `git branch -D feat/workflow-runtime-03`. `origin/feat/workflow-runtime-03` existe con un subconjunto de commits; borrarla en remoto exige autorización del propietario (`P5`/`P8`) y no es necesaria para revertir el código.
- **Si ya estuviera integrada:** `git revert --no-commit 6b467282..<merge>` y un solo commit de reversión. El cambio es aditivo: 29 ficheros nuevos y 9 modificados (`+10686`/`−12`); los modificados son `.gitignore` (`+2`), `internal/cli/exitcodes.go` (`+17`), `internal/cli/run.go` (`+14`), `internal/cli/repo_security_test.go` (`+44`), documentación e Issue template. No hay ficheros borrados (`0 D`).
- **Estado en runtime:** las corridas viven en `.gandalf/workflows/`, ignorado por Git (`.gitignore:137-138`, comprobado con `git check-ignore -v`). Borrar ese directorio no toca el repositorio. No hay base de datos, migraciones, daemon ni feature flag que desactivar.
- **Verificación del rollback:** `git status --short` vacío y `git diff --name-status master...HEAD` sin entradas `D`; el árbol de proyecto del usuario que usan los tests (`AC-10`) queda idéntico tras cada corrida.

## Capas no ejecutadas

- **HERRAMIENTA-AUSENTE** — `staticcheck`, `golangci-lint` y `govulncheck` no están instalados en esta máquina; nada corrió en su lugar. Cobertura de esos análisis (bugs estáticos finos y CVEs de dependencias) **no comprobada**.
- **HERRAMIENTA-AUSENTE** — `go test -race`: el toolchain responde `-race requires cgo` y no hay `gcc` en el entorno. Las carreras de datos del motor y del almacén **no se han buscado con ese instrumento**.
- **SUSTITUIDA** — cobertura: se ejecutó `go test -cover` y reporta `engine 87.7%`, `gates 86.3%`, `schema 83.0%`, `store 73.4%`, `cli 81.5%`, `render 65.7%`, pero sin umbral: es un informe, no un gate, y no detecta una regresión. La constitución v1.2.0 difiere el umbral hasta habilitar CI remoto; el arreglo es fijar `-coverprofile` + `diff-cover --fail-under=100` sobre las líneas tocadas.
- **NO-APLICA** — migraciones de datos, contratos de red y despliegue: la épica no toca ninguno.

## Hallazgos descartados

- «`gofmt -l .` marca 62 ficheros, luego el formato está roto» — descartado: `gofmt -d` sobre ellos muestra el mismo contenido con finales `\r\n` que impone el checkout de Windows, y `gofmt -l` acotado al ámbito del plan sale vacío (`internal/workflow`, `internal/cli/workflow*.go`, `internal/render/workflow*.go`).
- «La guardia de seguridad podría estar en verde por no mirar» — descartado: `TestRepoSecurityGate/bloquea_claves_API_y_archivos_de_sesion_simulados` recorre casos conocidos malos y exige hallazgo (`repo_security_test.go:231`), con casos buenos que deben pasar (`:247`).

## Veredicto: PASS

Las nueve comprobaciones de tarea, los seis controles Go del proyecto, las diez claves de aceptación y el plan de rollback tienen evidencia ejecutada en esta ronda. Las tres capas ausentes quedan declaradas arriba y viajan al humano con el veredicto. Listo para la revisión de cierre de la rama.
