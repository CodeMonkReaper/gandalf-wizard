# ADR-001 — Incorporación de Gandalf Control Center

- Estado: aceptada para alcance e integración documental; decisiones físicas pendientes.
- Fecha: 2026-11-19.
- Alcance: DEC-11, GW-57, GW-51–52, GW-01–04, GW-14, GW-19–24, GW-39–43, GW-49, GW-55 y AC-17–24.
- Contexto: GW-SPEC-001 v1.0.0-draft prevé un dashboard y editores; el PDF GW-57 v0.1 explicita la administración interactiva de extremo a extremo.
- Decisión: implementar Control Center como cliente de los servicios comunes del núcleo Go, con capacidades configurables, composición, observación y ejecución autorizada. Mantener fuente de verdad común, políticas obligatorias, portabilidad, Romero externo mediante puerto y revisiones de plan inmutables.
- Alternativas: limitarse a CLI/TUI (insuficiente para DEC-11); construir un runtime paralelo en UI (rechazado por duplicación e inconsistencias); cliente integrado (seleccionado).
- Consecuencias: contratos de administración y autorización compartidos; nuevo alcance gráfico y mayor superficie de seguridad en terminal/editor; pruebas de paridad, concurrencia, aislamiento y capacidades. No se exige implementar todo en el MVP.
- Migración: conservar IDs GW-01–56, DEC-01–10 y AC-01–16. Incorporar GW-57.1–6, DEC-11 y AC-17–24; el PDF histórico permanece íntegro, con equivalencia explícita de IDs.
- Evidencia: auditoría comparativa de los dos documentos y autorización del propietario para ejecutar la consolidación; no se han ejecutado pruebas ni inspeccionado código.
- Aprobación: se interpreta la instrucción del propietario «hazlo» como ratificación de la incorporación de alcance propuesta en la auditoría precedente, no como aprobación de tecnologías, firmas, protocolos o garantías de implementación. Esas decisiones requieren ADR y evidencia posterior.
- Numeración: ADR-001 es un identificador local propuesto para este paquete; verificar colisiones antes de integrarlo al repositorio real.
