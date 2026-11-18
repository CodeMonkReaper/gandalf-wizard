---
type: plan
title: Plan — Router 0.2
description: Contratos, diseño, pruebas y tareas del router de texto de la épica 15.
tags: [sdd, plan, router]
timestamp: 2026-11-18T12:36:00Z
topic: sdd
slug: router-02
branch: feat/router-02
status: proposed
---

# Plan — Router 0.2

Especificación [router-02](../specs/router-02.md), [constitución v1.2.0](../constitution.md) y [épica #15](https://github.com/CodeMonkReaper/gandalf-wizard/issues/15). Este plan cubre la primera de las dos especificaciones de la épica.

## 0. Restricciones globales

- Go `go1.27.3`, módulos Go y CLI modular. Controles `gofmt -l .` con salida vacía, `go vet ./...` y `go test ./...` (TDD estricto).
- «La configuración en 5 capas, los subcomandos existentes, la salida tabla + `--json` con `schema_version: "1"` y los códigos de salida 0–7 siguen igual».
- «Nada se ejecuta sin autorización explícita en la configuración». «Ninguna salida, error o log muestra una credencial».
- «Antigravity y Vibe usan su propia sesión iniciada. Gandalf no lee ni copia sus credenciales». «`doctor` sigue siendo solo lectura».
- Complejidad `baja|media|alta`, por defecto `media`, pide `low|medium|high`. Riesgo `bajo|medio|alto`, por defecto `medio`. Riesgo alto excluye perfiles no verificados.
- Las ejecuciones reales de aceptación con proveedores requieren autorización humana. El PR y la integración siguen los principios 5 y 7.

## 1. Contexto y límites

AC1–AC19 amplían la CLI de Foundation 0.1. Reutilizar `config.Load`, la inyección de dependencias de `cli.Run`, el diagnóstico de ROMERO existente y los renderizadores de tabla y JSON. La sincronización se trabaja después en `sync-skills-patrones`. La sección «Fuera de alcance» de la spec rige todas las tareas, en especial ejecución solo de texto, sin herramientas, sin streaming al usuario y sin persistencia de resultados.

## 2. Arquitectura

```text
[CLI y doctor] → [configuración efectiva]
      ↓                   ↓
[catálogo y política] → [ejecución secuencial]
                              ↓
                 [puerto común de ejecutores]
                   ↓          ↓         ↓
                 [agy]      [vibe]   [APIs HTTP]  externos
```

- **Configuración y CLI** (internos). Validan la política local, enlazan órdenes y renderizan contratos públicos. El proceso principal propaga Ctrl+C mediante un contexto cancelable.
- **Catálogo y selector** (internos). Componen un único snapshot por invocación y calculan un plan puro y determinista. El orden explícito del propietario tiene precedencia sobre la coincidencia de esfuerzo.
- **Orquestador** (interno). Ejecuta el plan de a un perfil, bajo plazo por intento, y registra resultados normalizados.
- **Adaptadores** (internos). Traducen el puerto común a los procesos CLI y a HTTP. Separar sondeo local, descubrimiento facultativo e inferencia permite garantizar que `doctor` y `route` funcionen sin inferencia ni llamadas nuevas a proveedores.

## 3. Interfaces y contratos

Firmas de diseño. Los nombres de paquetes y archivos del desglose son previsiones para clasificar el riesgo.

```text
config.Load(LoadInput) -> EffectiveConfig{..., Router: Policy} | ConfigError
Policy = {executors: Executor[], profiles: Profile[], attempt_timeout: Duration}
Executor = {id, kind, authorized, protocol?, base_url?, credential_env?}
Profile = {id, executor_id, model, efforts: map[Effort]NativeValue}
Features = {complexity, risk}
Adapter.Probe(ctx, Executor) -> Probe{installed, auth: ready|missing|unknown,
                                      text_only: enforced|unproven, reason}
Adapter.Discover(ctx, Executor) -> Candidate[]{model, efforts} | unavailable
Adapter.Execute(ctx, Execution) -> Text | SafeFailure
Execution = {executor: Executor, profile: Profile, effort: Effort?, task: Text}
catalog.Build(ctx, Policy, Registry, DiscoveryMode) -> Catalog
DiscoveryMode = local-only | local-and-documented-free
Catalog = {profiles: ProfileView[], executors: ProbeView[]}
ProfileView = {profile: Profile, authorized, availability, reason, origin, verification}
router.Select(Catalog, Features, manualID?) -> Decision
Decision = {ordered: Choice[], considered: Assessment[], manual: Bool}
Choice = {profile: Profile, effort: Effort?, explanation}
Assessment = {profile_id, eligible, reason}
router.Run(ctx, Policy, Decision, task: Text, Registry) -> Result
Result = {status, profile, effort, attempts: Attempt[], explanation, output}
Attempt = {profile, effort, outcome, reason}
```

- **Configuración.** Bloque opcional `router`, con lista ordenada de perfiles. Para este bloque, el proyecto puede reemplazar la lista de perfiles y ajustar el plazo. La capa global del usuario fija ejecutores autorizados y, para cada API, el vínculo entre protocolo, origen HTTPS y nombre de variable de entorno. Un proyecto no concede autorización ni sustituye destinos o referencias a secretos. El default es catálogo vacío, inferencia denegada y plazo de 60 s por intento. No hay override de credenciales por flag. Validar identificadores, duplicados, referencias, plazo positivo, URLs sin credenciales y campos de credencial en ambas capas. Usar código 3 para configuración inválida. No trasladar valores secretos al catálogo ni a `config show`.
- **Catálogo.** `Probe` es local, sin cuota, cambios de disco, lectura de tokens nativos ni refresh. Presencia de binario y autenticación son estados separados. Si la CLI carece de un indicador de sesión local documentado, reportar autenticación desconocida. Credencial API presente significa configurada, con validez remota aún sin comprobar. Usar descubrimiento documentado y de solo lectura cuando esté disponible, sin asumir que listar modelos demuestra autorización, autenticación o soporte de todos los esfuerzos. `models list` puede usar descubrimiento comprobado sin cuota y solo para ejecutores autorizados. `route` y `run` no lanzan procesos de ejecutores no autorizados. `route`, `run` antes de inferencia y `doctor` usan descubrimiento local únicamente. Fusionar por ejecutor y modelo, conservar el orden de perfiles declarados y ordenar los candidatos sin perfil explícito por identificador. Estos candidatos se muestran como inventario y el selector explica que carecen de posición declarada. Un fallo de descubrimiento conserva declaraciones con evidencia desconocida. La evidencia de disponibilidad y de verificación se conserva por separado. Distinguir «no autorizado», «no disponible», «no verificado» y «disponible», con su razón y origen.
- **Selección.** Eliminar perfiles sin autorización o sin capacidad de ejecutar, y perfiles no verificados si el riesgo es alto. Un modelo con disponibilidad sin verificar puede intentarse con riesgo bajo o medio si el ejecutor tiene autenticación lista según su sondeo local y el modo solo texto está demostrado. Autenticación ausente o desconocida y modo solo texto no demostrado descartan el perfil para todo riesgo. Cuando el esfuerzo pedido no está declarado, omitirlo y explicar el motivo. Generar motivos por candidato, el elegido y el siguiente elegible. `--profile` limita el plan al perfil exacto. Los motivos de todos los candidatos conservan el orden del catálogo.
- **Ejecución.** Resolver el ejecutor por ID y verificar su autorización vigente y destino antes de abrir cada proceso o petición. Cancelación del contexto padre corta la ejecución y la cadena de fallback. Un timeout del hijo detiene ese intento, registra «tiempo límite» y habilita el siguiente. Una indisponibilidad sobrevenida o error del ejecutor cuenta como intento fallido. Cada perfil se usa una vez, y solo se incluye la salida de texto del éxito. Los errores de proveedor, stderr y cuerpos crudos quedan fuera de las salidas. Redactar los valores de credenciales conocidas también si el proveedor los devuelve dentro de un texto exitoso.
- **Adaptadores CLI.** Proceso directo con argumentos separados y directorio aislado. Limitar entorno heredado, aislar logs/sesión transitoria y limpiar recursos al terminar. El proceso nunca recibe claves de otras APIs ni de ROMERO. Probar en Windows que el control efectivo impide herramientas integradas, plugins y MCP aunque el perfil del usuario los tenga activos. `agy --settings` ofrece ajustes de sesión para investigar. En Vibe investigar el filtro de herramientas y el aislamiento de configuración y sesión. Un prompt que pide solo texto es insuficiente como control. Si la versión no admite un control comprobable, marcar el ejecutor no disponible con el motivo. Detectar salidas inválidas y finalizar todo el proceso gestionado cuando se cancele o caduque el intento. Evidencia de referencia en [Antigravity headless](https://antigravity.google/docs/cli/headless/), [permisos](https://antigravity.google/docs/permissions?tab=cli) y [Vibe CLI](https://docs.mistral.ai/vibe/code/cli/work-with-cli).
- **Adaptadores HTTP.** Usar solicitudes de texto sin declaración de herramientas. Implementar Responses de OpenAI, Chat Completions compatible para Mistral y extensiones, y GenerateContent de Gemini. Traducir únicamente esfuerzos explícitos cuyo significado esté declarado para el modelo. Extraer solo texto final y descartar chunks de razonamiento. Sin redirects ni reintentos HTTP automáticos de generación. Solo enviar la variable de entorno al origen HTTPS autorizado por la configuración global. No asumir compatibilidad universal de OpenCode Console. Un modelo con protocolo compatible entra por configuración y su catálogo declarado, con prueba local y evidencia viva separada. [OpenAI](https://developers.openai.com/api/docs/guides/reasoning) · [Mistral](https://docs.mistral.ai/api/endpoint/chat) · [Gemini](https://ai.google.dev/api/generate-content) · [OpenCode](https://opencode.ai/docs/console/models/).
- **CLI y salida.** Reusar `cli.Run(ctx, args, deps) -> int`. JSON de éxito y error esperado conserva `schema_version: "1"`. `models list` añade `profiles` con los campos de `ProfileView` a su envoltorio. `route` publica elegido, esfuerzo, criterios, alternativas y fallback. `run` usa el envoltorio de la spec con `command: "run"`, `status`, `profile`, `effort`, `attempts`, `explanation`, `output`. Convención nueva propuesta: 8 sin perfil elegible, 9 candidatos agotados, 10 cancelado. Un `doctor` sin ejecutores conserva el resultado anterior. Con ejecutores añade `executors` con diagnóstico local. Un estado desconocido se informa como tal. Los códigos anteriores conservan su significado y mapeo.

## 4. Datos y flujo

La política vive en las configuraciones actuales. Las claves API solo existen en el entorno y en la petición al origen autorizado. El catálogo, la decisión ordenada y los intentos viven en memoria una invocación. Las pruebas usan IDs ficticios. La aplicación no persiste historial ni resultados.

Ejemplo de ruta automática: la CLI carga la política y forma el catálogo local. El selector conserva A y B en el orden declarado, asigna `high` a A y omite esfuerzo en B. El intento A caduca y termina antes de empezar B. B devuelve texto y la salida informa ambos intentos. Si el contexto padre se cancela durante A, B no arranca y el código es 10.

## 5. Estrategia de pruebas

- **Unitaria y adaptadores.** Usar reloj o plazos controlados, procesos de prueba con registros de invocación y servidores `httptest`. Verificar esfuerzo, parseo, redirecciones, aislamiento de entorno, plazos, limpieza y cancelación. Para el bloqueo de herramientas nativas, el simulado solo cubre nuestro lanzador. Antes de habilitar la invocación normal, demostrar el control del proveedor en una sonda autorizada con herramientas centinela inocuas y configuración aislada en un directorio temporal. La inspección posterior de eventos complementa la política previa.
- **Aceptación en proceso.** Ejecutar por `cli.Run`, comprobar códigos y JSON contra las referencias, contar invocaciones y comparar árbol y salidas con un centinela de credencial ante éxito y fallo. Ejecutar la suite existente para códigos 0–7. Para cancelación, incluir una comprobación del enlace entre Ctrl+C y el contexto de la aplicación en Windows.
- **Evidencia viva.** Con autorización humana, un prompt trivial por Antigravity, Vibe y al menos una API directa, y un fallback real con primer perfil autorizado declarado con un modelo inválido y segundo sano. Usar riesgo medio y disponibilidad sin verificar para que el primer perfil llegue al ejecutor, registrar rechazo y éxito posterior. Si un candidato se descarta antes de invocarlo, esa prueba no demuestra fallback en vivo. Guardar fecha, versiones, código y JSON depurado sin respuesta completa. Marcar por ejecutor «no verificado» mientras falte evidencia. La suite local puede completarse en paralelo con la espera de autorización.

## 6. Secuencia y dependencias

Definir política y puerto, luego construir catálogo y selección. Preparar el orquestador y los tres adaptadores sobre los mismos contratos. Integrar en la CLI cuando estén disponibles las piezas, probar por el punto de entrada y cerrar con verificación local, viva y revisión.

## 7. Riesgos y decisiones abiertas

- **Aislamiento de Antigravity y Vibe.** La ayuda de la versión instalada de `agy` muestra `--settings` por sesión. La configuración restringida y la ausencia de efectos laterales aún requieren demostración. Si el control o la sonda falla, el adaptador queda deshabilitado y se eleva la decisión de alcance. Vibe requiere comprobar la misma garantía efectiva.
- **Compatibilidad.** El catálogo y los esfuerzos dependen de las versiones y del modelo. Verificar la documentación y los contratos de los binarios antes de implementar cada traducción. OpenCode se tratará según las suposiciones Q2/Q3 y solo donde coincida el protocolo.
- **Autorización.** No hay permiso de consumo implícito en el plan. Sin pruebas vivas, T010 y el cierre de la especificación original quedan pendientes.
- **Revisión.** El forecast de `rsc risk` es por rutas. Recalcular sobre el diff de cada tarea. Los cambios relacionados con secretos, permisos y lanzamiento de procesos merecen revisión explícita de seguridad, incluidos los casos de fallo.

## Tasks

<!-- generated by tasks on 2026-11-18; IDs estables -->

| ID | [P] | Tarea | Done-check previsto | Depende de | Traza | Tier por ruta |
| --- | --- | --- | --- | --- | --- | --- |
| T001 | | Añadir política y validación al cargador | `go test ./internal/config -run TestRouterPolicy -count=1` comprueba defaults, reemplazo, denegación de cambios sensibles desde proyecto y rechazo de credenciales en archivos | — | AC5, AC13, AC14, AC16, AC17 | 1 |
| T002 | | Definir puerto y componer catálogo | `go test ./internal/router -run TestCatalog -count=1` comprueba origen, orden, credencial ausente, CLI ausente y degradación sin descubrimiento | T001 | AC1, AC2, AC5, AC15, AC19 | 1 |
| T003 | | Implementar selección explicable | `go test ./internal/router -run TestSelection -count=1` comprueba determinismo, motivos, manual, no elegibles, riesgo y esfuerzo | T002 | AC3–AC5, AC9, AC10, AC12, AC19 | 1 |
| T004 | | Orquestar intentos y cancelación | `go test ./internal/router -run TestRun -count=1` comprueba éxito, agotamiento, fallback ante fallo y timeout, cancelación antes y durante intento, y redacción | T003 | AC5–AC13, AC18 | 1 |
| T005 | [P] | Integrar Antigravity con modo solo texto | `go test ./internal/router/antigravity -run TestAdapter -count=1` comprueba puerto, argumentos, alcance y rechazo sin control instalado. T010 demuestra la política real | T002 | AC2, AC5, AC6, AC11–AC13, AC15, AC18 | 1 |
| T006 | [P] | Integrar Mistral Vibe con modo solo texto | `go test ./internal/router/vibe -run TestAdapter -count=1` comprueba puerto, argumentos, alcance y rechazo sin control instalado. T010 demuestra la política real | T002 | AC2, AC5, AC6, AC11–AC13, AC15, AC18 | 1 |
| T007 | [P] | Integrar APIs directas y proveedores compatibles | `go test ./internal/router/direct -run TestAdapter -count=1` comprueba los tres protocolos, extensión por configuración, plazos, ningún redirect y credenciales acotadas | T002 | AC1, AC2, AC5, AC6, AC11–AC13, AC15, AC16, AC18 | 1 |
| T008 | | Exponer comandos y ampliar doctor | `go test ./internal/cli ./internal/doctor ./cmd/gandalf -run 'TestRouterCLI\|TestRouterDoctor\|TestRouterInterrupt' -count=1` comprueba dispatch, tabla/JSON, códigos, diagnóstico sin cuota y cancelación desde el proceso | T004–T007 | AC1–AC12, AC15, AC17–AC19 | 1 |
| T009 | | Ejecutar aceptación y regresión local | `go test ./internal/cli -run TestRouterAcceptance -count=1` y `go test ./...` cubren AC1–AC19, archivos, centinela y compatibilidad | T008 | AC1–AC19 | 1 |
| T010 | | Registrar verificación viva y cierre | Con autorización, registrar tres éxitos, bloqueo de herramientas en ambos CLIs y fallback vivo en `verifications/`. Todos los done-checks previos, `gofmt -l .`, `go vet ./...` y `go test ./...` verdes, con revisión de cierre | T009 | AC1–AC19, spec «Cómo lo demuestra» | 0 |

### Interfaces por tarea

- **T001** consume `config.Load(LoadInput) -> EffectiveConfig | ConfigError` existente. Produce `EffectiveConfig.Router: Policy` con el esquema y precedencia de §3. El código 3 se verifica en T009.
- **T002** consume `EffectiveConfig.Router: Policy`. Produce `Adapter.Probe(ctx, Executor) -> Probe`, `Adapter.Discover(ctx, Executor) -> Candidate[] | unavailable`, `Adapter.Execute(ctx, Execution) -> Text | SafeFailure` y `catalog.Build(ctx, Policy, Registry, DiscoveryMode) -> Catalog`. `Registry` resuelve el puerto por ID de ejecutor.
- **T003** consume `Catalog`. Produce `router.Select(Catalog, Features, manualID?) -> Decision`.
- **T004** consume `router.Select(Catalog, Features, manualID?) -> Decision` y `Adapter.Execute(ctx, Execution) -> Text | SafeFailure`. Produce `router.Run(ctx, Policy, Decision, task: Text, Registry) -> Result`.
- **T005, T006, T007** consumen `Executor` y `Execution`. Cada uno produce una implementación de `Adapter.Probe(ctx, Executor) -> Probe`, `Adapter.Discover(ctx, Executor) -> Candidate[] | unavailable` y `Adapter.Execute(ctx, Execution) -> Text | SafeFailure`, en paquetes separados.
- **T008** consume `config.Load(LoadInput) -> EffectiveConfig | ConfigError`, `catalog.Build(ctx, Policy, Registry, DiscoveryMode) -> Catalog`, `router.Select(Catalog, Features, manualID?) -> Decision`, `router.Run(ctx, Policy, Decision, task: Text, Registry) -> Result` y el puerto de los adaptadores. Produce los cuatro contratos públicos de §3 por `cli.Run(ctx, args, deps) -> int`.
- **T009** consume `cli.Run(ctx, args, deps) -> int`, envoltorios de §3 y códigos 0–10. Produce evidencia de aceptación local y regresión.
- **T010** consume los contratos de CLI de §3 y la matriz de T009. Produce evidencia viva depurada y estado de verificación por ejecutor.

### Previsión de revisión y entrega

Estimación inicial de 900–1600 líneas en 18–30 archivos entre config, router, adaptadores, CLI, doctor y pruebas. Supera el presupuesto de 400 líneas y 12 archivos de `config.yaml`. Aplicar `ask-on-risk` a la entrega y preparar cortes revisables por contrato, adaptadores e integración. Riesgo de rutas según el desglose: nueve tareas tier 1 y una tier 0. Recalcular riesgo real por diff. El PR de cierre de la épica debe incluir `Closes #15` una vez entregadas ambas especificaciones.
