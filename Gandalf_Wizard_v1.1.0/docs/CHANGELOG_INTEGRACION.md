# Registro de consolidación — v1.1.0-draft

## Resolución documental de los doce hallazgos

| Hallazgo | Cambio incorporado | Estado después de consolidar |
|---|---|---|
| 1. IDs de pruebas duplicados | C.3, AC-17–24 y tabla de equivalencia del PDF. | Resuelto documentalmente. |
| 2. Responsabilidades GW-51/52/57 | GW-51 posee configuración, GW-52 contratos, GW-57 experiencia gráfica. | Resuelto documentalmente. |
| 3. Seguridad y políticas | GW-57.b, modo Expert sin privilegios extra; AC-20. | Norma integrada; implementación pendiente. |
| 4. Terminal y editor | GW-57.a/b, anexo §4.2, OPEN-14. | Controles requeridos; sandbox por diseñar/probar. |
| 5. Cambios durante ejecución | GW-57.c, revisión inmutable, puntos seguros y límites por modalidad; AC-22. | Norma integrada; protocolo pendiente. |
| 6. Independencia de Romero | GW-57.d, puerto/adaptador y sistema externo; AC-23. | Corrección normativa; PDF histórico preservado. |
| 7. Stack tecnológico | GW-57.d, OPEN-12, anexo §6. | Candidatos explícitos; decisión física pendiente. |
| 8. Capacidades reales | GW-57.b/c/d y AC-23, estados de GW-03 preservados. | Norma integrada; conformidad real pendiente. |
| 9. Contratos incompletos | A.10, anexo §4, OPEN-13. | Contratos lógicos integrados; esquemas y firmas pendientes. |
| 10. Administración universal | Matriz de superficies por 57 áreas y seis grupos. | Cobertura a nivel de área; descriptores detallados pendientes. |
| 11. Nomenclatura | GW-57.1–6, perfiles y modos de UI separados; alias históricos. | Normalización documental; esquema final pendiente. |
| 12. Control documental | Versión 1.1.0-draft, índice, DEC-11, GW-56, anexos A/C/D/E y roadmap transversal. | Integrado. |

## Verificación estructural realizada

- Se verificaron los 57 encabezados GW únicos y consecutivos.
- Se compararon las 10 decisiones originales y las 16 filas originales de aceptación: permanecen literalmente idénticas.
- Se compararon las secciones GW-01–56: 53 permanecen literalmente idénticas; GW-51, GW-52 y GW-56 contienen las modificaciones puntuales descritas. Los demás cambios se concentran en control documental y anexos.
- Se verificaron los 24 IDs de escenarios únicos y consecutivos, y los 57 registros de la matriz de superficies.
- Se verificaron los enlaces relativos a archivos del paquete, y la identidad SHA-256 del PDF histórico con el PDF entregado.

Estas son comprobaciones documentales automatizadas, no tests de código, contratos ejecutables, integración, seguridad o cumplimiento real. La versión conserva el sufijo draft. Las prioridades, IDs técnicos propuestos, distribución de fases y criterios ampliados son decisiones de consolidación de la auditoría autorizada, no evidencia de que el repositorio ya las soporte.

## Próximo paso técnico

Inspeccionar Gandalf y Romero reales, resolver OPEN-01–14 y formalizar los esquemas GUAF y A.10 mediante ADR, pruebas de conformidad y una implementación vertical. Verificar el ID de ADR contra el registro real antes de un commit.
