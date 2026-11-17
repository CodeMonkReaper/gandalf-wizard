---
type: constitution
title: Gandalf Wizard — Constitución
description: Principios de desarrollo ratificados por el propietario.
tags: [sdd, constitution]
timestamp: 2026-11-17T23:28:00Z
topic: sdd
version: v1.2.0
ratified: 2026-11-17
last-amended: 2026-11-17
---

# Gandalf Wizard — Constitución

> Versión: v1.2.0. Ratificada por el propietario: 2026-11-17. Última enmienda: 2026-11-17.
> Los diez principios y REVIEW.md fueron aceptados explícitamente tras revisar el borrador. Las decisiones abiertas requieren resolución antes de la implementación que dependa de ellas.

## Hechos y alcance

- Objetivo: configurar, generar, ejecutar, supervisar, evaluar y optimizar entornos asistidos por IA. Contexto conceptual: [contexto maestro](../../../gandalf-wizard-contexto-maestro.md).
- Proyecto software complejo; workflow aprobado: issues, ramas y pull requests. Rama actual por defecto: `master`; remoto público autorizado en `origin`: [CodeMonkReaper/gandalf-wizard](https://github.com/CodeMonkReaper/gandalf-wizard).
- El contexto maestro contiene propuestas de producto, arquitectura y stack; requieren decisiones y especificaciones aprobadas.
- Existen convenciones iniciales de Go en [go.md](../stack/go.md), módulo `go.mod` y paquetes iniciales de Foundation 0.1 en desarrollo bajo rama aislada.

## Principios ratificados

1. **Separación de repositorios.** Mantener Gandalf y el fork RSC independientes. En Gandalf instalar únicamente los recursos normales del harness; comprobar el árbol y los remotos antes de integrar cambios.
2. **Decisiones explícitas.** Registrar cada decisión significativa con fecha, alternativas, motivo y estado en [decisions.md](decisions.md). Una propuesta del contexto maestro conserva ese estado hasta aprobación explícita.
3. **ASDD antes de implementar.** Ratificar la constitución y aprobar la especificación antes de código de producto. Vincular especificación, plan, tareas y criterios de aceptación mediante las skills `constitution`, `specify`, `plan`, `tasks` y `analyze`.
4. **Evidencia observable.** Registrar comandos, resultados y límites en `verifications/`. Cada capacidad declarada operativa debe tener evidencia; clasificar las demás como adaptadas, consultivas, no soportadas o no verificadas. Aplicar `verify` y `review`.
5. **Mínimo privilegio.** Conservar las aprobaciones del asistente. Requerir autorización humana para servicios MCP externos, recursos remotos, pushes y PR; proteger credenciales de repositorios y documentos. Comprobar configuración local y diff con `security-audit`.
6. **Controles comprobables.** Describir como determinista solo un control con mecanismo de ejecución y pruebas de paso y bloqueo pertinentes. Las instrucciones de rol o metodología se revisan como instrucciones consultivas.
7. **Integración por Issues y PR.** Toda feature, tarea o cambio sustancial debe contar con un Issue abierto en GitHub ([CodeMonkReaper/gandalf-wizard](https://github.com/CodeMonkReaper/gandalf-wizard/issues)) desde el inicio del trabajo, desarrollarse en ramas y proponerse mediante PR vinculado al Issue con aprobación humana. La configuración del workflow no equivale a protección remota de rama.
8. **Autoría humana.** Utilizar la identidad Git del responsable; omitir atribuciones de coautoría de IA y pies de generación. Revisar en la fase `ship`.
9. **Continuidad documentada.** Antes de un relevo, actualizar tareas, decisiones, evidencias, el Issue correspondiente y un checkpoint en `sessions/`. Leer [el índice](../index.md) al retomar; no usar un historial privado como fuente canónica del estado.
10. **Una fuente de skills.** Mantener `.rsc/skills/` como base canónica, `.rsc.json` como declaración y copias administradas por RSC en Windows. Cambiar conocimiento en `02-DOCS/`; instalar y sincronizar skills con el fork fijado.

## Decisiones pendientes

- **Stack canon y transporte inicial (resuelto 2026-11-17):** Go (`go1.27.3`, módulos Go con `go.mod`, arquitectura inicial de monolito modular / CLI sidecar) y transporte inicial REST sobre el contrato público `pkg/romeroclient` de ROMERO (con puerto `RomeroReadPort` preparado para sumar MCP después).
- **Comandos verificables de calidad en Go (resuelto 2026-11-17):** formato con `gofmt -l .` (salida vacía), análisis estático con `go vet ./...` y pruebas automatizadas con `go test ./...` (TDD estricto). Los umbrales de cobertura global y gates de CI remoto se concretarán al habilitar remoto.
- Nombres de ramas/commits, organización del código y contratos de API: abiertos. Emoji activado es una preferencia del harness; no impone gitmoji.
- Alcance inicial, contratos ROMERO, autonomía y políticas del producto: pendientes de especificación y aprobación.
- UI, accesibilidad y presupuestos de rendimiento: definir si corresponden al alcance aprobado y con métricas observables.

## Definition of Done

- [ ] Constitución ratificada y especificación aprobada; decisiones pendientes relevantes resueltas.
- [ ] Criterios de aceptación cubiertos por comprobaciones con evidencia y límites explícitos.
- [ ] Comandos de calidad acordados para el stack ejecutados satisfactoriamente.
- [ ] Revisión de permisos, secretos y controles pertinentes completada.
- [ ] Issue vinculado actualizado; cambios, decisiones y checkpoint documentados; integración por PR autorizada.

## Registro de enmiendas

| Date | Version | Change | Why |
|------|---------|--------|-----|
| 2026-11-17 | draft | Draft generated by `rsc onboard`. | The SDD floor needs a constitution; ratification is the `constitution` phase's. |
| 2026-11-17 | draft | Propuestos diez principios derivados de las instrucciones del propietario. | Preparar ratificación sin elegir stack ni requisitos funcionales. |
| 2026-11-17 | v1.0.0 | Ratificados los diez principios y REVIEW.md tras revisión del propietario: «esta bien acepto». | Aprobación explícita de la base ASDD; stack, arquitectura, calidad y alcance inicial permanecen abiertos. |
| 2026-11-17 | v1.1.0 | Aprobados por el propietario el stack Go (`go1.27.3`), transporte inicial REST (`pkg/romeroclient`), aislamiento en worktree/rama y controles `go test ./...`, `go vet ./...` y `gofmt -l .`. | Cerrar decisiones de stack, transporte y gates de calidad antes de desglosar `tasks` e implementar Foundation 0.1. |
| 2026-11-17 | v1.2.0 | Autorizado repositorio público `CodeMonkReaper/gandalf-wizard` y regla obligatoria de trabajar siempre con Issues en GitHub vinculados a especificación, plan, progreso y PR. | Garantizar trazabilidad completa en repositorio público desde el origen. |
