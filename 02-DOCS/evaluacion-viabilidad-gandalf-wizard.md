# Gandalf Wizard — Evaluación de viabilidad

Gandalf Wizard es técnicamente viable como plataforma modular para configurar, ejecutar, supervisar, evaluar y mejorar trabajo asistido por IA. La base actual cubre la CLI, la consulta de memoria y el routing de texto. El siguiente salto es ejecutar workflows que puedan usar herramientas, recuperarse de fallos y demostrar qué permisos, fuentes y versiones utilizaron.

## Alcance de la evaluación

La evaluación sigue los 56 bloques y las 2.060 entradas del inventario adjunto. Trata en conjunto las entradas que comparten mecanismo, dependencia o límite. Evalúa viabilidad técnica y secuencia de desarrollo. Una estimación de plazo, costo y demanda requiere equipo, presupuesto y usuarios objetivo.

El inventario conserva su carácter de visión de producto. **La evaluación propone cambios de soporte, condiciones verificables y prioridades.** Roo Code oficial pasa a soporte histórico. La defensa y la autorización humana ya forman parte del inventario. El uso de L10 como valor predeterminado sigue abierto a decisión. [Contexto maestro](../gandalf-wizard-contexto-maestro.md) · [Constitución ratificada](wiki/sdd/constitution.md)

## Base comprobada

- **Foundation 0.1 y CLI.** Go, configuración, diagnóstico y consulta de memoria de ROMERO. La prueba viva demostró búsqueda textual degradada y lectura de detalle con procedencia. La recuperación semántica, los grafos y la escritura necesitan pruebas propias de punta a punta. [CLI](wiki/sdd/specs/cli-gandalf.md) · [Prueba viva de ROMERO](wiki/sdd/verifications/2026-11-18-romero-live-integration.json)
- **Router 0.2.** Perfiles de modelo y esfuerzo, selección explicable y fallback de solo texto. Las corridas vivas cubrieron Antigravity, Vibe y la API de Mistral. Las APIs directas OpenAI y Gemini tienen pruebas locales simuladas. Codex tiene detección local y puede invocar Gandalf. Su ejecución como destino del router sigue pendiente. [Router 0.2](wiki/sdd/specs/router-02.md) · [Pruebas vivas](wiki/sdd/verifications/2026-11-18-router-02-live-verification.json) · [Detección de harnesses](../internal/harness/detect.go)

## Núcleo, integraciones y catálogos

### 1. Núcleo universal — parcialmente operativo

