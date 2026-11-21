# Revisión de cierre de workflow-runtime-03

**5 hallazgos: 3 bloqueantes, 2 por corregir. Veredicto: CHANGES REQUESTED.**

Revisión del 21 de noviembre de 2026 sobre `8197fff`. La base de la épica es `6b467282`. El primer corte está integrado en `origin/master` (`3bb0f29`, PR #52). El cálculo `rsc risk --base origin/master --json` dio tier 1. Participó un revisor de contexto fresco.

| Hallazgo | Issue | Evidencia | Condición para cerrar |
| --- | --- | --- | --- |
| Bloqueante · El preflight de Router ignora la selección del paso (AC-01, S6) | #49 | `workflowRouterEligible` en `internal/cli/workflow.go` usa la selección genérica. `routerPort.Execute` aplica perfil, riesgo y complejidad del paso. La selección genérica puede encontrar un candidato aunque el perfil explícito esté ausente. | Verificar la elegibilidad local de cada paso. Probar que un perfil explícito ausente con un candidato por defecto disponible produce rechazo antes de crear una corrida. |
| Bloqueante · ROMERO pierde la estructura y las limitaciones declaradas en la especificación | #49 | `memoryPort.Query` en `internal/cli/workflow.go` reduce resultados normales, vacíos y parciales a texto concatenado. Desaparecen el estado, la atribución y `DegradationNotice` definidos en `internal/romero/types.go`. | Conservar resultados estructurados y limitaciones en el artefacto. Comprobarlo con el adaptador de producción y un servidor ROMERO de prueba. |
| Bloqueante · Una cancelación inicial puede revertirse (AC-08) | #46, #47 | `engine.Run` crea el checkpoint antes de tomar el lock y conserva su copia inicial. Si `Cancel` adquiere el lock entre esos pasos y publica `cancelled`, `Run` puede tomarlo después y escribir `running` con la copia anterior. | Proteger la publicación inicial con el lock o comprobar el último estado bajo lock. Probar la intercalación con una barrera determinista. |
| Por corregir · Consultar una corrida inexistente crea estado local | #49 | El binario ejecutó `workflow status --json 00000000000000000000000000000000` en una carpeta temporal. Devolvió `not_found` y código 3, y creó `.gandalf/workflows/runs`. `runWorkflowStatus` abre `store.OpenRoot` con creación de directorios. | Abrir el almacén para lectura y conservar el código 3 sin crear directorios. |
| Por corregir · Un gate evaluado puede quedar registrado como omitido (AC-03/AC-09) | #46 | En `internal/workflow/engine/execution.go`, la barrera que omite dependientes también alcanza al gate que ya rechazó la salida. Se conserva `gates[].status=fail`, pero el paso queda `skipped` y falta su evento `gate_evaluated`. | Registrar el veredicto del gate y conservar la omisión de los pasos posteriores. |

## Verificación independiente

- Pasaron `go vet ./...`, `go build ./...`, el build y vet cruzados con `GOOS=linux GOARCH=amd64` y `go test -count=1 ./...` (19 paquetes). Pasaron los done-checks dirigidos de T004–T008, incluida la suite de proceso real y `TestRepoSecurityGate`. El formato acotado salió vacío y `git diff --check origin/master...HEAD` pasó. El formato global enumera 62 archivos con CRLF en el checkout.
- Se invirtió el diff binario en índices Git temporales. El árbol resultante coincidió exactamente con `3bb0f29^{tree}` para el corte pendiente y con `6b467282^{tree}` para la épica completa. La prueba no cambia ramas, archivos del proyecto ni el índice habitual. En GitHub, la reversión debe corresponder al PR integrado que se quiera revertir.
- El PASS anterior documenta la ejecución de la suite existente. Los hallazgos de esta revisión dejan abierto el cierre integral. T008 necesita cubrir los casos de integración afectados y T009 debe volver a verificar el candidato corregido. Siguen pendientes los análisis declarados ausentes en el informe previo.

## Revisión de las correcciones

**APPROVE. Cero bloqueantes y cero por corregir pendientes.** Revisión independiente de tier 1 sobre la rama local con base `8197fff`. La primera vuelta detectó que `resume` exigía perfiles completados, que la redacción sobre JSON serializado podía invalidar el artefacto y que faltaban casos de trazabilidad de gates. La segunda vuelta comprobó la selección de pasos restantes, la redacción sobre valores y el registro de los gates de corrección, aprobación y evaluación hermana. El revisor ejecutó los paquetes engine, gates, store y cli con resultado verde.

El detalle de la corrección de los cinco hallazgos, la regresión Go y la reversión comprobada está en la [nueva verificación de cierre](../verifications/workflow-runtime-03-2026-11-21-r2.md). Queda pendiente la integración por cortes.
