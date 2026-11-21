---
type: plan
title: Plan técnico de sincronización reproducible
description: Arquitectura, contratos y verificación de la primera entrega de Gandalf para Codex y Vibe.
tags: [sdd, plan, sync-skills-patrones]
timestamp: 2026-11-19T16:07:54Z
topic: sdd
slug: sync-skills-patrones
status: approved
approved: 2026-11-19
---

# Plan técnico de sincronización reproducible

> [Especificación aprobada](../specs/sync-skills-patrones.md) · [Constitución v1.2.0](../constitution.md) · [Épica #28](https://github.com/CodeMonkReaper/gandalf-wizard/issues/28)

## 0. Restricciones globales

- Go `1.27.3`, CLI modular, pruebas primero. Controles de proyecto `gofmt -l .`, `go vet ./...` y `go test ./...`. [Configuración ASDD](../config.yaml) · [Convenciones Go](../../stack/go.md)
- «La autorización se limita al proyecto, los destinos y el conjunto de cambios de la vista previa». «Gandalf dispone del respaldo antes de escribir». [Especificación](../specs/sync-skills-patrones.md)
- «Los informes presentan metadatos y diagnósticos con el contenido de archivos y los valores de secretos excluidos». «La aplicación actúa dentro del proyecto autorizado y sobre contenido cuya gestión puede atribuirse a Gandalf». [Especificación](../specs/sync-skills-patrones.md)
- «Se conservan los contratos de la CLI, la consulta de memoria y el router. La detección de harnesses y el diagnóstico existentes siguen siendo de solo lectura». Mantener los significados ya asignados a los códigos 0–10. [CLI entregada](../specs/cli-gandalf.md) · [Router 0.2](../specs/router-02.md)

## 1. Contexto y límites

La primera entrega usa un catálogo y una selección local del proyecto. Las skills y las reglas de trabajo llegan a Codex CLI y Mistral Vibe CLI. Los patrones son referencias versionadas del proyecto. El ejemplo aprobado exige confluencia de dos copias con el mismo catálogo, selección y entorno. [Alcance y AC-01–AC-03](../specs/sync-skills-patrones.md)

El punto de entrada sigue siendo `internal/cli.Run` con dependencias inyectables. Los nuevos comandos tienen un contrato propio de configuración y salida. Se conserva la resolución de configuración de los comandos existentes y se reutiliza la detección local de ejecutables cuando dé el dato requerido.

## 2. Arquitectura

```text
[CLI Gandalf] → [Servicio de sincronización] → [Catálogo local y selección] internos
                            ↓
                 [Planificador de solo lectura] → [Perfiles Codex y Vibe] internos
                            ↓
                 [Escritor y registro de operaciones] → [Archivos y respaldo del proyecto]
                                                                  ↓
                                                          [Codex CLI / Vibe CLI] externos
```

- **CLI y presentación** traducen solicitudes y resultados a texto y JSON. Los informes solo reciben metadatos y causas tipadas.
- **Catálogo** registra una copia inmutable de cada versión local y resuelve las referencias exactas de la selección.
- **Planificador** calcula diferencias, valida compatibilidad y propiedad, y emite un identificador determinista de vista previa. Los perfiles definen descubrimiento nativo y proyección por destino. Si dos perfiles generan el mismo archivo con el mismo contenido, comparten una sola escritura.
- **Escritor** revalida la vista bajo exclusión por proyecto, respalda el estado inicial y registra los efectos. El mismo componente restaura y reanuda una restauración interrumpida.

**Decisión principal.** Una vista previa recalculable alimenta a un único escritor local. Guardar la vista previa en un archivo facilitaría intercambiarla, pero convertiría la consulta en una escritura. El identificador que la CLI muestra al operador vincula su autorización con el estado revisado sin persistir la consulta.

## 3. Interfaces y contratos

| Operación prevista | Entrada y resultado | Garantía |
| --- | --- | --- |
| `catalog register` y `catalog list` | Tipo, identificador, versión y fuente local para registrar. Las consultas muestran la ficha y la huella | Registrar contenido idéntico es idempotente. Un contenido distinto bajo la misma identidad y versión devuelve conflicto. |
| `sync preview` | Raíz del proyecto y selección declarada. Devuelve identificador de vista, destinos y acciones por ruta relativa | Solo lectura, orden estable. Rechaza una versión, dependencia, ruta o destino inválidos antes de emitir una vista aplicable. |
| `sync apply` | Identificador de vista autorizado explícitamente por el operador. Devuelve resultado y operación asociada cuando escribe | Recalcula y compara la vista completa bajo exclusión. Mantiene el conjunto intacto ante una precondición inválida. La vista sin cambios omite la creación de una operación. |
| `sync restore` | Identificador de operación dentro del mismo proyecto. Devuelve estado restaurado, pendiente o en conflicto | Valida primero el conjunto escrito por la operación. Reconoce archivos ya restaurados y preserva una edición posterior. |

El identificador de vista es SHA-256 de una representación canónica que incluye identidad de la raíz, revisión de selección, orígenes y huellas de catálogo, perfiles y versiones de destino y estados anterior y previsto de cada archivo. Se vuelve a resolver el contenido al aplicar. El identificador vincula los cambios y la autorización procede de la invocación explícita y de los permisos del sistema. No actúa como credencial. Los fallos de permisos y validación reutilizan los códigos compatibles existentes y el resultado estructurado distingue compatibilidad, vista obsoleta, operación ocupada y recuperación pendiente.

## 4. Datos y flujo

- **Selección.** `.gandalf-sync.json` versionado por el proyecto declara destinos y componentes por tipo, identificador y versión exacta. Los recursos se registran en el catálogo local antes de resolver la selección. Las dependencias explícitas se validan contra el conjunto elegido.
- **Catálogo y operaciones.** Directorio de estado local bajo la raíz del proyecto, separado de la selección y del material publicado. El catálogo conserva los bytes fijados, la procedencia y la huella. El registro de operaciones conserva estados inicial y previsto por ruta, respaldos y progreso de cada escritura. El estado privado se excluye del control de versiones y requiere acceso restringido con mecanismos de la plataforma.
- **Proyección candidata.** Skills bajo `.agents/skills/` y un bloque de instrucciones de Gandalf en el `AGENTS.md` del proyecto. Los perfiles solo habilitan combinaciones demostradas de versión y plataforma. El planificador conserva los bytes ajenos en un archivo compartido. Los marcadores ambiguos, las entradas ajenas y las colisiones de ruta bloquean la aplicación. Una actualización que requiera retirar un archivo se diagnostica para resolución explícita por restauración.
- **Propiedad.** Un recibo de aplicación identifica el contenido que Gandalf gestionó. La mera coincidencia de ruta, nombre o huella no transfiere propiedad. Para un archivo compartido, solo el bloque reconocido y autorizado es gestionable. El plan informa cuando coincide con contenido administrado por otra herramienta.

### Aplicar y recuperar

1. Resolver las versiones y perfiles y calcular cambios sin escribir. El operador revisa los resultados y confirma el identificador.
2. Tomar un bloqueo local de escritor que el sistema libera al terminar el proceso. Comprobar recuperación pendiente, volver a resolver la vista completa y rechazar cualquier diferencia antes de tocar destinos.
3. Guardar el estado anterior y el registro de intención antes de la primera escritura. Escribir por archivo con precondición de identidad y huella, reverificadas junto al reemplazo: el enlace verificado se elimina por handle y la publicación renombra sin reemplazo, de modo que una entrada externa sustituta nunca se sobrescribe (decisión Q1, 2026-11-19). Validar rutas dentro de la raíz efectiva con `os.Root` y rechazar enlaces y tipos de archivo no admitidos. Cada cambio se prepara en el mismo volumen. El mecanismo de reemplazo se valida para la plataforma objetivo.
4. Si el proceso se interrumpe, comparar huellas anterior y prevista para determinar qué alcanzó a escribirse. Un estado distinto de ambos queda en conflicto. En restauración, reunir las comprobaciones antes de escribir y reanudarlas cuando el operador haya reconciliado los archivos. El registro incluye las escrituras de restauración para poder reintentar.

El conjunto de archivos tiene prevalidación común y recuperación por archivo. El bloqueo serializa a los procesos de Gandalf. Los cambios externos se vuelven a comprobar antes de cada escritura. La prueba de reemplazo y recuperación debe demostrar el comportamiento real de Windows antes de habilitar la mutación. Go documenta que `Rename` en plataformas no Unix carece de garantía de atomicidad. [Raíces y operaciones de archivo en Go](https://pkg.go.dev/os#Root) · [Rename en Go](https://pkg.go.dev/os#Rename) · [Bloqueos en Windows](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-lockfileex)

La configuración de instalaciones anteriores no requiere migración. La nueva selección y el estado local se crean al usar la capacidad.

## 5. Estrategia de pruebas

| Criterio | Nivel | Prueba y dependencia real |
| --- | --- | --- |
| AC-01 | Integración de catálogo | Registrar los tres tipos y la referencia del patrón. Cambiar el origen y comprobar que se usan los bytes fijados. Cubrir conflicto de identidad y versión. Directorio temporal real. |
| AC-02 | Contrato CLI | Comparar archivos y directorios antes y después de vista previa con la selección de ejemplo y vacía. Usar perfiles fijados y capturar solo metadatos. |
| AC-03 | Extremo a extremo nativo | Aplicar en dos copias temporales con mismo catálogo. Comparar huellas y probar descubrimiento de la skill y recepción de la regla en ambos ejecutables reales. Registrar versión, SO y banda de confianza de proyecto. |
| AC-04 | Integración autorizada | Intentar aplicar sin identificador y con el de otro proyecto, destinos o selección. Confirmar una vista válida que conserva el texto ajeno y repetir desde una vista actual. Probar también en una copia ya aplicada. |
| AC-05 | Integración de rechazo | Probar versiones, dependencias, perfiles, conflictos, enlaces o rutas fuera de raíz y dos cambios con uno obsoleto. Lanzar dos procesos coordinados para probar exclusión, incluido aplicar frente a restaurar. |
| AC-06 | Integración de almacenamiento | Restaurar una actualización y un alta. Comparar bytes y ausencia previa y verificar la relación entre respaldo y recibo. Sistema de archivos temporal real. |
| AC-07 | Fallo controlado y proceso real | Interrumpir antes y después de reemplazar un archivo y durante la restauración. Probar conflicto de edición, preservación de un archivo aún intacto, recuperación y reintento. |
| AC-08 | Contrato de salida | Introducir un marcador ficticio en contenido y errores de prueba. Inspeccionar texto, JSON y evidencia en éxito y error. Verificar que los respaldos quedan privados. |
| AC-09 | Regresión | Ejecutar los controles de Go del proyecto, incluidas las suites de CLI, ROMERO, router y secretos. |

Para AC-03, el ensayo inicial usará la sesión nativa o los mecanismos de diagnóstico de los CLI en perfiles temporales. Si observar la regla requiere iniciar una petición, capturar solo el contexto de prueba mediante un endpoint local simulado. Los binarios y su descubrimiento son reales. La evidencia conserva identificadores y huellas. Se prueban Codex `0.169.0` y Vibe `3.1.2` disponibles en Windows y se registra el resultado exacto por destino. [Lista nativa de skills de Codex](https://github.com/openai/codex/blob/main/codex-rs/app-server/tests/suite/v2/skills_list.rs) · [Construcción de contexto de Vibe](https://github.com/mistralai/mistral-vibe/blob/main/vibe/core/system_prompt.py)

## 6. Secuencia y dependencias

1. **Comprobar viabilidad.** Probar el descubrimiento de ambos CLI y las operaciones seguras de archivo, bloqueo y sustitución en Windows. Los resultados fijan el perfil y la política de mutación.
2. **Crear catálogo y selección.** Validar versiones, procedencia, dependencias y estados incompletos. Puede avanzarse con datos sintéticos mientras se ensaya la conformidad nativa.
3. **Construir la vista previa.** Integrar perfiles, resolución de propiedad y huella de la vista. Depende de 1 y 2.
4. **Construir el escritor y la recuperación.** Establecer el bloqueo, el respaldo y el registro de cambios. Depende de 1 y 3.
5. **Integrar la CLI y cerrar aceptación.** Validar contratos públicos, restauración, ambos destinos reales y regresión. Depende de 2–4.

## 7. Riesgos y decisiones abiertas

| Riesgo | Impacto observable | Condición de salida |
| --- | --- | --- |
| Sustitución o exclusión insuficiente en Windows | Sobrescritura, estado ambiguo o dos procesos modificando el mismo proyecto | Ensayo de sustitución y bloqueo con procesos reales, rutas conflictivas y fallos inyectados. Un destino cuya escritura segura quede sin demostrar permanece bloqueado. |
| Diferencia de descubrimiento entre versiones o confianza | Los bytes se proyectan, pero el harness ignora el recurso o carga otro | Ensayo nativo temprano de skill e instrucción. Registrar perfil y precedencia, y tratar la combinación sin prueba como no verificada. |
| Cambios externos durante la aplicación | Se invalida un archivo previo al reemplazo, con operación parcial | Comprobar inmediatamente antes de escribir, detener el conjunto e identificar solo los archivos que requieren recuperación. AC-05 y AC-07 comprueban estos estados. |
| Respaldo expuesto o identificación ambigua de contenido | Divulgación de bytes originales o apropiación de cambios ajenos | Verificar privacidad del estado local, exclusión Git, informes sin contenido y recibo de propiedad. |

Los ensayos fijarán la primitiva de reemplazo y el método exacto para observar instrucciones de ambos CLI. El plan requiere evidencia de ambos antes de dar AC-03 por cumplido. El rollback funcional es la restauración autorizada de cada operación. El rollback de entrega revierte el PR y conserva el catálogo y los respaldos locales para su tratamiento explícito.

## Tasks

<!-- generated by tasks on 2026-11-19; IDs are stable, do not renumber -->

Los comandos de prueba son compromisos de implementación. Cada ejecución con filtro debe mostrar `PASS` para la prueba nombrada. Una prueba ausente u omitida requiere resolver la tarea. Los ensayos nativos se habilitan con `GANDALF_SYNC_NATIVE_TEST=1` y usan proyectos temporales y servicios simulados en loopback.

| ID | [P] | Tarea | Comprobación de cierre | Depende de | Criterios | Riesgo previsto |
| --- | --- | --- | --- | --- | --- | --- |
| T001 | [P] | Demostrar descubrimiento de Codex y Vibe en perfiles aislados | `go test -count=1 -v ./internal/syncnative -run '^TestSyncNativeProbe$'` con la prueba nativa habilitada. La evidencia registra skill y regla observadas en ambos CLI, sus versiones, SO y precedencia | — | AC-03, AC-05 | 1 |
| T002 | [P] | Validar escritura confinada y bloqueo local | `go test -count=1 -v ./internal/syncfs -run '^TestSyncFilesystem$'` pasa en Windows con archivos reales, rutas y enlaces conflictivos, dos procesos, terminación del titular y estados reconocibles antes y después de reemplazar | — | AC-05, AC-06, AC-07 | 1 |
| T003 |  | Implementar catálogo inmutable y lectura de selección | `go test -count=1 -v ./internal/synccatalog -run '^TestSyncCatalog$'` comprueba los tres tipos, cambios del origen, colisiones de identidad, dependencias fijadas, patrón referenciado y selección vacía | T002 | AC-01, AC-02, AC-05 | 1 |
| T004 |  | Calcular vista previa y proyección sin escribir | `go test -count=1 -v ./internal/projectsync -run '^TestSyncPreview$'` comprueba huellas deterministas, cambios por destino, destino no probado, ausencia de efectos y conflictos de propiedad y rutas | T001, T002, T003 | AC-02, AC-03, AC-04, AC-05 | 1 |
| T005 |  | Aplicar vista autorizada con respaldo y registro recuperable | `go test -count=1 -v ./internal/projectsync -run '^TestSyncApply$'` demuestra los rechazos completos, preservación de bytes ajenos, respaldo privado y excluido de Git antes de escribir, segunda vista sin cambios e interrupción antes y después de reemplazar | T002, T004 | AC-03, AC-04, AC-05, AC-06, AC-07, AC-08 | 1 |
| T006 |  | Restaurar operaciones y reanudar la recuperación | `go test -count=1 -v ./internal/projectsync -run '^TestSyncRestore$'` prueba alta y actualización, conflicto posterior, reconciliación, recuperación solo de lo escrito, reintento y exclusión frente a aplicar | T002, T005 | AC-05, AC-06, AC-07 | 1 |
| T007 |  | Integrar catálogo y sincronización en la CLI | `go test -count=1 -v ./internal/cli -run '^TestSyncCLI$'` cubre despacho, argumentos de autorización, resultados de texto y JSON, códigos previos y marcador ficticio ausente de salidas y errores | T003, T004, T005, T006 | AC-01, AC-02, AC-04, AC-06, AC-08, AC-09 | 1 |
| T008 |  | Verificar portabilidad y recuperación de extremo a extremo | `go test -count=1 -v ./internal/cli -run '^TestSyncAcceptance$'` y `go test -count=1 -v ./internal/syncnative -run '^TestSyncNativeAcceptance$'` con la prueba nativa habilitada. Dos copias convergen, ambos ejecutables reconocen la referencia aprobada y la evidencia omite el marcador ficticio | T007 | AC-01 a AC-08 | 1 |
| T009 |  | Cerrar verificación y registrar rollback | Ejecutar las ocho comprobaciones anteriores, comprobar formato con `gofmt -l .` y ejecutar `go vet ./...`, `go build ./...` y `go test -count=1 ./...`. Registrar resultados, versiones y recuperación comprobada en la wiki | T008 | AC-01 a AC-09 | 0 |

### Interfaces por tarea

Los nombres expresan contratos lógicos. Las formas compartidas son `FileState{exists, sha256, file_id, bytes}`, `Change{path, before, after, owner}` y `View{project_id, targets, selection_sha256, id, changes, diagnostics}`. Los bytes se consumen en el planificador y escritor. A la presentación llegan solo ruta relativa, acción, huellas y diagnósticos tipados.

- **T001.** Consume dos CLI con versión identificada y una skill y una regla sintéticas. Produce evidencia `NativeProfile{target, version, os, trust, skill_location, rule_location, precedence}` o un diagnóstico no verificado por destino.
- **T002.** Produce `OpenProject(root) → ScopedProject | Error`, `Observe(path) → FileState | Error`, `TryExclusive(operation) → Lease | Busy | Error` y `Change(lease, path, expected: FileState, desired: FileState) → Changed | Conflict | Error`. Todas las rutas de la interfaz son relativas y se operan bajo la raíz. El lease conserva el identificador de operación y los temporales de publicación se nombran con él (`.gandalf-sync-<operación>-<id>.tmp`) para que el registro de T005 los atribuya.
- **T003.** Consume `Observe`, `TryExclusive` y `Change` de T002 para el estado local. Produce `Register(kind, id, version, source) → CatalogEntry | Conflict | Error` y `Resolve(selection) → []CatalogEntry | Diagnostic`. La entrada es `{kind,id,version,origin,sha256,resources,dependencies}`. Los patrones se conservan como referencias.
- **T004.** Consume `NativeProfile` de T001, `Observe` de T002 y `Resolve` de T003. Produce `Preview(project, selection, profiles, receipts) → View | Diagnostic`. Recibe recibos `{path, owner, after_sha256}`. Sin recibo, el contenido existente se trata como ajeno salvo la incorporación autorizable en el archivo compartido.
- **T005.** Consume `Preview`, `TryExclusive`, `Observe` y `Change`. Produce `Apply(project, view_id) → ApplyResult | Conflict | RecoveryPending` y un registro `Operation{id, project_id, phase, entries[]}`. Cada entrada conserva `{path, before, after, state}`. Las intenciones se registran antes de reemplazar. El recibo expone al planificador la propiedad confirmada.
- **T006.** Consume los registros de T005 y las primitivas de T002. Produce `Restore(project, operation_id) → Restored | Conflict | RecoveryPending`. Actualiza el estado de restauración por entrada bajo el mismo bloqueo. El recibo de propiedad acompaña al estado recuperado.
- **T007.** Consume el catálogo de T003, la vista de T004 y los resultados de T005 y T006. Expone los comandos de §3. La salida general es `{schema_version,status,reason,project,targets[],components[],view_id?,operation_id?,changes[]}`. Las fichas son `{kind,id,version,origin,sha256}` y los cambios son `{path,action,targets[],before_sha256?,after_sha256?}`.
- **T008.** Consume los CLI reales, sus perfiles y la interfaz de Gandalf de T007. Produce versiones, plataforma, resultados y huellas por destino en `verifications/sync-skills-patrones-native.md`.
- **T009.** Consume las ocho comprobaciones anteriores y los controles de Go. Produce el veredicto y las condiciones de rollback en `verifications/sync-skills-patrones-closing.md`.

## Previsión de revisión y entrega

| Dimensión | Previsión | Base |
| --- | --- | --- |
| Líneas modificadas | 1.200–2.400 | Catálogo, escritor, recuperación, CLI y pruebas nativas |
| Archivos | 18–30 | Paquetes Go, pruebas por plataforma y evidencias |
| Riesgo | 8 tareas de nivel 1, 1 documental de nivel 0 | Clasificación por rutas previstas en las nueve invocaciones de `rsc risk --json` con política de `origin/master`. La implementación recalcula sobre el diff real. |
| Entrega | `ask-on-risk` | La previsión supera el presupuesto de 400 líneas y 12 archivos. El primer corte revisable contiene ensayos y catálogo. La integración se revisa después del escritor y la recuperación. |

T001 y T002 tienen archivos y datos de prueba independientes. T003 puede avanzar al quedar lista la interfaz de T002 mientras termina la conformidad nativa. La condición para mutar destinos es demostrar T001 y T002. El cierre depende de las evidencias vivas de ambos CLI.

## Subissues en GitHub

| Tarea | Subissue |
| --- | --- |
| T001 | [#29](https://github.com/CodeMonkReaper/gandalf-wizard/issues/29) |
| T002 | [#30](https://github.com/CodeMonkReaper/gandalf-wizard/issues/30) |
| T003 | [#31](https://github.com/CodeMonkReaper/gandalf-wizard/issues/31) |
| T004 | [#32](https://github.com/CodeMonkReaper/gandalf-wizard/issues/32) |
| T005 | [#33](https://github.com/CodeMonkReaper/gandalf-wizard/issues/33) |
| T006 | [#34](https://github.com/CodeMonkReaper/gandalf-wizard/issues/34) |
| T007 | [#35](https://github.com/CodeMonkReaper/gandalf-wizard/issues/35) |
| T008 | [#36](https://github.com/CodeMonkReaper/gandalf-wizard/issues/36) |
| T009 | [#37](https://github.com/CodeMonkReaper/gandalf-wizard/issues/37) |
