---
type: plan
title: Plan — Consulta de memoria de proyecto en ROMERO
description: Plan técnico a nivel de estructura, contratos, flujo de datos y estrategia de pruebas para Foundation 0.1.
tags: [sdd, plan, foundation, romero]
timestamp: 2026-11-17T22:52:00Z
topic: sdd
slug: consulta-memoria-romero
status: approved
---

# Plan — Consulta de memoria de proyecto en ROMERO

> Spec: [../specs/consulta-memoria-romero.md](../specs/consulta-memoria-romero.md) · Constitution: [../constitution.md](../constitution.md) · Status: approved
> Last updated: 2026-11-17

## 0. Global Constraints

- **Operaciones permitidas hacia ROMERO:** exclusivamente operaciones de lectura (`search` y `getDetail` sobre el contrato público de ROMERO). Prohibido solicitar o ejecutar creación, modificación, calificación de recuerdos, resolución de contradicciones o cambios de permisos (`spec §Fuera de alcance`, `spec §Criterios #6`, `constitution §Principios #5`).
- **Guardia previa de ámbito de proyecto:** cuando no exista un proyecto activo único (ausente o ambiguo entre varios candidatos), Gandalf debe realizar exactamente `0` consultas a ROMERO y solicitar antes la identificación del proyecto (`spec §Comportamiento`, `spec §Criterios #7`).
- **Aislamiento estricto por proyecto y confidencialidad:** solo se devuelven recuerdos pertenecientes al `projectId` activo autorizado. Ante recuerdos de otro proyecto o denegación de acceso al proyecto activo, la salida no debe filtrar contenido, resumen, identificadores (`memoryId`, `sourceRef`) ni la existencia misma de recuerdos protegidos (`spec §Criterios #4`).
- **Etiquetas canónicas de estado y fallo:**
  - Cero coincidencias en búsqueda completada: `"Sin coincidencias visibles"` (`spec §Criterios #5`).
  - Búsqueda completada en modo degradado: conservar explícitamente el aviso de búsqueda degradada junto con los resultados o con cero resultados (`spec §Comportamiento`, `spec §Criterios #5`).
  - Interrupción antes de confirmar ningún resultado: `"Consulta incompleta"` (`spec §Comportamiento`, `spec §Criterios #5`).
  - Interrupción tras confirmar al menos un resultado con su detalle: `"Consulta parcial"`, incluyendo únicamente las entradas ya confirmadas y omitiendo todo dato o identificador de las entradas pendientes (`spec §Comportamiento`, `spec §Criterios #8`).
  - Recuerdo válido sin referencia visible a su fuente original: conservar la referencia del recuerdo e indicar `"Fuente de origen no disponible"` (`spec §Criterios #3`).
- **Presentación de decisiones y sustituciones:** cada recuerdo visible se presenta por separado con su resumen atribuido a ROMERO, su calificación de evidencia (ej. `observed`, `hypothesis`), la referencia del recuerdo (`M1`) y la referencia de la fuente original (`F1`). Cuando coinciden una decisión vigente y otra sustituida autorizadas en el proyecto, se muestran ambas por separado e identificando cuál fue sustituida únicamente si ROMERO declara esa relación (nunca deducida solo por discrepancia o fecha anterior) (`spec §Comportamiento`, `spec §Criterios #1, #2, #9`).
- **Contenido atribuido sin efectos colaterales:** cualquier instrucción o sugerencia de acción contenida dentro de un recuerdo se trata como texto atribuido inerte; nunca dispara herramientas ni altera permisos (`spec §Criterios #6`).
- **Higiene de secretos, repositorios y autoría:** prohibido incluir credenciales, tokens o datos privados en código, pruebas o artefactos de verificación; no introducir código fuente de RSC en el repositorio de Gandalf; commits con autoría humana exclusiva sin `Co-Authored-By` de IA (`constitution §Principios #1, #4, #5, #8`, `REVIEW.md`).
- **Stack y controles de calidad obligatorios:** Go (`go1.27.3`, módulo en `go.mod` sin dependencia externa al repositorio de ROMERO, paquete `internal/romero`), transporte inicial REST con `net/http` estándar sobre el contrato público de `pkg/romeroclient` (`POST /api/v1/memories/search` con `include_superseded: true` y `GET /api/v1/memories/{id}`, revisión `3f46f9a322b3b44b946889d8474ac835b98bf9fb`), verificación obligatoria con `gofmt -l .` (salida vacía), `go vet ./...` y `go test ./...` (`constitution v1.1.0`).