Go, CLI y monolito modular dan la base. Contratos entre módulos, eventos, estado, configuración declarativa, esquemas YAML/JSON y migraciones son viables. La independencia de proveedor, dominio, modelo y harness la proporcionan los contratos y adaptadores. Publicaría la API, TUI y administración gráfica después de estabilizarlos. Para plugins multiplataforma evaluaría procesos externos con un protocolo versionado. [Paquete `plugin` de Go](https://pkg.go.dev/plugin)

### 2. Modalidades de funcionamiento — viables por etapas

La base local y no interactiva admite ejecución con un agente, modelo o harness. El mismo runtime puede incorporar interacción, programación, eventos y modalidades multiagente, multimodelo y multi-harness. La compilación, la ejecución nativa dentro de un harness y la híbrida dependen de extensiones autorizadas y del traspaso de estado. Remoto y distribuido requieren una necesidad de operación en varias máquinas o equipos.

### 3. Integraciones con harnesses — viables por capacidad y versión

- **Antigravity y Mistral Vibe** tienen ejecución de solo texto probada. **Codex** es un candidato inmediato por su presencia en el proyecto y su modo no interactivo documentado. [Router 0.2](wiki/sdd/verifications/2026-11-18-router-02-live-verification.json) · [Codex](https://learn.chatgpt.com/docs/non-interactive-mode)
- **OpenCode, Claude Code, GitHub Copilot CLI, Cline, Cursor y Amp** tienen superficies documentadas para automatización. Cada uno necesita comprobar autenticación, permisos, salida, cancelación y compatibilidad con la cuenta. [OpenCode](https://opencode.ai/docs/cli/) · [Claude Code](https://code.claude.com/docs/en/headless) · [GitHub Copilot CLI](https://docs.github.com/en/copilot/how-tos/copilot-cli/automate-copilot-cli/run-cli-programmatically) · [Cline](https://docs.cline.bot/usage/cli-overview) · [Cursor](https://cursor.com/docs/cli/using) · [Amp](https://ampcode.app/docs/cli/execute-mode)
- **Kimi Code y Aider** son candidatos. En Kimi se debe fijar la distribución y versión exactas. En Aider evaluaría un adaptador acotado a su flujo de edición. [Kimi CLI](https://moonshotai.github.io/kimi-cli/en/customization/print-mode.html) · [Aider](https://aider.chat/docs/scripting.html)
- **Gemini CLI** queda condicionado a las cuentas empresariales y credenciales de API que Google mantiene compatibles. Para los planes individuales afectados, priorizaría Antigravity. **Roo Code oficial** queda fuera del soporte activo propuesto por su cierre. Su entrada histórica puede ayudar a reconocer configuraciones antiguas. [Transición de Gemini CLI](https://developers.googleblog.com/an-important-update-transitioning-gemini-cli-to-antigravity-cli/) · [Cierre de Roo Code](https://github.com/RooCodeInc/Roo-Code-Docs/blob/main/docs/sunset.md)
- **Harnesses personalizados y SDK.** La detección de instalación y versión es viable. Capacidades, incompatibilidades y negociación necesitan una matriz y pruebas de conformidad por versión, sistema operativo y autenticación. Validaría el SDK con dos adaptadores distintos antes de publicarlo.

### 4. ROMERO — lectura operativa y ampliación condicionada

El cliente de ROMERO expone búsqueda, construcción de contexto, subgrafos, feedback y consolidación. La recuperación textual, semántica, híbrida, jerárquica, contextual, multidocumental y por grafos necesita pruebas por modalidad. RAG, Agentic RAG, GraphRAG, descomposición de consultas, expansión y reranking deben demostrar mejoras sobre la búsqueda base. Multimodalidad requiere conectores y un corpus apropiado.

ROMERO sigue a cargo de memorias, hechos, fuentes, versiones, indexación, ámbitos de acceso y organización del conocimiento cuando su contrato lo soporte. Esto incluye memorias episódicas, semánticas, procedimentales, compartidas, de proyecto y de organización, así como taxonomías, ontologías, metadatos, grafos y vigencia. Gandalf puede preparar conocimiento derivado, feedback y propuestas de consolidación. La publicación requiere permisos, revisión, procedencia y separación del original. [Contexto maestro](../gandalf-wizard-contexto-maestro.md) · [Prueba viva](wiki/sdd/verifications/2026-11-18-romero-live-integration.json)

### 5. Motor de objetivos — viable y prioritario

Modelaría intención, objetivos principales y secundarios, jerarquía, prioridades, restricciones, recursos, riesgos, entregables y criterios de aceptación, éxito y finalización. La IA puede proponer refinamientos y señalar ambigüedades o contradicciones. Versionar objetivos y vincularlos a tareas y resultados permite verificar cambios y cumplimiento.

### 6. Catálogos universales — viables con un registro común

Los 49 tipos pueden compartir identidad, versión, origen, dependencias y estado. Cada tipo añade su esquema específico cuando un consumidor lo necesita. Empezaría con skills, patrones, modelos, harnesses, herramientas y evaluaciones. Los catálogos restantes se incorporan al primer workflow que necesite su contenido. Mantener 49 sistemas separados multiplicaría las migraciones y validaciones.

### 7. Sistema de skills — viable con la frontera de RSC

`SKILL.md`, referencias, recursos, scripts y ejemplos cubren el formato base. Un manifiesto puede expresar composición, parámetros, condiciones, dependencias, capacidades, herramientas, seguridad, contratos y criterios de aceptación. Selección automática, contextual y semántica, traducción, combinación, generación, evaluación, optimización y detección de redundancia u obsolescencia requieren versiones y casos medibles. Lo mismo aplica a meta-skills, benchmarking y deprecación. Los scripts requieren permisos comprobables. RSC conserva la fuente canónica de las skills instaladas. [Agent Skills](https://agentskills.io/specification) · [Constitución](wiki/sdd/constitution.md) · [Épica #28](https://github.com/CodeMonkReaper/gandalf-wizard/issues/28)

### 8. Categorías de skills — viables como taxonomía

Las 71 categorías caben como etiquetas combinables. La cobertura real depende de disponer de skills y evaluaciones por categoría. Ciberseguridad se orienta a defensa. Legal, finanzas, ciencia e ingeniería requieren fuentes y validaciones según el uso.

### 9. Patrones de diseño — viables como catálogo consultivo

Los 35 patrones pueden registrar problema, señales, alternativas, costos y ejemplos. El sistema puede recomendar cualquiera de ellos con una justificación. La adopción depende del contexto del proyecto.

### 10. Patrones arquitectónicos — viables como referencias y plantillas

Los 40 patrones tienen cabida con criterios de adopción. El núcleo puede seguir como monolito modular. Microservicios, CQRS, Event Sourcing, Saga, serverless y otros patrones distribuidos se aplican cuando el proyecto requiere sus propiedades.

### 11. Patrones de agentes — viables con distinto costo

Routing, cadenas, paralelismo, supervisión, revisión independiente, intervención humana y recuperación tienen un camino directo. ReAct, reflexión, debate, adversarial, consenso, Best-of-N, swarm, aprendizaje de herramientas y recursión requieren límites y comparación con un solo agente. La independencia de la revisión debe medirse.

### 12. Metodologías de desarrollo — viables como perfiles y workflows

Todas las metodologías listadas tienen cabida. La selección manual, la recomendación explicable y los perfiles personalizados son abordables. Adaptación, composición e incompatibilidades requieren contratos de etapas, entregables, aprobaciones y precedencia. Las instrucciones descriptivas conservan carácter consultivo.

### 13. Antipatrones — viables como reglas y señales

Los 38 antipatrones tienen cabida. Dependencias circulares, recursión, presupuesto y referencias ausentes admiten comprobaciones acotadas. Complejidad innecesaria, sesgos, consenso falso, alucinaciones y sobreajuste requieren rúbricas, evidencia y revisión.

## Ejecución, modelos y dominios iniciales

### 14. Motor de workflows — viable y central

Una representación intermedia, validador y ejecutor de grafos permiten tareas parametrizadas, dependencias, ramificaciones, subworkflows y paralelismo acotado. El estado persistente habilita historial, checkpoints, pausa, reanudación, recuperación, cancelación y reintentos. Simulación, depuración, visualización, plantillas, preflight, postflight e importación y exportación pueden reutilizar el contrato. Bucles, máquinas de estado, adaptación y ejecución incremental necesitan semántica explícita. La reproducción del historial puede usar respuestas guardadas. Las nuevas llamadas a modelos pueden variar. Los efectos externos requieren idempotencia, compensación o intervención según la operación. [Temporal sobre replay e idempotencia](https://docs.temporal.io/activity-definition)

### 15. Workflow maestro — viable como plantilla configurable

Las 40 etapas caben como capacidades configurables. El flujo base recibe la solicitud, fija el objetivo, carga contexto, resuelve políticas, ejecuta, verifica y entrega. Selección de dominio, metodología, agente, harness, modelo y esfuerzo se activa según la tarea. Aprobación, monitoreo, revisión adversarial, correcciones, replanificación y actualización de memoria se activan según riesgo y permisos. Activar todas las etapas aumenta el costo de peticiones simples.

### 16. Motor de dominios — viable con perfiles componibles

Clasificador, selección automática o manual, dominio principal, dominios complementarios y especializaciones por industria o proyecto pueden compartir un contrato. Composición, transferencia de contexto y dominios personalizados requieren reglas de permisos, precedencia y compatibilidad.

### 17. Modos de especialización — viables como extensiones

Los 32 modos siguen en la visión. Software Engineering y Documentación son buenos candidatos para validar el núcleo con repositorios, herramientas y memoria existentes. Los demás modos pueden incorporar fuentes, herramientas y evaluaciones propias cuando haya un caso de uso. Cybersecurity Mode queda en el ámbito defensivo.

### 18. Estrategias de ejecución — viables con objetivos medibles

Direct, Standard, Rigorous, Resource-Constrained y perfiles personalizados se expresan con presupuesto, profundidad y revisión. Competencia, cooperación, consenso, exploración, experimentación, composición y cambios dinámicos requieren evaluación. Los perfiles de máxima calidad, profundidad, cobertura o velocidad indican una prioridad dentro de límites. El determinismo aplica a componentes donde se pueda demostrar.

### 19. Autonomía — viable bajo límites

Los niveles manual, supervisado, semiautónomo y autónomo pueden configurarse por proyecto, workflow, agente y tarea. Aprobaciones, presupuestos, riesgo, escalamiento, delegación, intervención y revocación se aplican en puntos que Gandalf controla. La historia puede sugerir ajustes. Ampliar permisos requiere autorización. La parada de emergencia bloquea trabajo futuro dentro de su alcance. Los efectos externos confirmados requieren tratamiento propio.

### 20. Model Registry — parcialmente operativo

El catálogo puede extenderse a modelos locales y remotos, ventanas de contexto, límites de salida, modalidad, herramientas, MCP, salidas estructuradas, esfuerzo, versión, costo y consumo. Cada dato conviene marcarlo como declarado, descubierto, medido o desconocido. Planes y cuotas dependen de la cuenta. Calidad, velocidad, fiabilidad y especialización requieren mediciones por tarea, modelo, harness y versión.

### 21. Router de modelos — parcialmente operativo

Selección explicada, fijación manual y fallback de solo texto dan la base. Los perfiles recomendados, balanceados, de costo, cuota, velocidad y calidad pueden usar filtros obligatorios y preferencias configurables. Filtraría primero por capacidad, permiso, disponibilidad y presupuesto. Historial y benchmarks ayudan a ordenar los candidatos. Selección por proyecto, dominio, tarea, agente o etapa comparte mecanismo. Normalización de esfuerzo, escalamiento, cascadas, ensambles y cambios dinámicos requieren evaluación y registro. Si la ejecución pudo producir un efecto, el fallback depende de conocer su estado o contar con idempotencia.

### 22. Suscripciones y recursos — viable con datos visibles

Gandalf puede registrar planes y restricciones, medir consumo, estimar costos, presupuestar por escala, limitar tokens, tiempo y reintentos, alertar, priorizar cargas y comparar modelos locales y remotos. Informes, pronósticos y recomendaciones de contratación o cancelación deben distinguir datos medidos de estimaciones. Planes y cuotas exactas quedan como desconocidos si el proveedor carece de una interfaz autorizada. Prevenir agotamiento externo en todos los casos queda fuera de la garantía por esa dependencia.

### 23. Orquestación multiagente — viable después del aislamiento

Registro, roles, ciclo de vida, scheduling, comunicación, supervisión, delegación, cancelación, sustitución e intercambio de artefactos son alcanzables. Probaría planificador, ejecutor y verificador aislados. Equipos dinámicos, jerarquías, agentes persistentes, memoria compartida, negociación y consenso se justifican si mejoran calidad o costo. Los límites de cantidad, recursión y propagación de errores pertenecen al runtime.

### 24. Compilador de harnesses — viable para capacidades verificadas

La épica #28 puede empezar por skills, reglas y patrones. Un compilador puede generar configuraciones nativas, detectar incompatibilidades, preservar ediciones locales y respaldar cambios. `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, `SKILL.md`, comandos, subagentes, MCP, hooks, políticas, permisos y workflows se validan por destino. La salida debe indicar soporte nativo, traducido, emulado o ausente. Un permiso expresado solo como instrucción tiene carácter consultivo. [Constitución](wiki/sdd/constitution.md) · [Épica #28](https://github.com/CodeMonkReaper/gandalf-wizard/issues/28)

### 25. Software Engineering — viable como primer dominio

Requerimientos, ASDD/SDD, diseño de software y datos, UI/UX, implementación, pruebas, revisión, debugging, refactor, documentación, dependencias y versiones tienen herramientas y resultados verificables. Pruebas unitarias, integración, E2E, aceptación, rendimiento, seguridad defensiva, propiedades, mutación, fuzzing y contratos se activan según proyecto. Migraciones, CI/CD, despliegue, rollback, monitoreo y mantenimiento añaden controles por ambiente.

### 26. Entrepreneur — viable como apoyo a decisiones

Discovery, mercado, clientes, estrategia, pricing, marketing, ventas, operaciones, financiación, MVP, pitch, riesgos y experimentos pueden usar modelos y plantillas. TAM/SAM/SOM, caja, punto de equilibrio, CAC, LTV, churn, MRR, ARR, ROI y runway requieren datos y supuestos visibles. Un Business Viability Score puede resumir una rúbrica con su sensibilidad. Go/No-Go/Pivot y las recomendaciones de negocio dependen del responsable y de evidencia de terreno.

### 27. Frameworks de Entrepreneur — viables como plantillas

Los 42 marcos caben como playbooks con entradas, pasos y entregables. Canvas, FODA, OKR y matrices estructuran el trabajo. A/B testing, valoración, Six Sigma y métodos de inferencia requieren datos, diseño y criterios adecuados.

### 28. Agentes de Entrepreneur — viables como roles

Los 24 perfiles y el comité caben en un runtime común. Los roles ejecutivos, de inversión y legales generan hipótesis, revisiones y borradores para el responsable. Comprometer dinero, contratos o decisiones requiere una operación autorizada.

## Documentación, evidencia, herramientas y seguridad

### 29. Deep Documentation Intelligence — viable por capas

Extracción, clasificación, reconstrucción, resumen, comparación, versiones y referencias son un buen inicio. Entidades, relaciones, requisitos, contradicciones, riesgos, obligaciones, ontologías y síntesis multidocumental necesitan corpus y evidencia. Análisis causal, experto, histórico, adversarial y de cobertura requiere revisión o criterios específicos. OCR, tablas, gráficos, imágenes, planos y CAD dependen de herramientas por formato. Process mining requiere registros compatibles. Informes, matrices, glosarios, diagramas, mapas y grafos pueden compartir contrato de artefactos. Publicar conocimiento en ROMERO requiere validación y autorización.

### 30. Especializaciones documentales — viables como perfiles

Las 37 áreas caben en el mismo sistema. Software, APIs, procesos y operaciones permiten iniciar con un corpus controlado. Medicina, derecho, finanzas, contabilidad, tributación, ingeniería y seguridad requieren fuentes, confidencialidad y validación profesional según el uso. La extracción con citas puede preceder a las conclusiones de dominio.

### 31. Profundidad L1–L10 — viable como política medible

L1–L10 pueden expresar fuentes, cobertura, verificaciones, diversidad de perspectivas, revisión independiente y presupuesto. El escalamiento automático puede responder al riesgo y a vacíos detectados. **El inventario propone L10 por defecto para Deep Documentation.** Mi recomendación es activarlo según objetivo, corpus y presupuesto. También es viable conservarlo como predeterminado si significa el nivel de mayor profundidad dentro del alcance acordado. En ambos casos, el informe debe mostrar cobertura efectiva, secciones omitidas, fuentes inaccesibles e incertidumbres.

### 32. Metodologías documentales — viables como protocolos

Las 26 pueden expresar entradas, pasos y requisitos de evidencia. Revisiones sistemáticas, metaanálisis y triangulación necesitan protocolo y datos adecuados. IRAC/CRAC requieren jurisdicción y fuentes. SysML, UML, BPMN y C4 aportan modelado. FMEA/FMECA, árboles de fallos y análisis causal necesitan registros e hipótesis verificables.

### 33. Agentes documentales — viables como roles

Los 31 perfiles y el comité caben en workflows comunes. Empezaría con parser, analista, verificador de citas y revisor independiente. Especialistas, grafos, ontologías, multimodalidad, auditoría de cobertura y comité se agregan cuando el corpus, el riesgo o el formato lo requieren.

### 34. Evidence Engine — viable y central

Un esquema puede vincular afirmaciones, requisitos, interpretaciones y recomendaciones con fuentes versionadas, fragmentos, localizadores, fechas y resultados de verificación. Esto incluye páginas, secciones, artículos, código, tablas, imágenes, diagramas y marcas temporales. Existencia y correspondencia exacta admiten pruebas directas. Suficiencia, independencia, contradicción y calidad requieren criterios del dominio. Verified, Supported, Inferred, Disputed, Unverified y Unknown necesitan transiciones auditables.

### 35. Legal Intelligence — viable como asistencia documentada

Puede localizar y comparar fuentes, extraer obligaciones, identificar jurisdicción, jerarquía, plazos, sanciones y excepciones, preparar matrices y exponer interpretaciones. Vigencia y jurisprudencia requieren fuentes autorizadas y fecha de consulta. Riesgos, conflictos normativos y conclusiones para actuar requieren revisión profesional.

### 36. Deep Research — viable con fuentes y trazabilidad

Planificación, preguntas, hipótesis, búsqueda iterativa, multilingüe y multifonte, triangulación, síntesis, bibliografía y revisión independiente reutilizan workflows y Evidence Engine. Cada corrida puede registrar universo consultado, criterios, sesgos y vacíos. Guardar resultados en ROMERO depende del contrato de escritura y la aprobación.

### 37. Engineering y Architect — viable como análisis asistido

Requisitos, interfaces, diseño, ADR, alternativas, ATAM, confiabilidad, tolerancias, unidades y trazabilidad tienen un camino claro. MBSE, CAD/BIM, simulación y verificación de cálculos dependen de herramientas y estándares aplicables. Los diseños que requieran aprobación profesional conservan esa aprobación humana.

### 38. Context Engine — viable con límites explícitos

Descubrimiento, recuperación, selección, ranking, compresión, expansión, fragmentación, caché, versiones, presupuesto y armado del prompt pueden compartir un contrato. Transferencia y recuperación entre agentes, modelos y harnesses puede pasar objetivos, fuentes, artefactos y resúmenes autorizados. El estado interno del proveedor es portable cuando este lo expone. Aislamiento, resistencia a contexto hostil y preservación de hechos críticos requieren pruebas.

### 39. Herramientas e integraciones — viables por adaptador

Registro, selección, descubrimiento, composición, permisos, confiabilidad, health checks y fallback son abordables. REST, CLI y MCP son candidatos iniciales. A2A cubre interoperabilidad con agentes externos. GraphQL, gRPC, SDK y webhooks entran por demanda. Navegadores, buscadores, IDE, Git, archivos, bases de datos, contenedores, nube, ERP/CRM, productividad y herramientas científicas o CAD/BIM requieren accesos y pruebas por integración. [MCP](https://modelcontextprotocol.io/specification/latest) · [A2A](https://a2a-protocol.org/latest/specification/)

### 40. Runtime — viable con costo creciente al distribuirlo

Scheduling local, colas, procesos, sesiones, timeouts, streaming, checkpoints, recuperación, backpressure, cancelación y apagado ordenado pueden construirse por etapas. El aislamiento por tarea, agente y proyecto debe probar límites de archivos, red y procesos secundarios. La ejecución distribuida, las colas de fallos y el estado entre máquinas agregan manejo de duplicados, pérdida de workers y recuperación.

### 41. Repositorios y código — viable con operaciones autorizadas

Discovery, historial, diffs, impactos, dependencias, propiedad, mapas, símbolos y AST pueden avanzar incrementalmente. Ramas y worktrees permiten separar agentes. Verificación, parches, changelogs y releases reutilizan el historial. Commits, PR, merges, protección y rollback deben obedecer autorización y política del repositorio. Un conflicto de merge puede producir un parche revisable.

### 42. Políticas y reglas — viable y necesario antes de ejecutar herramientas

Un modelo común puede expresar herencia, precedencia, restricciones, preferencias, permisos, aprobaciones y auditoría para todos los ámbitos. Los conflictos estructurados admiten comprobaciones directas. Las contradicciones en instrucciones de texto se reportan para revisión. El punto de ejecución debe aplicar la política efectiva. [Constitución](wiki/sdd/constitution.md)

### 43. Seguridad — viable como controles comprobables

Autenticación, autorización, RBAC/ABAC, secretos, cifrado, acceso de red y archivos, aislamiento, integridad, firma, SBOM, escaneo, auditoría y respuesta a incidentes se incorporan según el despliegue. Prompt injection, envenenamiento de contexto y fugas requieren prevención y pruebas de bloqueo. SAST, DAST y red teaming defensivo caben dentro del alcance definido en el inventario.

## Evaluación, aprendizaje, interfaces y escala

### 44. Evaluación y calidad — viable y prioritaria

Un contrato de evaluador puede cubrir objetivos, tareas, skills, modelos, harnesses, workflows, herramientas, evidencia, artefactos, costo y fiabilidad. Los gates deterministas bloquean por incumplimientos observables. Rúbricas, comparaciones por pares, modelos jueces, humanos y revisión adversarial cubren criterios interpretativos. Corrección y reevaluación necesitan límites y registro. Exactitud, completitud, sesgo y cobertura dependen de los casos medidos. [Estudio sobre LLM-as-a-Judge](https://arxiv.org/abs/2306.05685)

### 45. Evaluación competitiva — viable como experimento

Arenas, torneos, rankings, síntesis y comparaciones históricas pueden compartir el motor de evaluación. Empezaría con dos alternativas sobre las mismas tareas y condiciones, con criterios fijados antes de la corrida. Evaluación ciega, revisión independiente y control de contaminación se aplican según la decisión. Cada ranking necesita dominio, conjunto de tareas, fecha y presupuesto.

### 46. Benchmark Engine — viable con corpus versionado

Registro, generador, runner, validador y analizador son extensiones naturales. Todos los dominios y componentes listados admiten benchmarks si existen referencias, casos adversariales y datos de regresión apropiados. Mediría calidad, latencia, consumo, costo, éxito, errores y fiabilidad con incertidumbre. Las tareas reservadas permiten comprobar generalización.

### 47. Motores especializados — viables como responsabilidades compartidas

Los 31 nombres encajan en contratos de intención, investigación, planificación, decisión, evidencia, conocimiento, evaluación, optimización y recuperación. Confianza exige calibración. Causalidad y simulación requieren datos y supuestos comprobables. Evidence, Knowledge y Recovery deben reutilizar los bloques 4, 34 y 40. Implementar cada responsabilidad como servicio separado duplicaría contratos.

### 48. Artefactos y entregables — viables por formato

Registro, contratos, generación, validación, versiones, dependencias, trazabilidad y traspaso pueden compartir un modelo. Código, informes, planes, diagramas y matrices pueden empezar en Markdown, YAML, JSON y CSV donde corresponda. HTML, Mermaid, PlantUML y grafos estructurados añaden renderizado. PDF, DOCX, XLSX, PPTX y dashboards requieren generadores y validación visual. Cada dominio define la aceptación del contenido.

### 49. Observabilidad y auditoría — viable desde el primer workflow

Registrar eventos de ejecución y decisión con IDs, versiones, duración, errores, costos conocidos y referencias permite historial, alertas, informes y seguimiento. El contrato también cubre agentes, modelos, harnesses, herramientas, skills, contexto, evidencia y permisos. OpenTelemetry y tracing distribuido cobran valor con varios procesos. Las cuotas dependen del proveedor. Las trazas deben filtrar secretos y respetar retención y acceso.

### 50. Aprendizaje continuo — viable mediante promoción controlada

El sistema puede aprender de correcciones, éxitos y fallos medidos. Puede proponer versiones de workflows, skills, patrones, agentes, modelos, herramientas, metodologías y evaluaciones. Los experimentos comparan variantes, los benchmarks apoyan la promoción y las versiones permiten revertir cambios perjudiciales. El inventario ya exige evitar cambios autónomos no autorizados y aprendizaje basado en errores sin verificar. El ciclo puede funcionar sin reentrenar modelos. ROMERO mantiene su aprobación de escritura propia.

### 51. Configuración de proyectos — viable sobre la CLI actual

Manifiesto, perfiles, plantillas, discovery, inicialización, validación, entornos, comparación, importación y exportación tienen un camino directo. Migración, backup, restauración y sincronización requieren versionado. La herencia global, organizacional, por usuario, proyecto, dominio, workflow, etapa, agente y tarea necesita precedencia visible.

### 52. Interfaces de administración — viables tras estabilizar operaciones

La CLI ya existe. Una TUI o web de solo lectura puede mostrar recomendaciones, ejecución, agentes, tareas, dependencias, evidencia, costos, benchmarks, conocimiento y auditoría. Aprobaciones, pausa, reanudación y cancelación usan las mismas políticas. Asistentes, selectores, comparadores y exploradores se apoyan en catálogos. Editores visuales y centros de administración completos dependen de contratos estables de edición y autorización.

### 53. Extensibilidad y distribución — viable por etapas

API pública, SDK y registros son candidatos cuando los contratos internos tengan varios consumidores. Los SDK de plugins, adaptadores, skills, agentes, workflows, evaluadores, dominios y herramientas pueden compartir versionado, ejemplos y pruebas de conformidad. Paquetes, dependencias, firma, integridad, importación, exportación, repositorios y actualizaciones controladas son viables. Los marketplaces se difieren por el costo adicional de adquisición de autores, moderación y mantenimiento.

### 54. Infraestructura y almacenamiento — viable desde local

Almacenamiento embebido transaccional y un directorio de artefactos pueden cubrir configuración, proyectos, ejecución, checkpoints, eventos, evaluaciones, decisiones, métricas y referencias. Diseñaría migración, respaldo y restauración desde el primer estado persistente. Bases remotas, objetos, caché externa, colas y despliegue distribuido se justifican por carga o recuperación. Windows, Linux, macOS, contenedores, servidores remotos y self-hosting requieren pruebas con los ejecutores elegidos.

### 55. Principios arquitectónicos — viables con comprobación

Los 40 principios pueden guiar la visión. Objetivos, especificación, evidencia, contratos, autonomía acotada, control humano, observabilidad, trazabilidad, seguridad y degradación explícita admiten criterios de aceptación. La intercambiabilidad se demuestra reemplazando un adaptador por otro compatible. La reproducibilidad se demuestra reconstruyendo decisiones y resultados registrados. Un principio nuevo de carácter obligatorio debe seguir el proceso constitucional.

### 56. Estado de la especificación maestra — viable como visión

El adjunto amplía el inventario conceptual. Cada integración, modo y responsabilidad necesita prioridad, especificación y evidencia antes de declararse entregado. Las ampliaciones de fábrica autónoma y sistema operativo cognitivo conservan el estado conceptual del contexto maestro hasta una decisión explícita. [Contexto maestro](../gandalf-wizard-contexto-maestro.md)

## Cambios de alcance y decisiones abiertas

- **Soporte activo de Roo Code oficial.** Queda fuera por el cierre del producto. Es un cambio de un destino de integración. La entrada histórica puede servir para detectar configuraciones antiguas.
- **Ciberseguridad y automodificación.** Se conserva la defensa y la promoción controlada de mejoras. La creación de exploits y los workflows ofensivos autónomos quedan fuera de este alcance defensivo. La automodificación sin autorización ya está excluida por el inventario.
- **Garantías condicionadas.** La paridad se verifica por capacidad y destino. La cuota exacta depende de datos que el proveedor expone. El rollback depende del tipo de efecto y del control sobre él. La exhaustividad se mide sobre un corpus y un acceso definidos.
- **L10 predeterminado.** Es una decisión de diseño abierta. La recomendación de activarlo según el caso modifica el valor inicial propuesto en el adjunto. Mantenerlo predeterminado es viable con alcance, presupuesto y cobertura explícitos.
- **Fases posteriores.** TUI, dashboard, dominios adicionales, ejecución distribuida y marketplace siguen en la visión. Se difieren por dependencias técnicas, evidencia de uso y mantenimiento.

## Secuencia recomendada

1. **Verificar la sincronización de la épica #28.** Generar una skill y una regla entre los destinos elegidos. Comprobar que la segunda ejecución conserva la salida, preserva ediciones locales e informa capacidades ausentes. [Épica #28](https://github.com/CodeMonkReaper/gandalf-wizard/issues/28)
2. **Ejecutar un workflow pequeño de extremo a extremo.** Una solicitud, un objetivo verificable, dos tareas, un checkpoint, una aprobación y un informe de evidencia. Probar recuperación tras interrupción y comprobar que un reintento conserva un único efecto.
3. **Medir valor en Software Engineering y Documentación.** Comparar una tarea real por dominio contra un flujo directo en un solo harness. Medir precisión, cobertura, tiempo y consumo antes de ampliar agentes, adaptadores o interfaces.
