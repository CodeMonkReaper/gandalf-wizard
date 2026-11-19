# Matriz de trazabilidad y superficies — GW-SPEC-001 v1.1.0-draft

Fuente canónica: [especificación maestra](GANDALF_WIZARD_ESPECIFICACION_MAESTRA.md). Mapeo de cobertura de alto nivel de 57 áreas, no inventario exhaustivo de campos ni evidencia de implementación. La clasificación es una propuesta de diseño de esta consolidación; los descriptores concretos y acciones deberán definirse y probarse por recurso. Todos los requisitos y pruebas tienen estado de implementación **no verificado**.

Clases combinables: **C** configurable (parámetros expuestos), **P** componible, **X** extensible, **O** operable, **V** observable, **R** restringido por autoridad/capacidad, **I** invariantes o registros protegidos (no editables por preferencias ordinarias). Una clase se aplica a la parte correspondiente del recurso, no implica que toda el área pueda modificarse. Todas las operaciones, incluida lectura, respetan permisos; ninguna clase concede autoridad. GW-56 se observa como estado documental, no se edita como configuración ordinaria.

| ID | Área maestra | Superficie de administración | Clases | Aceptación base | GW-57 vinculado | Aceptación panel |
|---|---|---|---|---|---|---|
| GW-01 | Núcleo universal de Gandalf | Contratos del núcleo, configuración, versión, estado | C V I | AC-01 | 57.3, 57.6 | AC-17, AC-20 |
| GW-02 | Modalidades de funcionamiento | Modalidades y topologías de ejecución | C P O R | AC-01 | 57.3, 57.5 | AC-22 |
| GW-03 | Integraciones con harnesses | Adaptadores, instalaciones, compatibilidad | C X V R | AC-01, AC-04 | 57.1, 57.3 | AC-23 |
| GW-04 | Integración con Romero | Proveedor Romero, referencias y permisos | C X V R | AC-01, AC-09 | 57.1, 57.3, 57.5 | AC-23 |
| GW-05 | Objective Engine — ODD | Objetivos, criterios y restricciones | C P R | AC-01 | 57.1, 57.3 | AC-17, AC-20 |
| GW-06 | Universal Registry System | Registro, versiones, publicación, dependencias | C X O R | AC-02 | 57.1, 57.6 | AC-21 |
| GW-07 | Sistema universal de skills | Skills, manifiestos, recursos, generación | C P X R | AC-02 | 57.1, 57.3 | AC-20, AC-24 |
| GW-08 | Categorías de skills | Familias y etiquetas de skills | C X V | AC-02 | 57.1 | AC-24 |
| GW-09 | Patrones de diseño de software | Patrones de diseño | C P X | AC-02 | 57.1, 57.4 | AC-24 |
| GW-10 | Patrones arquitectónicos | Patrones arquitectónicos | C P X | AC-02 | 57.1, 57.4 | AC-24 |
| GW-11 | Patrones de agentes de IA | Patrones de agentes | C P X | AC-02 | 57.1, 57.4 | AC-19, AC-24 |
| GW-12 | Metodologías y enfoques de desarrollo | Metodologías, fases y reglas | C P X | AC-02 | 57.3, 57.4 | AC-19 |
| GW-13 | Anti-patrones | Antipatrones, diagnósticos y reglas | C X V R | AC-02 | 57.1, 57.3 | AC-20, AC-24 |
| GW-14 | Motor universal de workflows | IR, nodos, gates, runtime | C P O R | AC-03 | 57.4, 57.5 | AC-19, AC-22 |
| GW-15 | Workflow maestro universal | Flujo maestro, etapas y composición | C P R | AC-03 | 57.4, 57.5 | AC-19, AC-22 |
| GW-16 | Motor de dominios | Dominios, perfiles y composición | C P X | AC-03 | 57.4 | AC-19, AC-24 |
| GW-17 | Dominios y modos de especialización del producto final | Modos de especialización | C P X | AC-03 | 57.4 | AC-24 |
| GW-18 | Estrategias de ejecución | Estrategias y restricciones | C P R | AC-03 | 57.2, 57.4 | AC-19, AC-22 |
| GW-19 | Autonomía | Autonomía, intervención y aprobación | C O R I | AC-03 | 57.3, 57.5, 57.6 | AC-20, AC-22 |
| GW-20 | Model Registry | Modelos, esfuerzos, costos y capacidades | C X V R | AC-05 | 57.1, 57.2 | AC-18, AC-23 |
| GW-21 | Model Router — recomendación modificable | Routing, pesos, fijaciones y fallback | C O V R | AC-05 | 57.2 | AC-18 |
| GW-22 | Optimizador de suscripciones, cuotas y recursos | Suscripciones, cuotas, recursos y presupuesto | C O V R | AC-05 | 57.2, 57.5 | AC-18, AC-20 |
| GW-23 | Orquestación multiagente | Agentes, equipos, delegación y handoffs | C P O R | AC-06 | 57.4, 57.5 | AC-19, AC-22 |
| GW-24 | Compilador universal de harnesses | Compilación, adaptadores y diffs | C X O R | AC-04 | 57.4, 57.6 | AC-19, AC-23 |
| GW-25 | Software Engineering Mode | Software, editores, pruebas y gates | C P O R | AC-07 | 57.3, 57.4 | AC-19, AC-20 |
| GW-26 | Entrepreneur Mode | Entrepreneur, hipótesis, experimentos y score | C P O R | AC-08 | 57.3, 57.4 | AC-24 |
| GW-27 | Metodologías y frameworks de Entrepreneur | Frameworks empresariales | C P X | AC-08 | 57.1, 57.4 | AC-24 |
| GW-28 | Agentes de Entrepreneur | Roles y comité Entrepreneur | C P O R | AC-08 | 57.4 | AC-19, AC-24 |
| GW-29 | Documentation Mode — Deep Documentation Intelligence | Corpus, flujo documental, análisis y cobertura | C O V R | AC-09 | 57.3, 57.5 | AC-23, AC-24 |
| GW-30 | Especializaciones documentales | Especializaciones documentales | C P X R | AC-09 | 57.3, 57.4 | AC-24 |
| GW-31 | Niveles de profundidad documental | Profundidad L1–L10 y límites | C V R | AC-09 | 57.3 | AC-17, AC-23 |
| GW-32 | Metodologías documentales | Metodologías documentales | C P X | AC-09 | 57.4 | AC-24 |
| GW-33 | Agentes documentales | Especialistas, revisores y equipos | C P O R | AC-09 | 57.4 | AC-24 |
| GW-34 | Evidence Engine y procedencia | Evidencia, procedencia y verificación | V O R I | AC-09 | 57.1, 57.5, 57.6 | AC-20, AC-23 |
| GW-35 | Legal Intelligence | Perfiles legales, jurisdicción y vigencia | C P V R | AC-10 | 57.3, 57.4 | AC-24 |
| GW-36 | Deep Research | Planes y fuentes de investigación | C P O R | AC-10 | 57.3, 57.4 | AC-24 |
| GW-37 | Engineering y Architect | Ingeniería, arquitectura y herramientas | C P X R | AC-10 | 57.3, 57.4 | AC-24 |
| GW-38 | Context Engine | Contexto, presupuestos y transferencias | C O V R | AC-11 | 57.3, 57.5 | AC-20, AC-23 |
| GW-39 | Herramientas e integraciones | Herramientas, MCP, APIs y terminal | C X O R | AC-11 | 57.3, 57.5 | AC-20 |
| GW-40 | Execution Engine y runtime | Runtime, estados, reintentos y checkpoints | C O V R | AC-06, AC-11 | 57.5, 57.6 | AC-22 |
| GW-41 | Repositorios y código | Repositorios, archivos, Git y editor | C O V R | AC-06, AC-11 | 57.3, 57.5 | AC-20 |
| GW-42 | Policy Engine y reglas | Políticas, permisos, reglas y conflictos | C O R I | AC-11 | 57.3, 57.6 | AC-20 |
| GW-43 | Security Engine | Seguridad, secretos, identidad y revocación | C O V R I | AC-11 | 57.3, 57.6 | AC-20 |
| GW-44 | Evaluation Engine y Quality Gates | Rúbricas, criterios y quality gates | C X O R | AC-12 | 57.3, 57.4 | AC-19, AC-20 |
| GW-45 | Evaluación competitiva | Arenas, comparaciones y rankings | C O V R | AC-12 | 57.3, 57.5 | AC-24 |
| GW-46 | Benchmark Engine | Benchmarks, datasets y versiones | C X O V R | AC-12 | 57.1, 57.3 | AC-24 |
| GW-47 | Motores especializados de inteligencia | Parámetros y resultados de motores especializados | C X V R I | AC-12 | 57.3 | AC-24 |
| GW-48 | Artefactos y entregables | Artefactos, exportaciones y versiones | C O V R | AC-13 | 57.1, 57.5, 57.6 | AC-21, AC-24 |
| GW-49 | Observabilidad y auditoría | Observabilidad, eventos, auditoría y alertas | C O V R I | AC-13 | 57.5, 57.6 | AC-21, AC-22 |
| GW-50 | Meta-harness adaptativo y aprendizaje continuo | Propuestas de aprendizaje y promoción | C O V R I | AC-14 | 57.3, 57.6 | AC-20, AC-21 |
| GW-51 | Configuración universal de proyectos | Configuración, perfiles, entornos y restauración | C O V R | AC-15 | 57.1, 57.3, 57.6 | AC-17, AC-21 |
| GW-52 | Interfaces y experiencia de administración | Interfaces, asistentes y aprobación | C O V R | AC-15 | 57.1–6 | AC-17, AC-24 |
| GW-53 | Extensibilidad y distribución | SDK, extensiones, marketplace y distribución | C X O R | AC-15 | 57.1, 57.3, 57.4 | AC-20, AC-24 |
| GW-54 | Infraestructura y almacenamiento | Instalaciones, persistencia, backups y despliegue | C O V R | AC-15 | 57.1, 57.6 | AC-20, AC-21 |
| GW-55 | Principios arquitectónicos | Principios e invariantes arquitectónicos | V I | AC-16 | 57.3, 57.6 | AC-20 |
| GW-56 | Estado y alcance de la especificación maestra | Alcance, madurez y limitaciones documentales | V I | AC-16 | 57.1, 57.5 | AC-23, AC-24 |
| GW-57 | Gandalf Control Center — administración interactiva universal | Control Center, experiencia y administración | C P X O V R I | AC-17–24 | 57.1–6 | AC-17–24 |

## Trazabilidad de los seis grupos del PDF

| PDF v0.1 | ID canónico | Núcleo relacionado | AC global |
|---|---|---|---|
| 57.01 | GW-57.1 | GW-06, GW-49, GW-51–54 | AC-17, AC-23, AC-24 |
| 57.02 | GW-57.2 | GW-20–22 | AC-18, AC-24 |
| 57.03 | GW-57.3 | GW-01, GW-19, GW-42–43, GW-51 | AC-17, AC-20, AC-23, AC-24 |
| 57.04 | GW-57.4 | GW-14–17, GW-23–24, GW-52–53 | AC-19, AC-24 |
| 57.05 | GW-57.5 | GW-14, GW-19, GW-40, GW-49 | AC-22, AC-23, AC-24 |
| 57.06 | GW-57.6 | GW-06, GW-42–43, GW-49, GW-51 | AC-17, AC-20, AC-21, AC-24 |

Cada caso de implementación debe vincular IDs específicos, fixtures, evidencia y decisiones; no declarar cobertura de campos desconocidos únicamente por este mapeo. Las referencias `57.x` de la columna se interpretan como `GW-57.x`.