## 1. Context & constraints

- **Criterios de aceptación que moldean el diseño:** `spec §Criterios #1, #3, #8` exigen combinar la búsqueda con la lectura de detalle por recuerdo y tolerar interrupciones parciales; `spec §Criterios #4, #7` imponen validación previa de ámbito y opacidad ante accesos no autorizados; `spec §Criterios #2, #5, #6, #9` fijan la semántica de atribución, degradación, vigencia e inocuidad.
- **Restricciones del contrato de ROMERO:** según `spec §Qué resultado obtiene` (revisión `3f46f9a322b3b44b946889d8474ac835b98bf9fb` de `pkg/romeroclient/types.go`), la respuesta compacta de búsqueda aporta resumen, estado epistemológico e identificador del recuerdo, pero carece de la referencia a la fuente primaria, disponible en la consulta de detalle.
- **Reglas constitucionales activas:** `constitution §Principios #2, #3, #4, #5, #6` (decisiones explícitas, ASDD antes de código, evidencia observable en `verifications/`, mínimo privilegio y distinción entre controles deterministas y consultivos).
- **Fuera de alcance que el diseño no debe invadir:** captura o edición de recuerdos, recalibración bayesiana/epistemológica, resolución automática de contradicciones, gestión de permisos y sincronización de configuración entre anfitriones (`spec §Fuera de alcance`).

## 2. Architecture

```text
[ Caller / Host CLI ]
         |
         v
[ ProjectScopeGuard (INTERNAL) ]
         | (validated ProjectScope)
         v
[ MemoryQueryOrchestrator (INTERNAL) ] ---> [ ProvenancePresenter (INTERNAL) ]
         |
         v
[ RomeroReadPort (INTERNAL contract) ]
         |
         +---> [ RomeroFakeAdapter (INTERNAL test double) ]
         |
         +---> [ RomeroPublicClientAdapter (INTERNAL) ] --REST/MCP (read-only)--> [ ROMERO Service (EXTERNAL) ]
```

- **ProjectScopeGuard** (internal) — Verifica de forma determinista que la petición tenga un único `projectId` activo e identidad de consulta antes de permitir cualquier llamada externa.
- **MemoryQueryOrchestrator** (internal) — Coordina la búsqueda compacta y el enriquecimiento de detalle por recuerdo, clasifica estados (`completa`, `degradada`, `parcial`, `incompleta`, `denegada`) y filtra cualquier entrada no confirmada o fuera de ámbito.
- **RomeroReadPort** (internal) — Contrato de solo lectura que aísla al dominio de Gandalf del protocolo de transporte concreto hacia ROMERO.
- **RomeroPublicClientAdapter** (internal) — Adaptador de producción que traduce `RomeroReadPort` al contrato público de ROMERO (`search` + `getDetail`) sin acceder a su persistencia interna.
- **RomeroFakeAdapter** (internal) — Doble determinista en memoria para simular los nueve casos de aceptación, incluidos fallos de red intermedios, modo degradado y aislamiento entre proyectos ficticios (`Faro` y `OtroProyecto`).
- **ProvenancePresenter** (internal) — Transforma el resultado verificado del orquestador en la representación tabular/estructurada atribuida a ROMERO con sus etiquetas canónicas.
- **ROMERO Service** (external) — Servicio externo de memoria neuro-simbólica que evalúa permisos y devuelve recuerdos, certeza, vigencia y referencias de fuente.

**Top architectural decision:** **Lectura en dos pasos (`search` compacto + `getDetail` por recuerdo) gobernada por guardia previa de ámbito y aislamiento de fallo parcial**, detrás de un puerto de solo lectura (`RomeroReadPort`).
- *Alternativas consideradas:*
  1. Consumir únicamente la búsqueda compacta de ROMERO. Descartada porque la respuesta compacta no expone la referencia a la fuente original (`F1`), incumpliendo `spec §Qué resultado obtiene` y `spec §Criterios #1, #3`.
  2. Enriquecer todos los detalles en bloque con política todo-o-nada. Descartada porque una interrupción en el segundo recuerdo impediría devolver los recuerdos ya confirmados con `"Consulta parcial"`, incumpliendo `spec §Criterios #8`.
  3. Búsqueda + enriquecimiento secuencial/acotado por recuerdo con corte limpio ante interrupción (elegida). Satisface simultáneamente `spec §Criterios #1, #3, #7, #8, #9`.

## 3. Interfaces & contracts

