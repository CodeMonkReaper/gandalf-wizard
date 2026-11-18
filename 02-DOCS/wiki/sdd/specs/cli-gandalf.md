---
type: spec
title: Spec — CLI gandalf, configuración y doctor (cierre Foundation 0.1)
description: Punto de entrada único de Gandalf; configuración por capas; diagnóstico del entorno; consulta de memoria ROMERO desde la terminal con salida humana y JSON estable.
tags: [sdd, spec, foundation, cli, config, doctor]
timestamp: 2026-11-18T02:15:00Z
topic: sdd
slug: cli-gandalf
status: shipped
branch: feat/cli-gandalf (mergeada vía PR #14, c81a536; rama borrada)
issue: https://github.com/CodeMonkReaper/gandalf-wizard/issues/5
---

# Spec — CLI gandalf, configuración y doctor

> Slug: `cli-gandalf` · Estado: **aprobada en autopilot, no ítem por ítem** · Creada: 2026-11-18
> Hereda: [constitución v1.2.0](../constitution.md) · Épica: [Issue #5](https://github.com/CodeMonkReaper/gandalf-wizard/issues/5) · Antecedente: [consulta-memoria-romero](consulta-memoria-romero.md)

## Problema y por qué importa

La consulta de memoria en ROMERO (Foundation 0.1, Issue #2) existe solo como biblioteca. Ninguna persona ni agente puede usarla sin escribir código. Tampoco existe un lugar para declarar el proyecto activo y la dirección de ROMERO. Cuando algo falla (ROMERO apagado, credencial ausente, proyecto sin definir), nada lo explica.

## Coste de no construirlo

- La consulta ya construida y verificada queda sin uso real.
- Cada consumidor futuro (Router 0.2, Orchestrator 0.3) tendría que reinventar la lectura de configuración y el cableado.
- Los agentes (Codex, Vibe, Antigravity) no tienen una herramienta de terminal con salida estable para consultar la memoria del proyecto.

## La alternativa más barata

Ejecutar la prueba de integración existente con variables de entorno. Sirve a un desarrollador que conoce el código, pero no a un agente: no hay salida estable, ni códigos de salida, ni diagnóstico. Un programa mínimo con solo flags cubriría la consulta, pero obligaría a repetir los parámetros en cada llamada y seguiría sin explicar los fallos.

## Objetivos

- G1 — Un único comando `gandalf` con subcomandos y ayuda.
- G2 — Configuración por capas con precedencia conocida; se puede ver el valor efectivo de cada clave y de dónde viene.
- G3 — `gandalf doctor` diagnostica configuración, proyecto, credencial, conexión con ROMERO y harnesses instalados, sin modificar nada.
- G4 — `gandalf memory query` expone la consulta de memoria existente con salida humana y JSON estable.
- G5 — Códigos de salida distintos por clase de resultado, para que un agente decida sin leer texto.

## Fuera de alcance

- Escribir, sincronizar o traducir configuración hacia otros harnesses (Codex, Vibe, Antigravity, etc.). Hoy lo hace RSC (principio P10). La detección de harnesses es solo lectura.
- Registros de skills y de patrones (dominios VII y VIII). Van en otra épica.
- Capturar, modificar o calificar recuerdos en ROMERO. La CLI solo lee.
- Guardar la credencial en archivos, en el almacén del sistema operativo o en logs.
- Modo servidor MCP, daemon, interfaz interactiva (TUI), autoactualización y telemetría.
- Empaquetado e instaladores (binarios publicados, Homebrew, winget).
- Versionado, restauración y reparación de configuraciones (IDs 009 y 011).
- Colores, emojis o barras de progreso en la salida.
- Cualquier subcomando no listado en «Qué podrá hacer el usuario».

## Usuarios y contexto

- **El propietario**, en una terminal de Windows, en la raíz de un proyecto.
- **Agentes de IA** (Codex, Vibe, Antigravity) que ejecutan `gandalf` como herramienta de shell y leen la salida JSON y el código de salida.

## Qué podrá hacer el usuario

| Comando | Qué hace | Condición previa |
| --- | --- | --- |
| `gandalf help`, `gandalf --help` | Muestra los subcomandos. | Ninguna. |
| `gandalf version [--json]` | Muestra la versión. | Ninguna. |
| `gandalf config show [--json]` | Muestra cada clave, su valor efectivo y su origen. Indica si la credencial está presente, nunca su valor. | Ninguna. |
| `gandalf config init --project <id>` | Crea la configuración del proyecto en el directorio actual. Nunca sobrescribe. | No existe configuración de proyecto en ese directorio. |
| `gandalf doctor [--json]` | Ejecuta todos los diagnósticos y resume fallos y avisos. Solo lee. | Ninguna. |
| `gandalf memory query [--json] <pregunta>` | Consulta la memoria del proyecto activo en ROMERO. | Proyecto activo definido; credencial en el entorno. |

Los subcomandos que leen configuración aceptan flags que sustituyen un valor solo para esa ejecución: proyecto, identidad, dirección de ROMERO y tiempo límite.

**Capas de configuración**, de mayor a menor precedencia: flag → variable de entorno → configuración del proyecto → configuración global del usuario → valor predeterminado. La credencial de ROMERO solo se lee de una variable de entorno.

## Qué resultado obtiene

**Referencias escritas por el agente en autopilot** (entrada `review-after` Q1). Son comprobables de forma mecánica mediante archivos de referencia en las pruebas.

**1. Consulta de memoria (proyecto ficticio Faro, caso 1 de la spec antecedente):**

```text
$ gandalf memory query "¿Para qué conservamos los informes de prueba?"
Proyecto: faro

| Resumen atribuido a ROMERO | Estado declarado | Vigencia | Referencia del recuerdo | Referencia de la fuente original |
| --- | --- | --- | --- | --- |
| Conservamos los informes para comparar entregas. | observed | vigente | M1 | F1 |
```

**2. La misma consulta con `--json`:**

```json
{
  "schema_version": "1",
  "command": "memory query",
  "project": "faro",
  "status": "ok",
  "status_text": "",
  "degraded": false,
  "degradation_notice": "",
  "entries": [
    {
      "summary": "Conservamos los informes para comparar entregas.",
      "epistemic_state": "observed",
      "vigencia": "vigente",
      "memory_ref": "M1",
      "source_ref": "F1"
    }
  ]
}
```

Sin referencia de origen, `source_ref` vale `null` en JSON y la tabla muestra «Fuente de origen no disponible».

**3. Configuración efectiva:**

```text
$ gandalf config show
| Clave | Valor | Origen |
| --- | --- | --- |
| project | faro | proyecto |
| caller | luis | global |
| romero.base_url | http://127.0.0.1:8420 | predeterminado |
| romero.timeout | 10s | predeterminado |
| credencial (ROMERO_API_KEY) | presente | entorno |
```

**4. Diagnóstico con la credencial ausente:**

```text
$ gandalf doctor
[OK]   configuración     proyecto: <ruta>; global: no encontrado
[OK]   proyecto          faro
[OK]   romero.url        http://127.0.0.1:8420
[FAIL] credencial        ROMERO_API_KEY no está definida
       → Define ROMERO_API_KEY en el entorno de la sesión.
[FAIL] romero.conexión   credencial rechazada o ausente (401)
       → Revisa ROMERO_API_KEY.
[OK]   harness           codex: detectado · vibe: detectado · antigravity: no detectado · rsc: presente
Resultado: fallos=2 avisos=0
```

**5. Códigos de salida (contrato para agentes):**

| Código | Significado |
| --- | --- |
| 0 | Completado: consulta con resultados o sin coincidencias; doctor sin fallos; comando correcto. |
| 1 | Error interno inesperado. |
| 2 | Uso incorrecto: subcomando desconocido, flag inválido, falta un argumento. |
| 3 | Error de configuración: archivo inválido, proyecto ausente, `config init` sobre configuración existente. |
| 4 | Acceso denegado por ROMERO. |
| 5 | Consulta parcial. |
| 6 | Consulta incompleta o ROMERO no disponible. |
| 7 | `doctor` encontró al menos un fallo. |

## Qué debe respetar

- **Spec antecedente** [consulta-memoria-romero](consulta-memoria-romero.md): las nueve reglas de presentación, las etiquetas canónicas («Sin coincidencias visibles», «Consulta incompleta», «Consulta parcial», «Fuente de origen no disponible»), la confidencialidad ante una denegación (sin contenido, identificadores ni recuento) y cero llamadas a ROMERO sin proyecto definido.
- **Constitución P5 (mínimo privilegio):** la credencial nunca aparece en la salida estándar, la salida de error, los archivos de configuración ni los mensajes de error. Un archivo de configuración que intente declarar una credencial se rechaza.
- **Constitución P10 (una fuente de skills):** Gandalf no escribe en `.agents/`, `.codex/`, `.vibe/` ni `.rsc/`.
- **Decisión 2026-11-17 (T007):** biblioteca estándar de Go, sin dependencias externas en `go.mod`.
- **Retro 2026-11-18:** el scope de ROMERO es `project_shared`; en una denegación no se devuelve rastro de auditoría.
- **Soberanía humana:** la CLI solo escribe en su propia configuración de proyecto y solo con `config init`.

## Cómo lo demuestra

- Una suite de aceptación ejecuta la CLI dentro del mismo proceso, con un servidor ROMERO simulado, directorios temporales y un entorno controlado. Compara la salida con archivos de referencia. Hay una prueba por criterio.
- Una prueba de fuga ejecuta cada comando con una credencial centinela y comprueba que no aparece en ninguna salida.
- Una prueba de solo lectura compara el árbol de archivos antes y después de `doctor`.
- Una prueba de humo opcional contra una instancia ROMERO real (perfil lite) se ejecuta solo cuando están definidas sus variables de entorno.
- La evidencia queda en `02-DOCS/wiki/sdd/verifications/cli-gandalf-<fecha>.md` con los comandos ejecutados y sus resultados.

## Comportamiento

- **Camino principal:** la persona define el proyecto con `config init`, exporta la credencial, ejecuta `doctor` (todo OK) y consulta con `memory query`.
- **Bordes:**
  - Sin argumentos, `gandalf` muestra el uso y sale con 2.
  - `memory query` sin pregunta: uso incorrecto, código 2.
  - Clave desconocida en un archivo: aviso; no es un fallo.
  - La configuración de proyecto se busca desde el directorio actual hacia arriba; gana la más cercana.
- **Errores:**
  - Un archivo de configuración inválido detiene `config show` y `memory query` con código 3 e indica el archivo y el motivo. `doctor` lo registra como FAIL y sigue con el resto de diagnósticos.
  - ROMERO no responde dentro del tiempo límite: «Consulta incompleta», código 6.
  - Un diagnóstico que falla no impide ejecutar los demás.
  - Ningún mensaje de error incluye la credencial.

## Criterios de aceptación

| ID | Dado / Cuando / Entonces |
| --- | --- |
| AC1 | Dado el binario, cuando se ejecuta `gandalf --help`, entonces lista `version`, `config show`, `config init`, `doctor` y `memory query` y sale con 0. Con un subcomando desconocido, escribe el error en la salida de error y sale con 2. Sin argumentos, sale con 2. `gandalf version` imprime `gandalf <versión>` y sale con 0. |
| AC2 | Dada la misma clave definida en la configuración global, la de proyecto, el entorno y un flag, cuando se ejecuta `config show`, entonces gana el flag y su origen es `flag`. Al quitar cada capa, gana la siguiente en el orden flag → entorno → proyecto → global → predeterminado. |
| AC3 | Dada la credencial centinela en el entorno, cuando se ejecuta cualquier subcomando (con y sin `--json`, con éxito o con error), entonces la credencial no aparece en la salida estándar ni en la de error, y `config show` informa `presente`. |
| AC4 | Dado un directorio sin configuración de proyecto, cuando se ejecuta `config init --project faro`, entonces se crea la configuración con `project = faro` y sale con 0. Si ya existe, no la modifica y sale con 3. Con un identificador de proyecto inválido, sale con 3 y no crea nada. |
| AC5 | Dado un archivo de configuración con JSON inválido, un valor inválido (URL o tiempo límite) o una clave de credencial, cuando se ejecuta `config show` o `memory query`, entonces se indica el archivo y el motivo, no hay llamadas a ROMERO y sale con 3. Una clave desconocida produce un aviso y no impide ejecutar. |
| AC6 | Dada una configuración válida, la credencial y un ROMERO simulado que responde a su estado, cuando se ejecuta `doctor`, entonces los seis diagnósticos dan OK y sale con 0. |
| AC7 | Dado ROMERO inaccesible, la credencial ausente o el proyecto sin definir, cuando se ejecuta `doctor`, entonces el diagnóstico afectado da FAIL con una línea de remedio, el resto se ejecuta igual y sale con 7. |
| AC8 | Dados ejecutables de harness simulados en la ruta de búsqueda y un manifiesto RSC en el proyecto, cuando se ejecuta `doctor`, entonces informa detectado o no detectado por harness y la presencia de RSC, y el árbol de archivos no cambia. |
| AC9 | Dado Faro con M1 y F1 en el ROMERO simulado, cuando se ejecuta `memory query "¿Para qué conservamos los informes de prueba?"`, entonces la salida coincide con la referencia 1 y sale con 0. |
| AC10 | Dada la misma consulta con `--json`, entonces la salida es JSON válido, coincide con la referencia 2 y `entries` es siempre una lista (vacía cuando no hay resultados). |
| AC11 | Para cada estado de la consulta (ok, empty, partial, incomplete, access_denied, scope_required), la CLI muestra la etiqueta aprobada y sale con el código de la tabla 5. En access_denied y scope_required, la salida no contiene resúmenes ni identificadores de recuerdos. |
| AC12 | Dado ningún proyecto definido, cuando se ejecuta `memory query`, entonces el ROMERO simulado recibe 0 peticiones y la salida sale con 3. |
| AC13 | Dado un ROMERO simulado que no responde, cuando se ejecuta `memory query` con un tiempo límite de 1s, entonces termina en menos de 3s con «Consulta incompleta» y sale con 6. |

## Puntos que siguen abiertos

- **suposición tomada** — Q1: las referencias de salida (tablas, JSON y doctor) las escribió el agente. *Base:* autopilot sin referencia humana. *Riesgo:* si el propietario quiere otro formato, cambian los archivos de referencia y AC9–AC11. → `review-after`.
- **suposición tomada** — Q2: subcomandos en inglés (`config`, `doctor`, `memory query`) y textos en español. *Base:* la convención de las CLIs y la coherencia con `rsc doctor`; las etiquetas aprobadas ya están en español. *Riesgo:* renombrar los subcomandos después rompe a los agentes que ya los usen. → `review-after`.
- **suposición tomada** — Q3: si no se configura la identidad (`caller`), se usa el nombre de usuario del sistema operativo. *Base:* ROMERO autentica con la credencial; la identidad solo delimita el ámbito. *Riesgo:* bajo; se puede fijar en la configuración. → `review-after`.
- **suposición tomada** — Q4: la validación en Linux y macOS es por diseño (rutas según el sistema operativo) y no se ejecuta en este ciclo; solo se verifica en Windows. *Base:* el entorno disponible. *Riesgo:* fallos de rutas fuera de Windows. → `review-after`.
- **decisión diferida** — relación entre Gandalf y RSC para escribir configuración en otros harnesses (dominio I, IDs 003–004). Requiere una decisión propia y posiblemente enmendar P10.
- **decisión diferida** — registros de skills y patrones; empaquetado y distribución de binarios.
- **área no formulable** — cómo descubren e invocan otros agentes a Gandalf a largo plazo (shell frente a servidor MCP). Se concretará con el Router 0.2.

## Clarifications

- 2026-11-18 — El propietario eligió el alcance recomendado: CLI, configuración propia, doctor con detección de solo lectura y `memory query`; sin escribir en otros harnesses.
- 2026-11-18 — Credencial: solo variable de entorno; `doctor` avisa si falta.
- 2026-11-18 — Salida: tabla legible y `--json` estable.
- 2026-11-18 — Modo de ejecución: autopilot completo. Esta spec queda **aprobada en autopilot**; las suposiciones Q1–Q4 quedan en la cola de decisiones como `review-after`.
