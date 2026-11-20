---
type: spec
title: Spec — Workflow Runtime 0.3: motor local de workflows declarativos
description: WHAT y WHY de Workflow Runtime 0.3 — definición portable vinculada a objetivo mínimo ODD, preflight, ejecución DAG paralela, ciclos acotados, puertas humanas, checkpoints y gates deterministas.
tags: [sdd, spec, workflow-runtime, odd, gates]
timestamp: 2026-11-20T15:50:00Z
topic: sdd
slug: workflow-runtime-03
status: draft
branch: feat/workflow-runtime-03
issue: https://github.com/CodeMonkReaper/gandalf-wizard/issues/40
---

# Spec — Workflow Runtime 0.3: motor local de workflows declarativos

> Slug: `workflow-runtime-03` · Estado: `draft` · Creada: 2026-11-20
> Hereda: [constitución v1.2.0](../constitution.md) · Épica: [Issue #40](https://github.com/CodeMonkReaper/gandalf-wizard/issues/40)

## Problema y por qué importa

Hoy `gandalf` ejecuta tareas individuales de solo texto (`run`), consulta memoria en ROMERO (`memory query`) y sincroniza skills/reglas (`sync`), pero no puede encadenar varios pasos con dependencias ni verificar si el conjunto cumplió un objetivo. Cuando un trabajo necesita varios pasos, ramas paralelas, revisión intermedia o corrección ante un fallo de calidad, el operador debe coordinarlo a mano sin estado recuperable ni evidencia unificada.

## Coste de no construirlo

- Todo flujo multi-paso depende de coordinación manual o queda atado a un único harness externo.
- Un fallo a mitad de una secuencia (cuota, red, interrupción o salida inválida) obliga a reconstruir entradas y repetir pasos ya hechos.
- Quedan bloqueadas las fases posteriores de [GW-SPEC-001](../../../../Gandalf_Wizard_v1.1.0/docs/GANDALF_WIZARD_ESPECIFICACION_MAESTRA.md#L1464-L1478): compilación multi-harness (`AC-04`), dominios especializados (`AC-08`–`AC-10`) y control de runtime en Control Center (`AC-19`, `AC-22`).

## La alternativa más barata

Encadenar invocaciones de `gandalf memory query` y `gandalf run --json` en scripts externos con archivos temporales. Sirve para secuencias lineales cortas, pero no valida dependencias ni ciclos antes de gastar cuota, no ejecuta ramas paralelas con estado consistente, no detiene la ejecución de forma limpia para aprobación humana, no reanuda desde checkpoints tras un fallo y no ofrece gates deterministas ni veredicto trazable contra un objetivo.

## Objetivos

- **G1 — Definición portable vinculada a objetivo (`GW-05`, `GW-14`):** aceptar una definición declarativa de workflow independiente del harness, vinculada a un objetivo versionado con restricciones, entregables esperados y criterios de éxito.
- **G2 — Preflight sin gasto de cuota:** validar en solo lectura la integridad del grafo, dependencias, límites de ciclos, referencias entre pasos y elegibilidad del router antes de iniciar la ejecución.
- **G3 — Ejecución gobernada y paralela (`AC-03`):** ejecutar un corte vertical seguro sobre las capacidades ya verificadas (Router 0.2 solo texto, consulta ROMERO y validadores locales), corriendo ramas independientes en paralelo y aislando los artefactos por corrida sin modificar archivos del proyecto.
- **G4 — Gates deterministas y ciclos acotados de corrección (`GW-44`):** comprobar cada salida con criterios verificables antes de aceptarla y permitir ciclos de corrección limitados por un máximo explícito de iteraciones y condición de salida.
- **G5 — Aprobación humana explícita y recuperación por checkpoints (`GW-19`, `GW-40`):** pausar limpiamente en puertas de aprobación (`awaiting_approval`), permitir aprobar/rechazar, cancelar o reanudar desde checkpoints persistidos sin perder trazabilidad ni repetir pasos ya completados.
- **G6 — Cierre auditable contra el objetivo (`AC-01`, `AC-03`):** producir un informe final con trazabilidad `objetivo → paso → artefacto → evidencia → evaluación` y redacción estricta de secretos.

## Fuera de alcance

- Pasos que editen archivos del repositorio, ejecuten comandos arbitrarios de shell o invoquen herramientas externas de escritura (se mantiene el límite de solo texto de [router-02](router-02.md)).
- Compilación y ejecución multi-harness de la misma representación intermedia en harnesses externos (`Fase 04`, `AC-04`).
- Generación automática de workflows desde texto libre (Composer automático) y descomposición o refinamiento interactivo completo de objetivos (`GW-05.2`).
- Subworkflows anidados, ejecución distribuida en varios nodos y procesos residentes en segundo plano.
- Presupuestos concurrentes, medición de costes o cuotas reales del router (`Fase 05` ampliación) y optimización basada en benchmarks (`GW-46`, `GW-50`).
- Evaluadores basados en modelos (`LLM-as-a-Judge`) como gate bloqueante único; en 0.3 los gates bloqueantes son deterministas y comprobables.
- Escritura automática de conclusiones en ROMERO y panel visual (`GW-57`).
- Aprobación implícita o automática de puertas humanas y ampliación automática de permisos o límites en tiempo de ejecución.
- Cualquier cambio que rompa los contratos existentes de `version`, `config`, `doctor`, `memory`, `models`, `route`, `run`, `catalog` y `sync`.

## Usuarios y contexto

- **El propietario (operador en CLI local de Windows):** valida workflows dentro de un proyecto configurado con `gandalf`, lanza ejecuciones, inspecciona estado y artefactos, aprueba o rechaza puertas humanas, y reanuda o cancela corridas.
- **Agentes de IA y automatizaciones locales:** invocan los comandos en formato estructurado (`--json`) para validar, ejecutar y consultar estado, respetando las puertas que exigen decisión explícita del operador.

## Qué podrá hacer el usuario

| Comando | Qué hace | Condición previa |
|---|---|---|
| `gandalf workflow validate [--json] <definición>` | Preflight en solo lectura: valida el objetivo mínimo (`GW-05`), el grafo de dependencias, los límites de ciclos de corrección, las referencias entre artefactos y la elegibilidad local de ejecutores. Nunca consume cuota ni crea corridas. | Archivo de definición legible. |
| `gandalf workflow run [--json] <definición>` | Ejecuta preflight y, si es válido, crea una corrida con `run-id`, corre pasos respetando dependencias y paralelismo, aplica gates/ciclos y conserva checkpoints en cada transición. Se detiene al cerrar contra el objetivo, al llegar a una puerta humana, al cancelar o ante fallo bloqueante. | Preflight válido; ejecutores y proveedor ROMERO requeridos autorizados y disponibles. |
| `gandalf workflow status [--json] <run-id>` | Consulta de solo lectura del estado de la corrida: estado global, objetivo vinculado, pasos, iteraciones, artefactos, gates, aprobaciones pendientes y veredicto final. | `run-id` existente en el proyecto. |
| `gandalf workflow approve [--json] <run-id> --gate <gate-id> --decision <approve\|reject> [--reason <texto>]` | Registra la decisión auditable del operador sobre una puerta pendiente (`awaiting_approval`), ligada al `run-id`, a la puerta y a la huella de la definición y artefactos revisados. No lanza pasos por sí solo. | Corrida en `awaiting_approval` con esa puerta pendiente. |
| `gandalf workflow resume [--json] <run-id>` | Reanuda una corrida desde su último checkpoint válido conservando el mismo `run-id` y los pasos ya completados (no repite pasos `passed`). | Corrida en estado reanudable (`paused`, `failed` por fallo recuperable de paso o `awaiting_approval` ya aprobada) y definición sin alteraciones. |
| `gandalf workflow cancel [--json] <run-id> [--reason <texto>]` | Cancela de forma terminal (`cancelled`) una corrida no terminal preservando los artefactos y la evidencia de los pasos ya concluidos. Sobre una corrida en `running` actúa como solicitud cooperativa de cancelación. | Corrida en estado no terminal. |

**Tipos de pasos cubiertos en 0.3:**

- **`memory_query`:** consulta de solo lectura en ROMERO dentro del ámbito del proyecto; produce un artefacto estructurado con resultados y limitaciones.
- **`router_task`:** inferencia de solo texto a través de Router 0.2 (con complejidad/riesgo o perfil explícito y fallback autorizado) parametrizada con salidas de pasos previos.
- **`quality_gate`:** evaluación determinista local sobre artefactos de la corrida (existencia no vacía, estructura válida y campos requeridos, patrones obligatorios o prohibidos, límites de tamaño y detección de marcadores de secretos).
- **`human_approval`:** punto de parada obligatorio (`awaiting_approval`) que expone el resumen y los artefactos a revisar antes de continuar.

**Vínculo mínimo ODD (`GW-05`) y ciclos acotados (`AC-03`):**

- Toda definición declara un bloque de objetivo (identificador, versión, enunciado, restricciones, entregables esperados vinculados a salidas de pasos y criterios de éxito deterministas evaluados al cierre).
- Un paso `router_task` vinculado a un `quality_gate` puede declarar un ciclo de corrección con **máximo explícito de iteraciones** (`max_iterations`) y condición de salida (`pass` del gate); cada intento posterior recibe el diagnóstico del gate fallido y conserva el registro de intentos previos.

## Qué resultado obtiene

- Todos los comandos emiten salida legible en terminal y su equivalente estructurado con `--json` (versión de esquema estable), siempre sin exponer credenciales ni secretos.
- **Estados de corrida:**
  - *No terminales / reanudables:* `created`, `validated`, `running`, `awaiting_approval`, `paused` (interrupción operativa como `Ctrl+C` o caída controlada con checkpoint válido) y `failed` (cuando un paso recuperable falla por error transitorio de ejecutor/ROMERO o agotamiento de iteraciones, permitiendo reintentar lo pendiente mediante `resume`).
  - *Terminales (no reanudables):* `succeeded`, `partial`, `blocked` (rechazo humano o violación no recuperable de política o gate en `blocked`) y `cancelled` (cancelación explícita mediante `workflow cancel`).
- **Estados de paso:** `pending`, `running`, `awaiting_approval`, `passed`, `failed`, `skipped`, `cancelled`.
- **Estados de gate (`GW-44`):**
  - `pass`: criterio determinista satisfecho o puerta humana aprobada.
  - `fail`: criterio determinista incumplido en `quality_gate` (elegible para ciclo de corrección mientras queden iteraciones dentro de `max_iterations`).
  - `blocked`: condición dura no recuperable (gate determinista sin entrada válida o con detección de secreto, o puerta humana rechazada); detiene el paso sin reintentos y bloquea el flujo sin éxito.
  - `needs_human_review`: estado pendiente emitido exclusivamente por un paso `human_approval` mientras aguarda decisión en `workflow approve`.

### Referencia para aprobar

Un workflow `analisis-propuesta@1.0.0` vinculado al objetivo `OBJ-PROP-001@1.0.0` (*entregables:* `riesgos`, `recomendacion`; *criterios:* ambos entregables presentes, no vacíos y con formato válido):

1. `gandalf workflow validate` devuelve estado válido, muestra el plan del grafo (ramas paralelas `contexto_romero` y `borrador_riesgos` → unión `sintesis` con ciclo `max_iterations: 2` y gate determinista → puerta humana `aprobacion_revision` → cierre contra objetivo) y realiza **0 invocaciones** a ejecutores.
2. `gandalf workflow run` ejecuta `contexto_romero` y `borrador_riesgos` en paralelo. En `sintesis`, el intento 1 falla el gate determinista por omitir una sección obligatoria; el ciclo de corrección ejecuta el intento 2 incorporando el diagnóstico y supera el gate (`pass`). Al llegar a `aprobacion_revision`, la corrida se detiene en `awaiting_approval` e informa `run-id`, `gate-id` y los artefactos generados.
3. `gandalf workflow status <run-id>` muestra los pasos `passed`, las 2 iteraciones de `sintesis`, los artefactos registrados y la puerta pendiente.
4. El operador ejecuta `gandalf workflow approve <run-id> --gate aprobacion_revision --decision approve --reason "Riesgos verificados"` y después `gandalf workflow resume <run-id>`.
5. `resume` **no reejecuta** `contexto_romero`, `borrador_riesgos` ni `sintesis`; continúa tras la puerta aprobada, evalúa `OBJ-PROP-001@1.0.0` sobre los entregables finales y cierra en `succeeded` con trazabilidad completa `objetivo → pasos → artefactos → gates → aprobación → evaluación`.

## Qué debe respetar

- **Límite seguro sin escritura en el proyecto:** ningún paso modifica archivos del repositorio del usuario ni ejecuta comandos arbitrarios de shell; los artefactos y checkpoints quedan confinados al espacio de corridas del proyecto.
- **Cero fugas de secretos ([constitución v1.2.0, P5](../constitution.md#L31)):** todas las salidas CLI/JSON, checkpoints, artefactos y diagnósticos redactan credenciales declaradas y nunca vuelcan tokens ni claves.
- **Integridad de corridas y aprobaciones (`GW-19.4`, `GW-40`):** una aprobación aplica solo a su `run-id`, `gate-id` y huella de definición/artefactos; modificar la definición después de iniciar impide reanudar esa corrida o reutilizar su aprobación.
- **Gates obligatorios no eludibles (`GW-14`, `GW-44`):** ningún paso dependiente de un paso fallido o cancelado se inicia; un gate obligatorio fallido o un criterio incumplido del objetivo impide declarar `succeeded`.
- **Concurrencia segura por corrida:** si dos procesos intentan avanzar o decidir simultáneamente la misma corrida (`run`, `resume`, `approve`), solo uno progresa y el otro recibe un conflicto explícito sin corromper los checkpoints; `workflow cancel` sobre una corrida `running` actúa como solicitud cooperativa para detenerla limpiamente sin corromper su estado.
- **Compatibilidad total con lo ya entregado:** se preservan sin cambios los contratos y códigos de salida de [cli-gandalf](cli-gandalf.md), [consulta-memoria-romero](consulta-memoria-romero.md), [router-02](router-02.md) y [sync-skills-patrones](sync-skills-patrones.md).

## Cómo lo demuestra

- La verificación entra por la interfaz real de la CLI de `gandalf` con ejecutores y proveedor ROMERO controlados (sin consumir cuota ni tocar archivos del proyecto), más una prueba de humo controlada si el propietario la autoriza en `verify`.
- Se ejecutan el escenario de referencia (`analisis-propuesta@1.0.0`) y casos dedicados de fallo, concurrencia y seguridad:
  1. `validate` en solo lectura comprobando 0 invocaciones a ejecutores y árbol de archivos intacto.
  2. Ramas paralelas comprobando solapamiento real de ejecución en el registro de eventos.
  3. Ciclo de corrección: recuperación en el intento 2 tras fallo de gate en el intento 1, y parada exacta al agotar `max_iterations`.
  4. Puerta humana: parada en `awaiting_approval`, continuación tras `approve` + `resume` sin repetir pasos `passed`, y bloqueo tras `reject`.
  5. Interrupción controlada (`Ctrl+C`) dejando estado `paused`, fallo recuperable de paso dejando estado `failed`, y reanudación con `resume` conservando `run-id`, más rechazo si se altera la definición original.
  6. Cancelación activa mediante `workflow cancel` (tanto en `running` como en `awaiting_approval` o `paused`) cerrando en estado terminal `cancelled` mientras preserva checkpoints y evidencia parcial.
  7. Concurrencia sobre la misma corrida (dos operaciones simultáneas: una progresa y la otra recibe conflicto sin corromper el estado).
  8. Evaluación final contra el objetivo (`GW-05`): `succeeded` cuando todo cumple y `partial` o `failed` cuando falta un entregable o criterio del objetivo.
  9. Prueba con centinela de secreto verificando ausencia total del valor en stdout, stderr, `--json`, checkpoints y artefactos.
  10. Regresión completa de `version`, `config`, `doctor`, `memory`, `models`, `route`, `run`, `catalog`, `sync` y del gate de seguridad del repositorio.
- La evidencia queda registrada en `02-DOCS/wiki/sdd/verifications/` con comandos ejecutados, informes de corrida, trazas de checkpoints y veredictos de paso y bloqueo.

## Comportamiento

- **Camino principal:** `validate` verifica en solo lectura; `run` lanza las ramas listas en paralelo con checkpoint tras cada transición, evalúa gates y ciclos acotados, y se detiene en las puertas humanas; `approve` registra la decisión auditada; `resume` retoma desde el último checkpoint sin repetir pasos superados y emite la evaluación final del objetivo.
- **Casos borde:**
  - *Workflow sin puertas humanas ni ramas paralelas:* completa en una sola llamada a `run`.
  - *Fallo en una rama paralela:* no se inician los dependientes de esa rama; las ramas independientes en curso concluyen o se cancelan de forma controlada y la corrida refleja el resultado real (`failed` recuperable o `partial` terminal si la rama era explícitamente opcional).
  - *Puerta `human_approval` con ramas paralelas:* el comportamiento exacto de drenado de ramas activas al pausar en `awaiting_approval` y de reanudación cuando coexisten múltiples puertas humanas independientes queda abierto para resolverse en `clarify`.
  - *Agotamiento de ciclo de corrección:* al alcanzar `max_iterations` sin `pass`, el paso termina en `failed` conservando el historial de todos los intentos.
  - *Interrupción (`Ctrl+C` o caída controlada):* mantiene el último checkpoint de los pasos ya concluidos y marca la corrida como `paused` (no terminal); `resume` retoma únicamente lo pendiente sin repetir pasos `passed`.
  - *Cancelación explícita (`workflow cancel`):* sobre una corrida `running` emite solicitud cooperativa para no iniciar nuevos pasos, cerrar limpiamente los activos y terminar en `cancelled` (terminal); sobre `paused` o `awaiting_approval` cierra directamente en `cancelled`.
  - *Reanudación o aprobación repetida:* reanudar una corrida ya terminal (`succeeded`, `partial`, `blocked`, `cancelled`) no ejecuta trabajo y devuelve el estado final; volver a decidir una puerta ya cerrada se rechaza con diagnóstico claro.
- **Rutas de error (y qué no se revela):**
  - *Definición inválida* (ciclo no declarado, dependencia o artefacto inexistente, ciclo de corrección sin `max_iterations`, objetivo incompleto): bloquea antes de crear corrida y lista cada fallo con su identificador.
  - *Ejecutor no disponible o no autorizado, o ROMERO inaccesible:* fallo explícito redactado, sin revelar credenciales ni cuerpos de error crudos con secretos.
  - *Rechazo humano (`--decision reject`):* registra motivo y autor, cierra la corrida como `blocked` y no ejecuta ningún paso dependiente.
  - *`run-id` inexistente, checkpoint alterado o definición modificada:* bloquea con diagnóstico explícito sin sobrescribir evidencia previa.

## Criterios de aceptación

1. **AC-01 · Preflight en solo lectura y rechazo de definiciones inválidas.**
   - *Given* una definición válida con bloque de objetivo mínimo, *When* se ejecuta `gandalf workflow validate [--json]`, *Then* devuelve estado válido, describe el grafo y el objetivo, realiza 0 llamadas a ejecutores/ROMERO y no crea archivos.
   - *Given* una definición con ciclo no declarado, dependencia o artefacto inexistente, ciclo de corrección sin `max_iterations` u objetivo incompleto, *When* se ejecuta `validate` o `run`, *Then* bloquea antes de crear corrida y lista cada error con su identificador.
2. **AC-02 · Paralelismo real y respeto de dependencias.**
   - *Given* un workflow con dos ramas iniciales independientes y un paso posterior que depende de ambas, *When* se ejecuta `run`, *Then* las dos ramas se ejecutan de forma concurrente (solapamiento observable) y el paso dependiente solo comienza cuando ambas están en `passed`.
3. **AC-03 · Bloqueo de dependientes y gates obligatorios no eludibles.**
   - *Given* una rama obligatoria que falla por error de ejecutor o por `quality_gate` en `fail` o `blocked`, *When* avanza la corrida, *Then* ningún paso dependiente se inicia (quedan en `skipped`), se conserva la evidencia ya generada y la corrida nunca termina en `succeeded`.
4. **AC-04 · Ciclo acotado de corrección (recuperación y agotamiento).**
   - *Given* un paso `router_task` con `quality_gate` y `max_iterations: 2` que falla el gate en el intento 1 y lo supera en el intento 2 usando el diagnóstico, *When* se ejecuta, *Then* termina en `passed` en la iteración 2 conservando ambos intentos.
   - *Given* el mismo paso cuando todas las iteraciones hasta `max_iterations` fallan el gate, *When* se ejecuta, *Then* se detiene al alcanzar el límite sin bucle infinito y queda en `failed`.
5. **AC-05 · Puerta humana (pausa, aprobación y rechazo).**
   - *Given* un workflow con `human_approval`, *When* `run` alcanza esa puerta, *Then* pausa en `awaiting_approval` sin ejecutar dependientes e informa `run-id`, `gate-id` y artefactos.
   - *Given* esa corrida en `awaiting_approval`, *When* se ejecuta `approve --decision approve` y luego `resume`, *Then* no repite los pasos `passed` previos y continúa hasta finalizar.
   - *Given* esa corrida en `awaiting_approval`, *When* se ejecuta `approve --decision reject --reason <motivo>`, *Then* registra motivo y fecha, cierra en `blocked`, no ejecuta dependientes y bloquea cualquier `resume` posterior.
6. **AC-06 · Integridad de definición y alcance estricto de aprobaciones.**
   - *Given* una corrida en `awaiting_approval` o pausada, *When* cambia la definición original o se intenta usar una aprobación de otra corrida o puerta, *Then* `approve` o `resume` rechazan la operación con diagnóstico claro y sin ejecutar pasos.
7. **AC-07 · Checkpoints, interrupción, fallo recuperable y reanudación idempotente.**
   - *Given* una corrida interrumpida (`paused` tras `Ctrl+C` o caída controlada) o en `failed` recuperable tras completar al menos un paso, *When* se consulta `status` y se ejecuta `resume`, *Then* `status` muestra el último checkpoint mantenido y `resume` conserva el mismo `run-id`, reutiliza artefactos previos sin repetir pasos `passed` y completa los pendientes.
   - *Given* una corrida ya en estado terminal (`succeeded`, `partial`, `blocked`, `cancelled`), *When* se ejecuta `resume`, *Then* no ejecuta pasos y devuelve el estado terminal con diagnóstico.
8. **AC-08 · Cancelación cooperativa y exclusión mutua por corrida.**
   - *Given* una corrida en `running`, `paused` o `awaiting_approval`, *When* se invoca `workflow cancel`, *Then* detiene la corrida de forma controlada sin iniciar nuevos pasos, conserva la evidencia previa y cierra en el estado terminal `cancelled`.
   - *Given* dos intentos simultáneos de avanzar o decidir la misma corrida (`run`, `resume` o `approve`), *When* compiten, *Then* solo uno progresa y el otro recibe conflicto explícito sin corromper checkpoints.
9. **AC-09 · Evaluación contra objetivo (`GW-05`) y trazabilidad.**
   - *Given* el escenario de referencia `analisis-propuesta@1.0.0`, *When* se satisfacen todos los entregables y criterios de `OBJ-PROP-001@1.0.0`, *Then* cierra en `succeeded` con trazabilidad `objetivo → pasos → artefactos → gates → aprobación → evaluación`.
   - *Given* una corrida donde todos los pasos completan sin fallo de transporte pero falta un entregable o criterio del objetivo, *When* cierra, *Then* termina en `partial` o `failed` (nunca `succeeded`) indicando qué criterio falló.
10. **AC-10 · Aislamiento, redacción de secretos y no regresión.**
    - *Given* un centinela de secreto en configuración, entradas o salidas, *When* se inspeccionan stdout, stderr, `--json`, checkpoints y artefactos, *Then* el centinela no aparece en ningún punto y los archivos del proyecto permanecen intactos.
    - *Given* la suite existente de `gandalf`, *When* se ejecutan las pruebas previas junto a las de workflow, *Then* pasan sin regresiones.

## Puntos a clarificar

- **pregunta abierta** — ¿Qué conjunto exacto de reglas deterministas soporta `quality_gate` en 0.3 (p. ej. no vacío, estructura válida + campos requeridos, patrón obligatorio o prohibido, longitud mínima/máxima, detección de secretos) y si un gate puede declararse informativo (no bloqueante)?
- **pregunta abierta** — ¿Cuál es el criterio exacto que separa un cierre `partial` de un cierre `failed` (p. ej. solo cuando falla una rama declarada opcional y el objetivo principal se cumple, o también cuando se cumple parte de los criterios del objetivo)?
- **pregunta abierta** — En CLI local, ¿basta con que `workflow approve` exija el `run-id`, `gate-id` y motivo auditable, o debe exigir también una confirmación explícita vinculada a la huella del checkpoint para evitar aprobaciones accidentales?
- **pregunta abierta** — Cuando una rama paralela alcanza una puerta `human_approval` mientras otra rama independiente sigue ejecutándose (o alcanza otra puerta `human_approval`), ¿debe `run` esperar a que las ramas activas completen sus pasos listos antes de pausar en `awaiting_approval`, y puede `resume` continuar la rama ya aprobada aunque otra puerta independiente siga pendiente?
- **suposición tomada** — La interrupción (`Ctrl+C` o caída controlada) deja la corrida en `paused` (reanudable con `resume`), mientras `workflow cancel` la cierra en `cancelled` (terminal); asimismo, `failed` por fallo recuperable de paso admite `resume`, en tanto `succeeded`, `partial`, `blocked` y `cancelled` son terminales. *Base:* `GW-40` y necesidad de recuperar interrupciones operativas sin violar la terminalidad de `cancelled`. *Riesgo:* si el operador prefiere que `Ctrl+C` cierre como `cancelled` terminal o que `failed` no sea reanudable, cambiará la tabla de transiciones en `clarify`.
- **suposición tomada** — `workflow cancel` sobre una corrida en `running` actúa como solicitud cooperativa hacia el proceso ejecutor activo, mientras la exclusión mutua rechaza concurrencia entre `run`, `resume` y `approve`. *Base:* ausencia de un demonio residente en segundo plano junto al requisito `AC-08` de cancelar corridas activas sin corromper checkpoints. *Riesgo:* si se prefiere rechazar `cancel` mientras viva el proceso de `run`/`resume`, `cancel` quedaría restringido a `paused` y `awaiting_approval`.
- **suposición tomada** — `approve` solo registra la decisión auditada y no ejecuta pasos; `resume` ejecuta la continuación. *Base:* responsabilidad única, auditoría limpia e idempotencia del Enfoque A. *Riesgo:* si el operador prefiere reanudar en el mismo comando mediante una opción explícita, cambia el contrato de `approve`.
- **suposición tomada** — Todo ciclo de corrección exige `max_iterations` explícito en la definición y el paralelismo se acota con un límite conservador configurable. *Base:* `GW-14` y `GW-40`. *Riesgo:* si se pide un valor por defecto implícito para ciclos, cambia el comportamiento de `validate`.
- **suposición tomada** — Todos los comandos de workflow mantienen `--json` con esquema versionado estable y asignan códigos de salida sin colisionar con los ya reservados por Foundation, Router y Sync. *Base:* decisiones previas de `cli-gandalf`, `router-02` y `sync-skills-patrones`. *Riesgo:* si se requieren códigos dedicados para `awaiting_approval` o fallo de gate, se fijarán en `clarify`/`plan`.
- **decisión diferida** — Formato físico de la definición, mecanismo de persistencia local de checkpoints y artefactos, control de exclusión mutua en Windows y diseño modular del código (corresponden a `plan`).
- **área no formulable** — Compatibilidad exacta del modelo de eventos y checkpoints con el control de runtime en tiempo real y los puntos seguros de `GW-57` (`AC-22`), pendiente de los contratos físicos de `OPEN-13`.

Referencias de producto para esta entrega: [GW-SPEC-001 v1.1.0-draft, GW-05, GW-14, GW-15, GW-18, GW-19, GW-40, GW-42, GW-44, anexos A.5/A.6 y AC-01 / AC-03](../../../../Gandalf_Wizard_v1.1.0/docs/GANDALF_WIZARD_ESPECIFICACION_MAESTRA.md) · [Constitución v1.2.0](../constitution.md) · [Issue #40](https://github.com/CodeMonkReaper/gandalf-wizard/issues/40).
