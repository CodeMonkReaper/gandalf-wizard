# Matriz global de aceptación — GW-SPEC-001 v1.1.0-draft

Extracto sincronizado de C.3 y C.4 de la [especificación canónica](GANDALF_WIZARD_ESPECIFICACION_MAESTRA.md). AC-01–16 se preservan; AC-17–24 reemplazan los IDs locales AC-01–08 del PDF de GW-57. No hay resultados de pruebas reales; los 24 escenarios están pendientes de ejecución/verificación. Los casos detallados, umbrales y fixtures deberán formalizarse según C.4.

## Escenarios globales

| ID de prueba | Requisitos | Resultado esperado y evidencia |
|---|---|---|
| AC-01 | GW-01–05 | Core Go compila, objetivo versionado y plan trazable; Romero se consume por interfaz, sin dependencia de implementación. |
| AC-02 | GW-06–13 | Catálogo propio admite versiones, dependencias y skills portables; componente inválido/conflicto metodológico rechazado con diagnóstico. |
| AC-03 | GW-14–19 | Workflow con tareas paralelas y dependencias, checkpoints, cancelación, ciclo limitado, gates y aprobación; estado recuperable en test de fallo. |
| AC-04 | GW-03, GW-24 | Compilar y ejecutar la misma IR en dos harnesses; verificar capacidades, preservar configuración, bloquear funcionalidad obligatoria no soportada. |
| AC-05 | GW-20–22 | Recomendación explicable y modificable; override incompatible bloqueado; fallback autorizado; cuota desconocida señalada; presupuesto no excedido por reservas concurrentes dentro del control de Gandalf. |
| AC-06 | GW-23, GW-40–41 | Delegación acotada y aislamiento de agentes/archivos; detección de conflictos, recuperación o fallo seguro. |
| AC-07 | GW-25 | Feature de prueba con requisitos, especificación, código, tests, review, gates y artefactos verificables. |
| AC-08 | GW-26–28 | Caso Entrepreneur con supuestos, evidencia, viabilidad, alternativas, contraargumentos y decisión fundamentada; sin compromisos externos no autorizados. |
| AC-09 | GW-04, GW-29–34 | Corpus de prueba con versiones, contradicciones intencionales y fuentes no accesibles; L10 registra cobertura, hallazgos, referencias válidas y limitaciones; originales intactos. |
| AC-10 | GW-35–37 | Casos legal, research e ingeniería con criterios sectoriales, referencias, limitaciones y revisión profesional identificada cuando corresponda. |
| AC-11 | GW-38–43 | Pruebas de aislamiento, permisos, prompt injection, datos sensibles, secretos, fallos y revocación; acciones no autorizadas rechazadas y auditadas. |
| AC-12 | GW-44–47 | Gates deterministas y evaluación independiente/competitiva con resultados, métricas y criterios versionados; soluciones fallidas no declaradas exitosas. |
| AC-13 | GW-48–49 | Artefactos versionados y validados, trazabilidad completa de ejecución y decisión sin registro de secretos. |
| AC-14 | GW-50 | Propuesta de mejora evaluada contra baseline y dataset de reserva; promoción o descarte según política; rollback demostrado. |
| AC-15 | GW-51–54 | Configuración con herencia y overrides, interfaces y SDK conforme a contratos, instalación/backup/restauración probados para plataformas soportadas. |
| AC-16 | GW-55–56 | Matriz de trazabilidad completa, decisiones registradas, limitaciones explícitas y revisión de aceptación del propietario. |
| AC-17 | GW-57.1, GW-57.3, GW-57.6, GW-51–52 | **Paridad:** partiendo de revisiones equivalentes, aplicar el mismo cambio autorizado por panel, CLI y API; comparar configuración efectiva, procedencia y semántica de versionado; verificar TUI/SDK si están disponibles. Rechazar conflicto de revisión y permisos insuficientes. |
| AC-18 | GW-57.2, GW-20–22 | **Router:** editar pesos, normalizar suma positiva, fijar modelo/esfuerzo, restaurar recomendación y mostrar motivos, límites, incertidumbre y auditoría; rechazar pesos inválidos, candidatos no soportados y fallback no autorizado. |
| AC-19 | GW-57.4, GW-14, GW-23–24 | **Workflow:** crear/editar nodos y aristas, validar ramas, ciclos y referencias, importar/exportar IR sin pérdida semántica; separar layout, bloquear capacidades obligatorias ausentes y previsualizar compilación sin mutación. |
| AC-20 | GW-57.3, GW-57.6, GW-19, GW-39, GW-41–43 | **Seguridad:** rechazar mutaciones, comandos, escrituras y escalamiento no autorizados, incluso por Expert, terminal, asistente y API directa; validar permisos en la acción y no exponer secretos. |
| AC-21 | GW-57.6, GW-06, GW-49, GW-51 | **Versionado:** diff, restauración como nueva revisión, auditoría, integridad, conflicto concurrente y trazabilidad; comprobar que la simulación no muta estado externo y que restaurar configuración no revierte efectos externos automáticamente. |
| AC-22 | GW-57.5, GW-14, GW-19, GW-40 | **Runtime:** observar eventos, pausar/reanudar/cancelar donde se soporte; aplicar nueva revisión en punto seguro con aprobación vigente; mantener tarea en curso y plan anterior; probar modo compilador sin control, reconciliación y limitaciones explícitas. |
| AC-23 | GW-57.1, GW-57.3, GW-57.5, GW-03–04, GW-24, GW-29–34 | **Integración:** mostrar capacidades y procedencia reales de harnesses y Romero, distinguir datos declarados/observados/estimados/desconocidos y estados de compatibilidad; no acceder a originales o funciones no autorizadas. |
| AC-24 | GW-57.1–6, GW-52 | **Experiencia:** verificar navegación, accesibilidad, Recommended/Advanced/Expert, estados vacíos/cargando/error/bloqueado, configuración efectiva y cobertura de acciones por las 57 áreas; el modo Expert no concede permisos adicionales. |

Estos son escenarios de aceptación, no pruebas ejecutadas. Antes de aceptar una release se requieren casos de prueba concretos, datasets, umbrales, ambiente, resultados observados y firma o aprobación del responsable según política.

## C.4. Definition of Done de una funcionalidad de Gandalf

1. Requisito y criterios vinculados a `GW-xx` y una decisión de alcance.
2. Contrato versionado; comportamiento de éxito, fallos, cancelación y seguridad definido.
3. Implementación integrada sin violar dependencias del núcleo.
4. Tests unitarios, contractuales, integración y regresión adecuados a riesgo y alcance.
5. Documentación de uso, configuración, compatibilidad y limitaciones actualizada.
6. Evidencia de pruebas realmente ejecutadas y resultados, con gate de aprobación aplicable.
7. Observabilidad, recuperación y rollback donde sean necesarios; revisión de cambios y seguridad.
8. Estado final registrado: completo, parcial, bloqueado o rechazado; nada se declara implementado sin evidencia.


## Equivalencia de IDs históricos del PDF

| ID local (PDF) | ID global |
|---|---|
| AC-01 | AC-17 |
| AC-02 | AC-18 |
| AC-03 | AC-19 |
| AC-04 | AC-20 |
| AC-05 | AC-21 |
| AC-06 | AC-22 |
| AC-07 | AC-23 |
| AC-08 | AC-24 |