```text
ProjectScopeGuard.resolveScope(context: WorkspaceContext) -> ResolvedScope | ScopeMissing | ScopeAmbiguous(candidates: List<ProjectId>)
  - invariant: pure local check; invokes 0 network/ROMERO operations
  - postcondition: ResolvedScope guarantees exactly 1 non-empty projectId and callerIdentity
```

```text
RomeroReadPort.search(scope: ResolvedScope, queryText: String) -> SearchResult | AccessDenied | ServiceInterrupted
  - precondition: scope came from ProjectScopeGuard.resolveScope
  - invariant: read-only operation (`POST /api/v1/memories/search`); sends `project_ids: [scope.projectId]`, `scopes: ["project"]`, `include_superseded: true`; never mutates memories, scores, or permissions
  - SearchResult shape: { hits: List<MemorySearchHit>, degraded: Boolean, degradationNotice: Optional<String> }
  - MemorySearchHit shape: { memoryId: String, projectId: Optional<ProjectId>, summary: String, epistemicState: String, status: String, vigenciaState: Active | Superseded(replacedByMemoryId: Optional<String>) }
```

```text
RomeroReadPort.getDetail(scope: ResolvedScope, memoryId: String) -> DetailResult | AccessDenied | DetailInterrupted
  - precondition: memoryId came from a search candidate for active scope
  - invariant: read-only operation (`GET /api/v1/memories/{id}`)
  - DetailResult shape: { memoryId: String, projectId: ProjectId, sourceRef: Optional<String> (empty when PrimarySourceID is zero/nil), status: String, epistemicState: String, supersededBy: Optional<String> }
```

```text
MemoryQueryOrchestrator.queryProjectMemory(context: WorkspaceContext, queryText: String) -> MemoryQueryOutcome
  - precondition: queryText is non-empty
  - invariant:
      1. If ProjectScopeGuard fails, RomeroReadPort call count == 0.
      2. Every returned AttributedMemoryEntry has detail.projectId == scope.projectId AND completed both search and getDetail.
      3. If DetailInterrupted occurs on hit K (K > 1), entries 1..(K-1) are preserved with outcomeStatus = PartialQuery, and hits K..N are completely omitted (no memoryId, summary, or partial fields leaked).
      4. If DetailInterrupted or ServiceInterrupted occurs before any hit is confirmed, outcomeStatus = IncompleteQuery and entries is empty.
```

```text
ProvenancePresenter.present(outcome: MemoryQueryOutcome) -> PresentedMemoryView
  - postcondition:
      - Maps empty sourceRef to "Fuente de origen no disponible".
      - Maps empty confirmed entries (normal completion) to "Sin coincidencias visibles".
      - Preserves degradation notice whether entries is empty or non-empty.
      - Marks superseded decisions explicitly only when vigenciaState == Superseded / supersededBy is present in ROMERO's payload.
      - Renders memory summaries strictly as attributed text without executing embedded instructions.
```

## 4. Data model & flow

**Entities**

- **ResolvedScope** — `projectId`, `callerIdentity`; delimita la consulta autorizada.
- **MemorySearchHit** — `memoryId`, `projectId` (opcional en la respuesta compacta de búsqueda), `summary`, `epistemicState` (`observed`, `hypothesis`, etc.), `status`, `vigenciaState` (`active` | `superseded`).
- **MemoryDetail** — `memoryId`, `projectId` (obligatorio en `GET /api/v1/memories/{id}`), `sourceRef` (opcional; vacío cuando `PrimarySourceID` es nulo/`00000000-0000-0000-0000-000000000000`), `status`, `epistemicState`, `supersededBy` (opcional).
- **AttributedMemoryEntry** — `summary`, `epistemicState`, `memoryId`, `sourceRefDisplay` (`sourceRef` o `"Fuente de origen no disponible"`), `vigenciaStatus` (`vigente` | `sustituida`).
- **MemoryQueryOutcome** — `status` (`ok` | `empty` | `partial` | `incomplete` | `access_denied` | `scope_required`), `degraded` (`Boolean`), `degradationNotice` (opcional), `entries` (`List<AttributedMemoryEntry>`), `auditTrail` (conteo y tipo de operaciones de lectura solicitadas, sin secretos ni datos privados).

**Primary flow** (`Consulta trazable con decisión vigente/sustituida y tolerancia a fallo parcial`)

