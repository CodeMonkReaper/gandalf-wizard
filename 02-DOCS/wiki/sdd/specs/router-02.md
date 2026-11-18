---
type: spec
title: Spec — Router 0.2: catálogo de perfiles, selección explicable y ejecución con fallback
description: WHAT y WHY de Router 0.2 — dos harnesses (Antigravity, Mistral Vibe) y APIs directas, catálogo de modelos y esfuerzos, selección explicada, ejecución no interactiva y fallback.
tags: [sdd, spec, router]
timestamp: 2026-11-18T12:30:00Z
topic: sdd
slug: router-02
status: clarified
branch: feat/router-02
issue: https://github.com/CodeMonkReaper/gandalf-wizard/issues/15
---

# Spec — Router 0.2

> Slug: `router-02` · Estado: **aprobada en autopilot, no ítem por ítem** · Creada: 2026-11-18
> Hereda: [constitución v1.2.0](../constitution.md) · Épica: [Issue #15](https://github.com/CodeMonkReaper/gandalf-wizard/issues/15) · Antecedente: [cli-gandalf](cli-gandalf.md)
> Inventario de referencia: contexto maestro 4.II (025–031), 4.III (032–042), 4.IV (043–054), 4.V (055–071). Segunda spec de la épica: `sync-skills-patrones` (pendiente).

## Problema y por qué importa

Hoy la persona elige a mano qué herramienta, qué modelo y qué esfuerzo usar en cada tarea. Esa elección no queda escrita ni explicada. Si la herramienta elegida falla o no está disponible, la tarea se detiene. La CLI `gandalf` existe, pero no sabe qué modelos hay, no elige nada y no ejecuta nada.

## Coste de no construirlo

- Cada elección de modelo y esfuerzo sigue siendo manual, implícita y sin rastro.
- Una caída o un límite de una herramienta para el trabajo hasta que alguien cambia de herramienta.
- Las etapas 0.3 (Orchestrator) y 0.4 (Governor) no tienen sobre qué construir: ambas necesitan un catálogo de perfiles y una forma común de ejecutar.

## La alternativa más barata

Una tabla escrita a mano en la wiki con «para tal tipo de tarea, usar tal modelo y esfuerzo», y la persona ejecuta cada herramienta por su cuenta. No cuesta nada y cubre parte de la elección, pero no detecta si un perfil está disponible, no ejecuta, no aplica fallback y no deja una explicación por decisión. Otra opción más pequeña es solo recomendar sin ejecutar; el propietario la descartó (decisión 2026-11-18, frontera 1) y pidió alcance de extremo a extremo.

## Objetivos

- G1 — Un catálogo de perfiles de ejecución (ejecutor + modelo + esfuerzo) con su disponibilidad y el origen de cada dato.
- G2 — Una selección determinista de perfil para una tarea, con explicación: criterios aplicados, perfil elegido y alternativas descartadas con su motivo.
- G3 — Ejecución no interactiva de una tarea en el perfil elegido, con resultado normalizado igual para todos los ejecutores.
- G4 — Fallback: si el perfil elegido no está disponible o falla, se intenta el siguiente perfil elegible, y la salida dice qué pasó.
- G5 — Ejecutores de esta etapa: Antigravity y Mistral Vibe (harnesses con suscripción) y APIs directas de proveedores autorizados.
- G6 — Nada se ejecuta sin autorización explícita en la configuración. Las credenciales nunca salen del entorno.

## Fuera de alcance

- Sincronización de skills y patrones: es la segunda spec de la épica (`sync-skills-patrones`).
- Codex, Claude Code, Gemini CLI, OpenCode CLI, Kimi Code y otros harnesses como ejecutores. Solo Antigravity y Mistral Vibe.
- Modelos locales (ID 024).
- Routing por coste, cuota, latencia o rendimiento histórico (061–063, 068–070) y el optimizador de suscripciones (dominio VI).
- Clasificación automática de la tarea a partir de su texto (043–047). En esta etapa las características de la tarea se declaran.
- Que el ejecutor modifique archivos, ejecute comandos o use herramientas en nombre de Gandalf. La ejecución pide una respuesta, no acciones.
- Aprobar permisos automáticamente en un ejecutor (sin auto-approve).
- Streaming de eventos parciales (029). Se entrega el resultado final.
- Guardar resultados o decisiones en ROMERO, aprendizaje y evaluación competitiva.
- Workflows, varios pasos, varios agentes o tareas en paralelo.
- Reintentos sobre el mismo perfil, colas o ejecución en segundo plano.
- Guardar credenciales en archivos, en el almacén del sistema o en logs.
- Cualquier subcomando no listado en «Qué podrá hacer el usuario».

## Usuarios y contexto

- **El propietario**, en una terminal de Windows, en la raíz de un proyecto con `gandalf` configurado.
- **Agentes de IA** que llaman a `gandalf` como herramienta de shell y leen la salida JSON y el código de salida.

## Qué podrá hacer el usuario

| Comando | Qué hace | Condición previa |
| --- | --- | --- |
| `gandalf models list [--json]` | Muestra el catálogo de perfiles: ejecutor, modelo, esfuerzos admitidos, disponibilidad y origen del dato. | Ninguna. |
| `gandalf route [--json] [características] <tarea>` | Elige un perfil para la tarea y explica por qué. No ejecuta nada. | Al menos un ejecutor autorizado. |
| `gandalf run [--json] [características] [--profile <perfil>] <tarea>` | Elige un perfil (o usa el indicado), ejecuta la tarea y devuelve el resultado normalizado. Aplica fallback si hace falta. | Ejecutor autorizado y disponible; su autenticación lista. |
| `gandalf doctor [--json]` | Además de lo actual, diagnostica cada ejecutor: instalado, autorizado y con autenticación lista. Solo lee. | Ninguna. |

**Características de la tarea** (opcionales):

| Característica | Valores | Por defecto | Efecto en la selección |
| --- | --- | --- | --- |
| Complejidad | baja, media, alta | media | Fija el esfuerzo pedido: baja → low, media → medium, alta → high. |
| Riesgo | bajo, medio, alto | medio | Con riesgo alto se excluyen los perfiles «no verificados». Bajo y medio no cambian el orden. |

El orden entre perfiles elegibles es el que declara el propietario en la configuración.

**Autorización:** el propietario declara en la configuración qué ejecutores están autorizados. Un ejecutor no autorizado aparece en el catálogo como «no autorizado» y nunca se elige ni se ejecuta.

**Selección manual (053):** `--profile` sustituye la recomendación. Si ese perfil no es elegible, Gandalf lo dice y no ejecuta otro en su lugar.

## Qué resultado obtiene

**Referencias escritas por el agente en autopilot** (entrada `review-after` Q1). Se comprueban de forma mecánica con archivos de referencia en las pruebas. Los nombres de modelos son de ejemplo; el catálogo real solo muestra modelos verificados.

**1. Catálogo:**

```text
$ gandalf models list
| Perfil | Ejecutor | Modelo | Esfuerzos | Disponibilidad | Origen |
| --- | --- | --- | --- | --- | --- |
| antigravity/modelo-a | antigravity | modelo-a | low, medium, high | disponible | descubierto |
| vibe/modelo-b | vibe | modelo-b | — | disponible | declarado |
| api-mistral/modelo-c | api:mistral | modelo-c | — | no disponible: falta la credencial | declarado |
| api-openai/modelo-d | api:openai | modelo-d | low, medium, high | no autorizado | declarado |
```

**2. Selección explicada:**

```text
$ gandalf route --complexity alta --risk medio "Revisar el diseño del módulo de pagos"
Perfil elegido: antigravity/modelo-a · esfuerzo: high
Criterios: complejidad alta → esfuerzo high; riesgo medio; solo perfiles autorizados y disponibles.
Alternativas:
- vibe/modelo-b: elegible, segundo en el orden (no admite esfuerzo high).
- api-mistral/modelo-c: descartado, no disponible (falta la credencial).
- api-openai/modelo-d: descartado, no autorizado.
Fallback previsto: vibe/modelo-b.
```

**3. Ejecución con fallback, en JSON:**

```json
{
  "schema_version": "1",
  "command": "run",
  "status": "ok",
  "profile": "vibe/modelo-b",
  "effort": "",
  "attempts": [
    { "profile": "antigravity/modelo-a", "effort": "high", "outcome": "failed", "reason": "el ejecutor terminó con error" },
    { "profile": "vibe/modelo-b", "effort": "", "outcome": "ok", "reason": "" }
  ],
  "explanation": "Elegido por complejidad alta; se usó el fallback tras el fallo del primer perfil.",
  "output": "<texto de la respuesta del ejecutor>"
}
```

**4. Sin perfiles elegibles:** mensaje en español que nombra el motivo de cada descarte y código de salida propio. No se ejecuta nada.

## Qué debe respetar

- **Constitución v1.2.0:** P3 (spec y plan aprobados antes de código), P4 (cada capacidad declarada operativa tiene evidencia; las demás se marcan como no verificadas), P5 (mínimo privilegio; las llamadas reales a proveedores consumen cuota y requieren autorización humana), P6, P7 (Issues y PR) y P8.
- **Contexto maestro:** no inventar modelos, esfuerzos ni cuotas (sección 8). Una suscripción no asegura acceso programático (sección 1). Anfitrión, ejecutor, modelo y esfuerzo son conceptos distintos. Los modelos proponen; el runtime comprueba y autoriza.
- **Spec `cli-gandalf` (entregada):** la configuración en 5 capas, los subcomandos existentes, la salida tabla + `--json` con `schema_version: "1"` y los códigos de salida 0–7 siguen igual. Las clases nuevas de resultado reciben códigos nuevos; ninguno existente cambia de sentido.
- **Decisión 2026-11-18 (T002):** los errores de configuración no repiten valores ni fragmentos de archivos.
- **Credenciales:** como `ROMERO_API_KEY`, las claves de API solo se leen del entorno. La configuración que contenga una credencial se rechaza. Ninguna salida, error o log muestra una credencial.
- **Autenticación nativa (027):** Antigravity y Vibe usan su propia sesión iniciada. Gandalf no lee ni copia sus credenciales.
- **`doctor` sigue siendo solo lectura** y no hace llamadas que consuman cuota.

## Cómo lo demuestra

- **Suite de aceptación en proceso** (patrón de la retro de cli-gandalf): cada criterio de aceptación tiene una prueba con ejecutores falsos que simulan éxito, fallo, tiempo límite, cancelación y falta de autenticación. Incluye un centinela de fuga de credenciales y la comprobación de que no se escribe ningún archivo fuera de lo previsto. Evidencia: `go test ./...` y la salida guardada en `verifications/`.
- **Integración viva autorizada:** una ejecución real de `gandalf run` en Antigravity, en Vibe y en al menos una API directa, con un prompt trivial. Se guarda en `verifications/` el JSON (sin la respuesta completa), la fecha y las versiones. Necesita autorización humana porque consume cuota; mientras no la haya, esos ejecutores se clasifican como «no verificados» (P4).
- **Fallback probado en vivo:** un perfil autorizado deliberadamente roto (modelo inexistente) seguido de uno sano, con el resultado registrado.

## Comportamiento

- **Camino principal:** `run` lee la configuración, construye el catálogo, filtra por autorizados y disponibles, ordena según las características, ejecuta el primero, normaliza la respuesta y la imprime con la explicación.
- **Orden determinista:** con el mismo catálogo, la misma configuración y las mismas características, la selección es siempre la misma.
- **Fallback:** ante no disponibilidad, error del ejecutor o tiempo límite, se pasa al siguiente perfil elegible. Cada intento queda en `attempts`. Se para en el primer éxito o cuando se acaban los perfiles.
- **Selección manual:** con `--profile`, un solo intento y sin fallback.
- **Esfuerzo:** si el perfil no admite esfuerzos, se ejecuta sin esfuerzo y la explicación lo dice. Nunca se envía un esfuerzo que el catálogo no declara.
- **Cancelación (030):** Ctrl+C detiene el ejecutor en curso, no lanza fallback, informa «cancelado» y devuelve su código. El tiempo límite no es una cancelación: cuenta como fallo del perfil y pasa al fallback.
- **Catálogo:** combina lo que el ejecutor informa por sí mismo (cuando lo permite) con lo declarado en la configuración. Cada fila indica su origen. Lo que no se pudo comprobar figura como «no verificado».
- **APIs directas extensibles:** añadir un proveedor con API compatible con uno ya soportado se hace por configuración, sin cambiar código.
- **Errores:** mensajes en español, accionables, sin credenciales ni respuestas crudas de proveedores.

## Criterios de aceptación

- AC1 — Dado un catálogo con perfiles autorizados y no autorizados, cuando se ejecuta `gandalf models list --json`, entonces cada perfil muestra ejecutor, modelo, esfuerzos, disponibilidad y origen, y los no autorizados figuran como tales.
- AC2 — Dado un perfil cuyo ejecutor no está instalado o cuya credencial falta, cuando se lista el catálogo, entonces aparece «no disponible» con el motivo.
- AC3 — Dadas las mismas entradas, cuando se ejecuta `gandalf route` dos veces, entonces la salida es idéntica.
- AC4 — Dada una tarea con complejidad alta, cuando se ejecuta `gandalf route`, entonces la explicación nombra el criterio aplicado, el perfil elegido, cada alternativa descartada con su motivo y el fallback previsto.
- AC5 — Dado un ejecutor no autorizado, cuando se ejecuta `route` o `run`, entonces nunca se elige ni se invoca.
- AC6 — Dado un perfil elegido que responde bien, cuando se ejecuta `gandalf run --json`, entonces `status` es `ok`, hay un solo intento y `output` contiene la respuesta normalizada.
- AC7 — Dado un primer perfil que falla y un segundo sano, cuando se ejecuta `gandalf run`, entonces se usa el segundo y `attempts` registra los dos con su resultado.
- AC8 — Dado que todos los perfiles elegibles fallan, cuando se ejecuta `gandalf run`, entonces termina con un código propio distinto de 0–7 y lista cada intento.
- AC9 — Dado que no hay perfiles elegibles, cuando se ejecuta `route` o `run`, entonces no se invoca ningún ejecutor y se devuelve un código propio con los motivos.
- AC10 — Dado `--profile` con un perfil no elegible, cuando se ejecuta `run`, entonces no se ejecuta ningún otro perfil y se explica por qué.
- AC11 — Dada una ejecución en curso, cuando el usuario la cancela (Ctrl+C), entonces el ejecutor se detiene, no se intenta otro perfil y el código de salida indica cancelación.
- AC18 — Dado un primer perfil que supera el tiempo límite y un segundo sano, cuando se ejecuta `gandalf run`, entonces el primero se detiene, se usa el segundo y `attempts` registra el primero con motivo «tiempo límite».
- AC19 — Dada una tarea con riesgo alto y un perfil «no verificado», cuando se ejecuta `gandalf route`, entonces ese perfil figura como descartado con ese motivo.
- AC12 — Dado un perfil sin esfuerzos declarados, cuando se ejecuta con complejidad alta, entonces no se envía esfuerzo y la explicación lo indica.
- AC13 — Dada una clave de API en el entorno, cuando se ejecutan `models list`, `route`, `run` y `doctor` (éxito y error), entonces la clave no aparece en ninguna salida ni archivo.
- AC14 — Dado un archivo de configuración con una credencial, cuando se carga, entonces se rechaza con el código de error de configuración existente.
- AC15 — Dados los ejecutores configurados, cuando se ejecuta `gandalf doctor`, entonces informa por ejecutor instalado/autorizado/autenticación lista sin consumir cuota ni escribir archivos.
- AC16 — Dado un proveedor de API nuevo compatible con uno soportado, cuando se declara solo en la configuración, entonces aparece en el catálogo y puede ejecutarse.
- AC17 — Dados los subcomandos de cli-gandalf, cuando se ejecuta su suite de aceptación, entonces sigue en verde y los códigos 0–7 no cambian.

## Puntos que siguen abiertos

- **suposición tomada** — «OpenCode» en la lista de APIs es la pasarela OpenCode Zen (API propia, compatible con el formato de OpenAI, con listado de modelos), no el harness OpenCode CLI, que queda fuera de alcance. *Base:* búsqueda del 2026-11-18 (opencode.ai/zen); aún no verificado contra la documentación oficial ni en vivo. *Riesgo:* si el propietario se refería al harness, sería un tercer ejecutor y otra tarea.
- **suposición tomada** — APIs directas de esta etapa: OpenAI (modelos GPT/Codex), Gemini y Mistral; el resto («etc.») por configuración (AC16). *Base:* respuesta del propietario en la frontera 2. *Riesgo:* si quiere más proveedores con APIs incompatibles, cada uno añade una tarea.
- **suposición tomada** — La ejecución pide solo una respuesta de texto: ningún ejecutor aplica cambios ni usa herramientas, y nunca se usa auto-approve. *Base:* P5 y la regla global sin auto-approve. *Riesgo:* si el propietario quiere ejecutores que editen código, es otra spec (0.3).
- **suposición tomada** — Las características de la tarea se declaran con flags y tienen valores por defecto; no se infieren del texto. *Base:* selección determinista y explicable (AC3). *Riesgo:* menos automático de lo que el inventario 043–047 describe.
- **suposición tomada** — La regla de selección es la tabla fija de «Características de la tarea» más el orden declarado en la configuración. No hay «tipo de trabajo» en 0.2 porque no cambiaría ninguna elección (YAGNI). *Base:* explicable y comprobable; revisión en frío. *Riesgo:* elecciones poco finas hasta tener evidencia histórica (1.0).
- **suposición tomada** — El catálogo combina descubrimiento (cuando el ejecutor lo ofrece, p. ej. `agy models`) y declaración en la configuración, sin datos inventados. *Base:* contexto maestro sección 8. *Riesgo:* un ejecutor sin descubrimiento depende de lo declarado.
- **decisión diferida** — Streaming, coste, cuotas, rendimiento histórico y modelos locales: etapas 0.4 y 1.0.
- **área no formulable** — Cómo se comportan las condiciones de uso de cada suscripción con la invocación automatizada; se aclara al verificar cada adaptador con documentación oficial.

## Clarifications

- 2026-11-18 — Frontera 1: adaptadores Antigravity + Mistral Vibe; suscripciones y APIs directas; alcance extremo a extremo; la sincronización de skills y patrones entra en la épica #15 como segunda spec.
- 2026-11-18 — Frontera 2: APIs de OpenAI (GPT/Codex), Gemini, Mistral, OpenCode y otras por configuración; dos specs; autopilot completo; PR de docs aparte primero (PR #16).
- 2026-11-18 — Revisión en frío (Issues found, 4): el tiempo límite cuenta como fallo con fallback y solo Ctrl+C cancela (AC11, AC18); características con valores y valores por defecto, sin «tipo de trabajo»; riesgo alto excluye perfiles no verificados (AC19); OpenCode pasa a suposición tomada. Corregido.
- 2026-11-18 — Modo de ejecución: autopilot completo. Esta spec queda **aprobada en autopilot**; las suposiciones quedan en la cola de decisiones como `review-after` (Q1–Q6).
