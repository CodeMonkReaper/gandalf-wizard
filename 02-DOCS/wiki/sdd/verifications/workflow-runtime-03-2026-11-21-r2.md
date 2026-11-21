---
type: verification
slug: workflow-runtime-03
verdict: PASS
source_state: "8197fff (dirty)"
---

# Nueva verificación de cierre de Workflow Runtime 0.3

Verificación del 21 de noviembre de 2026 sobre la rama `feat/workflow-runtime-03`. Base local `8197fff`. Hash Git del diff binario Go probado: `e86389668ffd37d310c10944dfa7c8f8eeda2d2f`. La revisión independiente terminó en **APPROVE**, con cero hallazgos pendientes.

| Hallazgo | Corrección y evidencia |
| --- | --- |
| #46, #47 · Una cancelación inicial podía sobrescribirse | `startCreatedRun` toma el lock y vuelve a cargar el estado antes de iniciar. La prueba publica el checkpoint, cancela y comprueba que el hash del checkpoint permanece intacto y que no se llama al proveedor. |
| #46 · Un gate evaluado podía quedar sin evento | Se publica el resultado según la disponibilidad de las dependencias y se conserva el evento de evaluación si el scheduler debe omitir el paso. Las pruebas comprueban el rechazo bloqueante, el gate informativo hermano, la salida de una aprobación y el agotamiento de un ciclo de corrección. Las pruebas de lifecycle verifican que un nuevo ciclo permitido sigue funcionando. |
| #49 · Preflight de Router genérico | `validate` y `run` seleccionan localmente con los parámetros de cada paso. La prueba rechaza un perfil ausente pese a tener otra opción elegible, sin crear corrida. `resume` comprueba pasos que podrían ejecutarse. La prueba retira la disponibilidad del perfil de un paso completado y comprueba que solo se ejecuta el perfil pendiente. |
| #49 · ROMERO perdía la evidencia estructurada | El puerto usa el renderizador de memoria para guardar resultados atribuidos y degradación en JSON. La redacción inspecciona valores decodificados y preserva la estructura. Pasó la prueba con una credencial sintética cuyo valor literal coincide con la clave entrecomillada `"status"`. El subproceso con HTTP local comprobó estado, atribución y limitación en el artefacto persistido. |
| #49 · Consultar una corrida inexistente escribía en disco | `status` abre el almacén existente sin crear directorios. La prueba afirma código 3, `not_found` y ausencia de `.gandalf`. |

Pasaron los done-checks T001–T009, `go test -count=1 ./...` (19 paquetes), `go vet ./...`, `go build ./...`, build y vet para `linux/amd64`, y `git diff --check`. El formato acotado salió vacío. El formato global señala 62 archivos preexistentes con finales CRLF del checkout. El recorrido de AC-01 a AC-10 del [informe original](workflow-runtime-03-closing.md) se reejecutó mediante las suites de esquema, gates, almacén, motor y CLI, incluidos los nuevos casos de AC-01, AC-03, AC-07, AC-08, AC-09 y AC-10.

El rollback del código se verificó aplicando e invirtiendo el diff en un índice Git temporal. El árbol final coincidió con `3bb0f29^{tree}` para el pendiente y con `6b467282^{tree}` para la épica completa. Una vez fusionado un corte, corresponde revertir su PR preservando los cortes previos que se quieran conservar.

Siguen sin ejecutar `go test -race` (CGO deshabilitado y compilador C ausente), `staticcheck`, `golangci-lint` y `govulncheck`. La aceptación de ROMERO usó HTTP local con un servidor de prueba. El candidato técnico está listo para el flujo de integración. La rama remota aún termina en `9143cc8` y el primer corte quedó integrado en #52.