1. El punto de entrada recibe `(context, queryText)` y llama a `ProjectScopeGuard.resolveScope(context)`. Si devuelve `ScopeMissing` o `ScopeAmbiguous`, retorna de inmediato `scope_required` con `0` llamadas a `RomeroReadPort` (Caso 7).
2. `MemoryQueryOrchestrator` invoca `RomeroReadPort.search(scope, queryText)`.
   - Si recibe `AccessDenied`, devuelve `access_denied` sin revelar existencia ni identificadores (Caso 4).
   - Si recibe `ServiceInterrupted`, devuelve `incomplete` (`"Consulta incompleta"`) (Caso 5).
3. El orquestador descarta defensivamente cualquier hit que incluya `projectId != ""` distinto de `scope.projectId` (Caso 4). Si la lista queda vacía, devuelve `empty` (`"Sin coincidencias visibles"`), conservando `degraded`/`degradationNotice` si ROMERO lo reportó (Caso 5).
4. Para cada hit válido en orden:
   - Invoca `RomeroReadPort.getDetail(scope, hit.memoryId)` (`GET /api/v1/memories/{id}`).
   - Si falla con `DetailInterrupted` o `AccessDenied`:
     - Si ya existe al menos una entrada confirmada, corta el bucle y devuelve `partial` (`"Consulta parcial"`) únicamente con las entradas ya confirmadas (Caso 8).
     - Si ninguna entrada se había confirmado aún, devuelve `incomplete` (`"Consulta incompleta"`).
   - Verifica defensivamente que `detail.projectId == scope.projectId`; si pertenece a otro proyecto, descarta esa entrada sin revelar datos ni identificadores (Caso 4).
   - Si el detalle llega sin `sourceRef` visible (`PrimarySourceID` nulo o vacío), asigna `"Fuente de origen no disponible"` (Caso 3); si llega con `sourceRef` (ej. `F1`), lo vincula junto con `memoryId` (`M1`), `epistemicState` y la marca de sustitución explícita si ROMERO la declara en `status`, `relations` o `supersededBy` (Casos 1, 2, 9).
   - Si tras procesar todos los hits válidos no queda ninguna entrada perteneciente a `scope.projectId`, devuelve `empty` (`"Sin coincidencias visibles"`), conservando `degraded`/`degradationNotice` si aplica.
5. `ProvenancePresenter` formatea las entradas confirmadas como evidencia atribuida inerte (Caso 6) junto con los avisos de estado o degradación.

- **Consistency boundaries:** Cada consulta es una operación de solo lectura sin estado persistido en Gandalf; la consistencia entre el resumen de búsqueda y el detalle de un `memoryId` se acota a los recuerdos cuyo `getDetail` finaliza con éxito dentro de la misma petición.
- **Migration impact:** `none` — Gandalf no mantiene base de datos propia para recuerdos ni accede a tablas internas de ROMERO (`spec §Fuera de alcance`, `gandalf-wizard-contexto-maestro.md §4.XV ID 263`).

## 5. Testing strategy

| Acceptance criterion | Level | Asserts | Fakes / mocks |
| --- | --- | --- | --- |
| `spec §Criterios #1` (Recuerdo trazable) | unit + contract | Devuelve resumen, estado `observed`, `M1` y `F1` para la pregunta en proyecto `Faro`. | `RomeroFakeAdapter` con dataset de `Faro` (`M1` → `F1`). |
| `spec §Criterios #2` (Varios recuerdos) | unit | Devuelve dos entradas separadas conservando cada una su propia referencia (`M1`, `M2`) y calificación (`observed`, `hypothesis`). | `RomeroFakeAdapter` devolviendo dos hits con estados distintos. |
| `spec §Criterios #3` (Origen incompleto) | unit | Conserva la referencia del recuerdo y muestra `"Fuente de origen no disponible"` cuando `sourceRef` es nulo/vacío. | `RomeroFakeAdapter` con `getDetail` sin `sourceRef`. |
| `spec §Criterios #4` (Ámbito y permisos) | unit + contract | Excluye recuerdos de `OtroProyecto` aunque la identidad tenga lectura en ambos; ante denegación en `Faro`, no filtra contenido, IDs ni existencia. | `RomeroFakeAdapter` multi-proyecto (`Faro`, `OtroProyecto`) y modo `AccessDenied`. |
| `spec §Criterios #5` (Vacío, degradación y fallo) | unit | Distingue `"Sin coincidencias visibles"`, aviso de búsqueda degradada (con y sin resultados) y `"Consulta incompleta"`. | `RomeroFakeAdapter` configurado en modos vacío, degradado e interrupción en `search`. |
| `spec §Criterios #6` (Lectura sin efectos solicitados) | unit | El texto con sugerencia de acción se presenta como contenido atribuido; el registro de operaciones verifica solo llamadas `search`/`getDetail`. | `RomeroFakeAdapter` con recuerdo cuyo resumen incluye comando/sugerencia + espía de llamadas del puerto. |
| `spec §Criterios #7` (Proyecto ausente o ambiguo) | unit | Solicita identificar el proyecto y el contador de llamadas a `RomeroReadPort` es `0` tanto sin proyecto como con dos proyectos candidatos. | Espía sobre `RomeroReadPort` que falla el test si recibe cualquier invocación. |
| `spec §Criterios #8` (Interrupción parcial) | unit | Tras confirmar `M1` con `F1` y fallar `getDetail` de `M2`, muestra `M1`/`F1` + `"Consulta parcial"` y cero datos o ID de `M2`. | `RomeroFakeAdapter` que resuelve `getDetail(M1)` e interrumpe `getDetail(M2)`. |
| `spec §Criterios #9` (Decisión sustituida) | unit + contract | Muestra decisión vigente y sustituida por separado, marca la sustituida según metadato de ROMERO, conserva certezas/referencias y respeta límites del Caso 4. | `RomeroFakeAdapter` con par de decisiones (`vigente` y `sustituida`) y caso negativo con fechas distintas sin marca de sustitución. |

