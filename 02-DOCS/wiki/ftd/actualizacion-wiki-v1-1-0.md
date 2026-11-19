---
author: Luis Haroldo Salamanca Fernandéz
branch: master
status: in-progress
---
# Actualización de la wiki con GW-SPEC-001 v1.1.0-draft

## Intent

Incorporar al índice y al checkpoint vigente la consolidación documental entregada el 19 de noviembre de 2026.

## Scope

Actualizar la wiki y sus referencias históricas. El paquete de origen conserva sus archivos y su ubicación. La implementación y sus aprobaciones siguen el proceso ASDD.

## Checklist

- [x] Confirmar que el índice llega al checkpoint y a todos los documentos de origen.
- [x] Contrastar el resumen con las áreas, decisiones, escenarios y pendientes del paquete.
- [x] Validar los enlaces locales de la wiki y comprobar que el diff se limita a documentación.
- [x] Preparar en Git la wiki, el paquete y el retiro del inventario anterior. Volver a ejecutar la suite de Go con el índice actualizado.

## Evidence

- El [índice](../index.md) lleva al [checkpoint vigente](../sdd/sessions/2026-11-19-consolidacion-v1-1-0.md). Entre ambos enlazan los ocho archivos del paquete.
- La comprobación estructural confirmó 57 áreas, 11 decisiones, 14 asuntos abiertos, 24 escenarios globales y 57 registros de trazabilidad. Los SHA-256 de los ocho archivos fuente coinciden con los anteriores al cambio.
- Se comprobaron 221 enlaces locales en 58 documentos de la wiki y del paquete. Se corrigieron cinco rutas de retorno al índice y dos rutas a la guía de revisión. El inventario histórico permanece accesible en Git y `git diff --check` pasó para la wiki.
- La primera suite de Go pasó en 10 de 11 paquetes. `TestRepoSecurityGate` falló mientras el índice de Git aún listaba `gandalf-wizard-contexto-maestro.md`, que ya faltaba en la copia de trabajo. Tras preparar su retiro junto con la wiki y los ocho archivos del paquete, `go test -count=1 ./...` pasó en los 11 paquetes y `git diff --cached --check` validó el cambio preparado.

## Next

Registrar en un commit el cambio documental preparado, de modo que la wiki y sus fuentes lleguen juntas a otros clones.
