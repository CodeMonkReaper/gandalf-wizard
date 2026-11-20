---
type: proposal
title: Primera entrega de sincronización de Gandalf
description: Propuesta de alcance y criterios de éxito de la épica 28.
tags: [sdd, proposal, sync-skills-patrones]
timestamp: 2026-11-19T15:35:19Z
topic: sdd
slug: sync-skills-patrones
status: confirmed
---

# Primera entrega de sincronización de Gandalf

## Problema

Quien mantiene un proyecto con varios harnesses tiene que instalar y revisar a mano las mismas instrucciones y recursos. Esa operación no deja por sí sola una prueba de qué versión recibe cada destino. La [épica #28](https://github.com/CodeMonkReaper/gandalf-wizard/issues/28) retoma este trabajo de Foundation 0.1.

## Intención

El responsable del proyecto declara qué componentes y versiones necesita. Gandalf muestra las diferencias, aplica los cambios que el usuario autoriza y registra el resultado por destino. Otro entorno puede reproducir la misma selección a partir de la declaración del proyecto y el catálogo.

## Alcance inicial propuesto

- **Catálogo propio.** Registrar skills, reglas y patrones disponibles localmente con identificador, versión exacta y procedencia. El proyecto puede consultar y vincular patrones por versión.
- **Destinos iniciales.** Codex CLI y Mistral Vibe CLI. El conjunto sincronizable cubre skills en formato Agent Skills con recursos declarativos, e instrucciones de trabajo del proyecto en Markdown. La prueba se realiza sobre versiones identificadas de cada CLI.
- **Vista previa y aplicación.** Mostrar versiones, archivos afectados, coincidencias, incompatibilidades y conflictos. Una aplicación explícita actualiza los archivos de proyecto autorizados. Si un cambio afecta contenido ajeno a la selección o el destino cambió desde la vista previa, Gandalf detiene la aplicación y explica el motivo.
- **Respaldo y restauración.** Antes de aplicar, registrar el estado previo de los archivos afectados. Una restauración explícita recupera ese estado cuando el destino coincide con el estado que dejó Gandalf. Ante cambios posteriores, informa el conflicto y conserva esos cambios.

La primera entrega se limita al proyecto local y a contenido declarativo. Las reglas son instrucciones de trabajo sujetas a los permisos nativos del harness.

## Referencias

- `GW-03` pide verificar la capacidad por harness y versión. `GW-06`, `GW-07` y `GW-09`–`GW-11` describen el catálogo propio, las skills portables y los patrones. `GW-51` cubre la configuración por proyecto. `AC-02` y el anexo C.2 piden versiones comprobables y una prueba de portabilidad entre dos harnesses. [Especificación maestra v1.1.0-draft](../../../../Gandalf_Wizard_v1.1.0/docs/GANDALF_WIZARD_ESPECIFICACION_MAESTRA.md)
- Codex CLI documenta el descubrimiento de skills e instrucciones dentro del repositorio. Mistral Vibe CLI documenta ambos recursos para proyectos de confianza. La conformidad de Gandalf se demostrará mediante la prueba de aceptación en las versiones objetivo. [Skills en Codex](https://learn.chatgpt.com/docs/build-skills#where-codex-loads-local-skills) · [Instrucciones en Codex](https://learn.chatgpt.com/docs/agent-configuration/agents-md#how-codex-discovers-guidance) · [Skills en Vibe](https://docs.mistral.ai/vibe/code/cli/skills#skill-locations) · [Confianza de proyecto en Vibe](https://docs.mistral.ai/vibe/code/safety-approvals-permissions#trusted-folders)

## Alternativas consideradas

1. **Aplicación explícita y reversible.** Verifica el ciclo completo. Exige definir cómo detectar conflictos y recuperar una aplicación interrumpida.
2. **Inventario y vista previa.** Verifica qué se podría reproducir. El responsable aplica los cambios y comprueba el resultado a mano.
3. **Gestión manual.** Cada proyecto mantiene su propia copia de los recursos. La comprobación de versiones y diferencias se repite por destino y por actualización.

## Riesgos y condiciones de restauración

El destino puede contener instrucciones propias o una entrada con el mismo nombre y origen distinto. La vista previa debe señalar el conflicto antes de escribir. La aceptación debe incluir una interrupción que deje el proyecto en un estado comprobable y recuperable. El informe distingue por destino los componentes disponibles, incompatibles y pendientes de verificar.

## Criterios de éxito propuestos

1. Dos copias nuevas de un proyecto con la misma selección reproducen las versiones y el contenido previsto. Codex CLI y Mistral Vibe CLI descubren una skill y las instrucciones de ejemplo. El informe identifica versiones y orígenes.
2. La vista previa deja los archivos intactos. Una aplicación confirmada realiza los cambios previstos. Repetir la misma aplicación informa cero cambios.
3. Si un archivo destino cambia entre la vista previa y la aplicación, Gandalf conserva el archivo y muestra el conflicto.
4. Restaurar una aplicación completa o interrumpida recupera el contenido anterior. Si hubo una edición posterior, la restauración preserva esa edición e informa el conflicto.

## Recomendación

Entregar la aplicación explícita y reversible en los dos destinos iniciales. Esta opción permite comprobar que Gandalf reproduce la selección de un proyecto de principio a fin. El propietario confirmó el alcance y aprobó la [especificación de la primera entrega](../specs/sync-skills-patrones.md) el 19 de noviembre de 2026.