- **Línea divisoria de integración real (e2e/integration):** Los 9 casos se prueban de forma determinista y sin red mediante `RomeroFakeAdapter` y pruebas de contrato sobre `RomeroPublicClientAdapter`. La prueba de integración contra una instancia viva de ROMERO (`spec §Cómo lo demuestra`) comprobará la recuperación real de un recuerdo conocido y el aislamiento de ámbitos una vez se autorice el entorno (`spec §Puntos que siguen abiertos`).
- **Dependencia que NO debe falsearse en contrato/integración:** En el test de contrato de `RomeroPublicClientAdapter`, la serialización/deserialización real frente a los esquemas JSON de `pkg/romeroclient/types.go` (revisión `3f46f9a322b3b44b946889d8474ac835b98bf9fb`); en la prueba de integración autorizada, la instancia de ROMERO real.

## 6. Sequencing & dependencies

1. **Resolver decisión de stack (`sdd-init`) y aislar en worktree/rama propia (`worktrees`)** — depends on: none — `[serial]`
   - Con el aislamiento en worktree/rama ya aprobado por el propietario, acordar lenguaje/runtime, test runner, linter y formatter, y registrar `02-DOCS/wiki/sdd/config.yaml` y la decisión en `decisions.md` antes de escribir código.
2. **Definir tipos de dominio, etiquetas canónicas y `ProjectScopeGuard`** — depends on: #1 — `[serial]`
   - Implementar entidades inmutables, guardia de ámbito (`Caso 7`) y pruebas unitarias de bloqueo de consultas sin ámbito único.
3. **Implementar `RomeroReadPort` y `RomeroFakeAdapter` multi-proyecto** — depends on: #2 — `[parallelizable con #5]`
   - Construir el puerto de solo lectura y el doble determinista con datos de `Faro` y `OtroProyecto` e inyección de fallos/degradación.
4. **Implementar `MemoryQueryOrchestrator` (flujo en dos pasos, aislamiento, vigencia y corte parcial)** — depends on: #2, #3 — `[serial]`
   - Cubrir con TDD los casos 1, 2, 3, 4, 5, 6, 8 y 9 contra `RomeroFakeAdapter`.
5. **Implementar `ProvenancePresenter`** — depends on: #2 — `[parallelizable con #3]`
   - Formatear salida tabular/estructurada atribuida, avisos de degradación/parcialidad/origen no disponible y distinción de decisión sustituida.
6. **Implementar `RomeroPublicClientAdapter` y pruebas de contrato HTTP/MCP** — depends on: #3, #4 — `[serial]`
   - Conectar el puerto al contrato público de ROMERO (`search` y `getDetail`) verificado contra payloads reales del esquema `3f46f9a322b3b44b946889d8474ac835b98bf9fb`, y generar el registro verificable de los 9 casos en `02-DOCS/wiki/sdd/verifications/`.

- **Parallel candidates:** Los pasos #3 (`RomeroReadPort` + `RomeroFakeAdapter`) y #5 (`ProvenancePresenter`) operan sobre archivos disjuntos una vez definidos los tipos de dominio en #2.
- **Hard ordering constraints:** El paso #1 (stack y comandos de calidad) debe cerrarse antes de escribir código de producto (`constitution §Decisiones pendientes`); #2 debe preceder a #3, #4 y #5 porque fija las estructuras de contrato; #6 requiere el contrato #3 y el orquestador #4.

