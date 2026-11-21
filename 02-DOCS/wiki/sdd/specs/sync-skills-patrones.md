---
type: spec
title: Sincronización reproducible de skills y reglas de Gandalf
description: Catálogo propio y sincronización local con vista previa, aplicación y restauración.
tags: [sdd, spec, sync-skills-patrones, catalogo]
timestamp: 2026-11-19T15:53:39Z
topic: sdd
slug: sync-skills-patrones
status: shipped
approved: 2026-11-19
issue: https://github.com/CodeMonkReaper/gandalf-wizard/issues/28
---

# Sincronización reproducible de skills y reglas de Gandalf

El propietario aprobó esta especificación, sus supuestos y el ejemplo de referencia el 19 de noviembre de 2026. [Épica #28](https://github.com/CodeMonkReaper/gandalf-wizard/issues/28) · [Propuesta confirmada](../proposals/sync-skills-patrones.md) · [Plan técnico](../plans/sync-skills-patrones.md)

## Problema y por qué importa

Quien trabaja en el mismo proyecto con varios harnesses necesita las mismas versiones de sus instrucciones y skills. Al copiar archivos a mano, puede perder el origen de una copia, usar una versión distinta de la esperada o sobrescribir cambios locales.

## Coste de no construirlo

Cada preparación o actualización obliga a repetir la selección, la copia, la revisión y la verificación por destino. La deriva entre copias dificulta reproducir el entorno al cambiar de equipo o de harness.

## La alternativa más barata

Mantener una lista de versiones en el repositorio y copiar los recursos a mano. Sirve para conjuntos pequeños. Los responsables deben detectar las diferencias, verificar cada destino y preparar sus propios respaldos en cada cambio.

## Objetivos

- Registrar y consultar un catálogo propio de skills, reglas de trabajo y patrones con identidad, versión exacta y procedencia.
- Reproducir una selección local de skills e instrucciones de proyecto en Codex CLI y Mistral Vibe CLI.
- Permitir una vista previa de solo lectura, una aplicación explícita y una restauración verificable.

## Fuera de alcance

La primera entrega se circunscribe a proyectos locales y a las integraciones declarativas de Codex CLI y Mistral Vibe CLI. Los patrones se consultan y se vinculan por versión como referencias de catálogo. La instalación global, la traducción de políticas ejecutables, la distribución remota y el despliegue de patrones como artefactos nativos corresponden a entregas posteriores.

## Usuarios y contexto

El responsable de un proyecto local selecciona los componentes y autoriza la aplicación o la restauración. Un colaborador con acceso de lectura puede inspeccionar la selección y sus diferencias. El operador debe tener permiso de escritura sobre los destinos que vaya a modificar. Cada harness conserva su mecanismo de confianza del proyecto.

## Qué podrá hacer el usuario

- Registrar recursos locales accesibles. Consultar cada entrada por tipo, identificador y versión exacta y conocer su procedencia.
- Declarar en el proyecto qué skills, reglas y referencias de patrón usa. Una skill contiene instrucciones en formato Agent Skills y recursos declarativos. Una regla es una instrucción de trabajo del proyecto en Markdown.
- Revisar los archivos afectados, las diferencias y la compatibilidad con cada destino. Autorizar explícitamente los cambios mostrados para ese proyecto.
- Inspeccionar el resultado y solicitar la restauración de la operación que dejó ese estado. Si una aplicación quedó interrumpida, identificarla y recuperar el proyecto antes de aplicar otros cambios.

## Qué resultado obtiene

El informe de vista previa identifica proyecto, destinos, componentes, versiones y procedencia. Cada diferencia muestra la acción prevista, la ruta relativa y las huellas de contenido cuando existen. Los informes presentan metadatos y diagnósticos con el contenido de archivos y los valores de secretos excluidos. El resultado de aplicación identifica lo aplicado y el respaldo asociado cuando hubo cambios. El resultado de restauración identifica el estado recuperado o el conflicto pendiente.

### Referencia para aprobar

Un proyecto selecciona la skill de prueba `revision-cambios@1.0.0`, la regla `evidencia-de-pruebas@1.0.0` y el patrón de referencia `revision-por-pares@1.0.0`. La skill contiene instrucciones y un archivo de referencia. La regla pide indicar qué pruebas se ejecutaron. Cada entrada en el informe cita la procedencia del ejemplo local.

1. La vista previa muestra el alta de la skill y la incorporación de la regla a las instrucciones del proyecto. El patrón aparece como referencia versionada.
2. El usuario autoriza los cambios. Gandalf informa aplicación y respaldo. Codex CLI y Mistral Vibe CLI cargan la skill y la regla en sus entornos de prueba.
3. Una nueva vista previa del estado aplicado informa cero cambios. Confirmar esa vista deja el proyecto intacto. Restaurar la operación original recupera los archivos previos y retira los archivos que creó si conservan el contenido aplicado.

## Qué debe respetar

- El catálogo de Gandalf es la fuente de las versiones seleccionadas. Cada combinación de tipo, identificador y versión fija un contenido. Si el origen cambia después del registro, Gandalf usa el contenido registrado o bloquea la aplicación con un diagnóstico de integridad. Registrar contenido diferente para la misma versión devuelve un conflicto. Las dependencias explícitas se comprueban contra las versiones declaradas.
- La autorización se limita al proyecto, los destinos y el conjunto de cambios de la vista previa. La aplicación completa se bloquea antes de escribir si la validación detecta cambios en la selección, el contenido de origen o los destinos de esa vista. El usuario recibe el motivo y puede revisar una nueva vista previa.
- La aplicación actúa dentro del proyecto autorizado y sobre contenido cuya gestión puede atribuirse a Gandalf. Conserva archivos y fragmentos ajenos, incluidos los recursos que administran otras herramientas en el mismo repositorio. Una ruta que escapa del proyecto o una colisión con contenido ajeno bloquea el conjunto antes de escribir. La restauración conserva las ediciones posteriores a la operación.
- Las reglas sincronizadas son consultivas. La confianza, las herramientas y los permisos de ejecución se rigen por la configuración nativa de cada harness. La vista previa, la aplicación y la restauración tratan el contenido como datos declarativos. Los respaldos se conservan localmente dentro del alcance del proyecto con acceso restringido al operador autorizado.
- Se conservan los contratos de la CLI, la consulta de memoria y el router. La detección de harnesses y el diagnóstico existentes siguen siendo de solo lectura. [Constitución v1.2.0](../constitution.md) · [CLI Gandalf](cli-gandalf.md) · [Consulta ROMERO](consulta-memoria-romero.md) · [Router 0.2](router-02.md)

## Cómo lo demuestra

La verificación usa dos copias temporales del mismo proyecto y un catálogo de prueba. Registra las versiones de Gandalf y de los dos CLI, el sistema operativo, los informes y las sumas de verificación del contenido. Usa el mecanismo nativo de cada harness para comprobar que descubre la skill y recibe la regla. Compara el estado inicial, el aplicado, el repetido y el restaurado. Las pruebas de rechazo incluyen una ruta que escapa del proyecto, una edición externa, un destino incompatible y una interrupción controlada. Una prueba con un valor ficticio de secreto verifica que las salidas de la función y los informes de prueba lo excluyen.

El entorno disponible tiene Codex CLI `0.169.0` y Vibe `3.1.2` en Windows. La comprobación de versiones confirma disponibilidad local. La prueba de aceptación dará evidencia de descubrimiento y restauración para las versiones que registre. [Skills de Codex](https://learn.chatgpt.com/docs/build-skills#where-codex-loads-local-skills) · [Instrucciones de Codex](https://learn.chatgpt.com/docs/agent-configuration/agents-md#how-codex-discovers-guidance) · [Skills de Vibe](https://docs.mistral.ai/vibe/code/cli/skills#skill-locations) · [Confianza de proyecto en Vibe](https://docs.mistral.ai/vibe/code/safety-approvals-permissions#trusted-folders)

## Comportamiento

- **Vista previa.** Una selección vacía informa cero cambios. Versiones ausentes, dependencias no satisfechas o destinos sin compatibilidad verificada devuelven el motivo y bloquean la aplicación del conjunto solicitado.
- **Aplicación.** Una operación autorizada da de alta o actualiza el contenido administrado que comprende la vista previa. Gandalf dispone del respaldo antes de escribir. Si compiten varias solicitudes de aplicación o restauración en el proyecto, solo una puede modificarlo a la vez y la otra recibe un diagnóstico. Gandalf comprueba cada archivo antes de escribir. Si detecta un cambio mientras escribe el conjunto, conserva el archivo editado, detiene las escrituras restantes e informa la recuperación pendiente.
- **Interrupción.** Ante fallo o interrupción de una escritura, Gandalf informa qué cambios quedaron aplicados y si el proyecto necesita recuperación. Mientras la recuperación está pendiente, el operador puede inspeccionar el estado y solicitar restauración. La aplicación se habilita cuando los archivos que Gandalf llegó a modificar coinciden con el estado respaldado.
- **Restauración.** La operación explícita recupera el contenido y la existencia previa de los archivos que Gandalf modificó. Si la comprobación previa detecta ediciones posteriores, conserva el conjunto e informa el conflicto. El operador puede resolver el conflicto fuera de la operación y reintentar cuando el archivo coincida con el respaldo anterior o con el último estado que dejó Gandalf. Si se interrumpe la restauración, el reintento reconoce los archivos ya recuperados. Restaurar otra vez una operación recuperada informa cero cambios.

## Criterios de aceptación

1. **AC-01 · Catálogo y selección.** Dado un recurso local de cada tipo, al registrarlos se pueden consultar versión y procedencia y vincular el patrón exacto al proyecto. Registrar la misma identidad y versión con contenido distinto devuelve conflicto. Si se modifica el origen después del registro, la aplicación usa el contenido fijado o se bloquea con diagnóstico de integridad.
2. **AC-02 · Vista previa de solo lectura.** Dado el ejemplo, al previsualizar aparecen la selección y sus diferencias para los dos destinos, y coinciden las sumas de los archivos del proyecto antes y después. Una selección vacía informa cero cambios.
3. **AC-03 · Reproducción.** Dadas dos copias nuevas con el mismo catálogo, declaración y entorno, aplicar la selección autorizada produce el mismo contenido administrado y las mismas versiones en ambas. Los dos CLI descubren la skill de prueba y cargan la regla. El patrón se conserva como referencia del proyecto.
4. **AC-04 · Autorización y repetición.** Una solicitud sin autorización o con la autorización de otro proyecto, destino o conjunto de cambios deja intactos los archivos y explica el bloqueo. Una aplicación confirmada conserva las instrucciones preexistentes ajenas a la selección. Tras aplicar, una nueva vista previa informa cero cambios y confirmarla deja intactos los archivos.
5. **AC-05 · Conflictos y compatibilidad.** Una versión ausente, dependencia no satisfecha, destino incompatible, ruta fuera de proyecto o colisión con contenido ajeno bloquea el conjunto y devuelve la causa. Si dos archivos requieren cambios y se edita uno entre vista previa y aplicación, ambos conservan su estado al inicio de la aplicación. Cuando dos operaciones compiten por el mismo proyecto, solo una muta los archivos y la otra informa el conflicto.
6. **AC-06 · Aplicación con respaldo.** Una actualización de contenido administrado seguida de una restauración recupera los bytes previos. Una aplicación que creó un archivo restaura su ausencia anterior. La evidencia vincula la restauración con el respaldo de la operación.
7. **AC-07 · Interrupción y recuperación.** Interrumpir la aplicación después de una primera escritura deja una operación recuperable. Restaurarla revierte esa escritura y conserva los demás archivos, incluida cualquier edición de un archivo que Gandalf aún iba a escribir. En una prueba independiente, editar un archivo que Gandalf ya modificó bloquea la restauración y preserva la edición. Tras reconciliar el archivo con el respaldo anterior o el último estado aplicado, restaurar recupera el estado anterior y habilita una nueva aplicación. Un reintento tras interrumpir la restauración completa la recuperación pendiente.
8. **AC-08 · Privacidad del informe.** Dado un valor ficticio de secreto en los datos de prueba, las salidas de vista previa, aplicación y restauración omiten ese valor. La evidencia conserva la acción, ruta y huella previstas sin volcar contenido de archivos.
9. **AC-09 · Compatibilidad de la CLI.** Pasan las pruebas existentes de configuración, diagnóstico, memoria, router y control de secretos junto con los nuevos casos de sincronización.

## Puntos a clarificar

- **suposición tomada** — La selección inicial usa versiones exactas de recursos locales. Si un componente declara dependencias, la primera entrega comprueba las versiones ya seleccionadas. *Base:* el alcance confirmado y `GW-06.2`. *Riesgo:* resolver rangos y descargar dependencias ampliaría el contrato.
- **suposición tomada** — La aplicación actualiza y añade recursos administrados. La retirada comprobable de un archivo creado ocurre al restaurar la operación. *Base:* el alcance confirmado de aplicación y recuperación. *Riesgo:* conciliar automáticamente recursos retirados requiere criterios adicionales de identidad y autorización.
- **suposición tomada** — La vista previa muestra diferencias por archivo y permite comprobarlas con huellas. *Base:* el alcance pide identificar cambios y proteger secretos. *Riesgo:* un diff textual exigiría definir qué contenido se muestra y cómo se protege.
- **decisión diferida** — El plan fijará los formatos de declaración, el mecanismo de vinculación entre autorización y vista previa y las comprobaciones nativas de cada CLI.

## Clarifications

- **2026-11-19 · Alcance.** El propietario confirmó catálogo propio, sincronización de skills y reglas en Codex CLI y Vibe CLI, referencias versionadas de patrones y aplicación explícita con respaldo y restauración. La alternativa de vista previa manual quedó documentada en la [propuesta](../proposals/sync-skills-patrones.md).
- **2026-11-19 · Límites comprobables.** La revisión precisó el bloqueo del conjunto ante una vista obsoleta, la recuperación tras conflictos, la integridad del catálogo y la privacidad del informe. Las suposiciones quedan a la vista para su aprobación.
- **2026-11-19 · Aprobación.** El propietario respondió «apruebo» a la solicitud de aprobar la especificación y el ejemplo de referencia para pasar al plan técnico.

Referencias de producto para esta entrega. [GW-SPEC-001 v1.1.0-draft, GW-03, GW-06, GW-07, GW-09–11, GW-51, AC-02 y anexo C.2](../../../../Gandalf_Wizard_v1.1.0/docs/GANDALF_WIZARD_ESPECIFICACION_MAESTRA.md)