## 7. Risks & open decisions

**Risks**

| Risk | Trigger | Impact | Mitigation / spike to retire it |
| --- | --- | --- | --- |
| Filtración accidental de identificadores en fallo parcial o denegación | Un log de error o mensaje de excepción incluye el `memoryId` que falló en `getDetail` o el nombre de un recurso denegado. | Incumple `spec §Criterios #4, #8` y principios de confidencialidad. | Construir `AttributedMemoryEntry` solo tras completar `getDetail` con éxito y sanitizar los errores de `RomeroReadPort` antes de pasarlos al presentador; aserción explícita negativa en tests de los casos 4 y 8. |
| Inferencia indebida de sustitución por fecha o contradicción | El código compara `createdAt` o textos en lugar de leer el metadato explícito de sustitución de ROMERO. | Incumple `spec §Comportamiento` y `spec §Criterios #9`. | Incluir un test negativo en el Caso 9 con dos decisiones de fechas distintas sin relación de sustitución declarada por ROMERO y verificar que ninguna se marque como sustituida. |
| Latencia o exceso de llamadas en `getDetail` cuando `search` devuelve muchos hits | Una búsqueda amplia dispara decenas de llamadas de detalle secuenciales. | Mayor probabilidad de interrupción parcial o lentitud. | Acotar en el contrato del adaptador el límite de resultados por consulta y mantener el corte limpio de `"Consulta parcial"` ante timeout. |
| Deriva respecto al contrato público de ROMERO | Cambios posteriores a la revisión `3f46f9a322b3b44b946889d8474ac835b98bf9fb` de `pkg/romeroclient/types.go`. | Fallo de deserialización en integración. | Fijar fixtures de contrato basados en la revisión verificada `3f46f9a322b3b44b946889d8474ac835b98bf9fb` y validar la versión del entorno autorizado antes de la prueba de integración. |

**Open decisions**

- **Stack de implementación y comandos de calidad (resuelto 2026-11-17):** Go (`go1.27.3`, módulo `go.mod`, paquete `internal/romero`) con verificación mediante `gofmt -l .`, `go vet ./...` y `go test ./...` (`constitution v1.1.0`, `02-DOCS/wiki/sdd/config.yaml`).
- **Transporte inicial del adaptador público de ROMERO (resuelto 2026-11-17):** transporte inicial REST sobre el contrato público `pkg/romeroclient` (revisión `3f46f9a322b3b44b946889d8474ac835b98bf9fb`), detrás del puerto `RomeroReadPort` para incorporar MCP en iteraciones futuras.
- **Entorno e instancia autorizada de ROMERO para integración (abierta):** identificar versión, URL/entorno autorizado, datos ficticios de dos proyectos y credenciales de prueba antes de ejecutar la verificación de integración en vivo (`spec §Puntos que siguen abiertos`).

## Tasks
<!-- generated by tasks on 2026-11-17; IDs are stable, do not renumber -->

| ID | [P] | Task | Done-check | Depends-on | Trace | Risk |
| --- | --- | --- | --- | --- | --- | --- |
| T001 |  | Initialize `go.mod` and domain types/canonical labels in `internal/romero/types.go` | `go test -v ./internal/romero -run TestDomainTypesAndLabels` | — | AC1, AC3, AC5, AC8, AC9 | 1 |
| T002 |  | Implement `ProjectScopeGuard` and tests in `internal/romero/scope_guard.go` | `go test -v ./internal/romero -run TestProjectScopeGuard` | T001 | AC7 | 1 |
| T003 | [P] | Implement `RomeroReadPort` and multi-project `RomeroFakeAdapter` in `internal/romero/port.go` and `internal/romero/fake_adapter.go` | `go test -v ./internal/romero -run TestRomeroFakeAdapter` | T001 | AC1, AC2, AC3, AC4, AC5, AC8, AC9 | 1 |
| T004 | [P] | Implement `ProvenancePresenter` and tests in `internal/romero/presenter.go` | `go test -v ./internal/romero -run TestProvenancePresenter` | T001 | AC1, AC2, AC3, AC5, AC6, AC8, AC9 | 1 |
| T005 |  | Implement two-step read orchestration, project/permission isolation and read-only guarantees in `internal/romero/orchestrator.go` | `go test -v ./internal/romero -run "TestOrchestrator_(Traceable\|Multiple\|MissingSource\|ScopeAndPermissions\|ReadOnlyNoSideEffects)"` | T002, T003 | AC1, AC2, AC3, AC4, AC6, AC7 | 1 |
| T006 |  | Implement empty, degraded, incomplete/partial interruption and superseded-decision handling in `internal/romero/orchestrator.go` | `go test -v ./internal/romero -run "TestOrchestrator_(EmptyDegradedIncomplete\|PartialInterruption\|SupersededDecision)"` | T005 | AC5, AC8, AC9 | 1 |
| T007 |  | Implement REST `RomeroPublicClientAdapter` and contract tests in `internal/romero/rest_client.go` | `go test -v ./internal/romero -run TestRomeroPublicClientAdapter_Contract` | T003, T006 | AC1, AC4, AC6, AC9 | 1 |
| T008 |  | Implement end-to-end acceptance suite for all 9 spec cases in `internal/romero/acceptance_test.go` | `go test -v ./internal/romero -run TestAcceptanceCases_AC1_to_AC9` | T004, T006, T007 | AC1, AC2, AC3, AC4, AC5, AC6, AC7, AC8, AC9 | 1 |
| T009 |  | Run full quality gate across the Go module | `gofmt -l .` (empty output) AND `go vet ./...` AND `go test ./...` | T001, T002, T003, T004, T005, T006, T007, T008 | all ACs | — |

**T001 — Interfaces**
- Consumes: Go `1.27.3` toolchain (`go.mod` at repository root).
- Produces: package `romero` types and constants in `internal/romero/types.go`:
  - `LabelNoMatches = "Sin coincidencias visibles"`, `LabelIncomplete = "Consulta incompleta"`, `LabelPartial = "Consulta parcial"`, `LabelMissingSource = "Fuente de origen no disponible"`.
  - `WorkspaceContext`, `ResolvedScope`, `VigenciaState` (`VigenciaActive`, `VigenciaSuperseded`), `MemorySearchHit`, `SearchResult`, `MemoryDetail`, `AttributedMemoryEntry`, `OutcomeStatus` (`StatusOK`, `StatusEmpty`, `StatusPartial`, `StatusIncomplete`, `StatusAccessDenied`, `StatusScopeRequired`), `MemoryQueryOutcome`.

**T002 — Interfaces**
- Consumes: `WorkspaceContext`, `ResolvedScope` from `T001`.
- Produces: `ProjectScopeGuard` in `internal/romero/scope_guard.go`:
  - `type ScopeErrorKind string` (`ScopeMissing`, `ScopeAmbiguous`).
  - `func ResolveScope(ctx WorkspaceContext) (ResolvedScope, error)` returning typed error with zero external I/O when active project is missing or multiple candidates exist.

**T003 — Interfaces**
- Consumes: `ResolvedScope`, `SearchResult`, `MemoryDetail` from `T001`.
- Produces: `RomeroReadPort` in `internal/romero/port.go` and `RomeroFakeAdapter` in `internal/romero/fake_adapter.go`:
  - `type RomeroReadPort interface { Search(ctx context.Context, scope ResolvedScope, query string) (SearchResult, error); GetDetail(ctx context.Context, scope ResolvedScope, memoryID string) (MemoryDetail, error) }`.
  - Sentinel errors `ErrAccessDenied`, `ErrServiceInterrupted`, `ErrDetailInterrupted`.
  - `RomeroFakeAdapter` recording every operation name (`search`, `getDetail`) for read-only verification and supporting multi-project datasets (`Faro`, `OtroProyecto`), degraded search, empty results, missing `sourceRef`, superseded metadata, and mid-loop interruption on `GetDetail`.

**T004 — Interfaces**
- Consumes: `MemoryQueryOutcome`, `AttributedMemoryEntry`, canonical labels from `T001`.
- Produces: `ProvenancePresenter` in `internal/romero/presenter.go`:
  - `type PresentedMemoryView struct { StatusText string; DegradationNotice string; Rows []PresentedRow }` where `PresentedRow` holds `Summary`, `EpistemicState`, `MemoryRef`, `SourceRef`, `VigenciaLabel`.
  - `func Present(outcome MemoryQueryOutcome) PresentedMemoryView` rendering summaries strictly as attributed text without executing embedded instructions.

**T005 — Interfaces**
- Consumes: `ResolveScope` (`T002`), `RomeroReadPort` and `ErrAccessDenied`/`ErrServiceInterrupted`/`ErrDetailInterrupted` (`T003`).
- Produces: `MemoryQueryOrchestrator` in `internal/romero/orchestrator.go`:
  - `func NewMemoryQueryOrchestrator(port RomeroReadPort) *MemoryQueryOrchestrator`.
  - `func (o *MemoryQueryOrchestrator) QueryProjectMemory(ctx context.Context, ws WorkspaceContext, query string) MemoryQueryOutcome` guaranteeing 0 port calls on scope failure, filtering both hits with `hit.ProjectID != "" && hit.ProjectID != scope.ProjectID` and details with `detail.ProjectID != scope.ProjectID` without leaking any data or IDs, and hiding all details/IDs/existence on `ErrAccessDenied`.

**T006 — Interfaces**
- Consumes: `MemoryQueryOrchestrator` and `RomeroReadPort` error/status contracts (`T003`, `T005`).
- Produces: complete handling inside `QueryProjectMemory` (`internal/romero/orchestrator.go`) for empty+degraded notices, `StatusIncomplete` on 0 confirmed entries, `StatusPartial` preserving only `1..K-1` confirmed entries on failure at hit `K` without leaking hit `K..N`, and `VigenciaSuperseded` set strictly from explicit ROMERO metadata.

**T007 — Interfaces**
- Consumes: `RomeroReadPort` (`T003`), `SearchResult`, `MemoryDetail` (`T001`), and REST schema fixtures matching `pkg/romeroclient/types.go` rev `3f46f9a322b3b44b946889d8474ac835b98bf9fb`.
- Produces: `RomeroPublicClientAdapter` in `internal/romero/rest_client.go`:
  - `func NewRomeroPublicClientAdapter(baseURL string, httpClient *http.Client, authTokenProvider func() string) *RomeroPublicClientAdapter` implemented exclusively with Go stdlib `net/http` (zero external ROMERO dependency in `go.mod`), sending `POST /api/v1/memories/search` (`project_ids`, `scopes: ["project"]`, `include_superseded: true`) and `GET /api/v1/memories/{id}` with `Authorization: Bearer <apiKey>` and a `4 MiB` response-body read cap, mapping zero/empty `PrimarySourceID` (`00000000-0000-0000-0000-000000000000`) to `""` (`"Fuente de origen no disponible"`), and never logging tokens or leaking denied IDs.

**T008 — Interfaces**
- Consumes: `MemoryQueryOrchestrator` (`T006`), `ProvenancePresenter` (`T004`), `RomeroFakeAdapter` (`T003`), `RomeroPublicClientAdapter` (`T007`).
- Produces: `internal/romero/acceptance_test.go` covering `AC1` through `AC9` end-to-end with deterministic verification over simulated data for `Faro` and `OtroProyecto`.

## Review Workload Forecast

| Dimension | Forecast | Why |
| --- | --- | --- |
| Estimated changed lines | ~350–400 | 9 slices across Go domain types, scope guard, port/fakes, orchestrator, presenter, REST adapter and tests (within `line_budget: 400`) |
| Files / areas | 12 (`go.mod` + `internal/romero/*`) | `go.mod` + 11 Go source/test files in a single isolated module (`file_budget: 12`) |
| Risk | Max tier `1`; `0` tier-2 tasks | `rsc risk` classifies `go.mod` and `internal/romero/*` as `default` (tier 1, 1 reviewer per task, no human merge blocker) |
| Suggested delivery | `single-pr` | Cohesive Foundation 0.1 vertical slice within `line_budget: 400` and `file_budget: 12`; switch to `ask-on-risk` if diff exceeds 400 lines |

```json result-envelope
{
  "status": "complete",
  "executive_summary": "Plan técnico actualizado con el stack Go (go1.27.3) y transporte inicial REST aprobados, más la lista de tareas T001..T009 y la estimación de carga de revisión.",
  "artifact": "02-DOCS/wiki/sdd/plans/consulta-memoria-romero.md",
  "next_recommended": "analyze",
  "risk": "medium",
  "skill_resolution": {
    "used": ["plan", "tasks"],
    "missing": ["go"],
    "fallback": [],
    "compact_rules": [
      "Every task needs a runnable done-check, a criterion trace and a risk forecast.",
      "Forecast review load before implementation."
    ]
  },
  "evidence": [
    "02-DOCS/wiki/sdd/constitution.md (v1.1.0)",
    "02-DOCS/wiki/sdd/config.yaml",
    "02-DOCS/wiki/sdd/plans/consulta-memoria-romero.md (## Tasks T001..T009 + ## Review Workload Forecast)"
  ]
}
```
