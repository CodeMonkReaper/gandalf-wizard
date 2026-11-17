# Revisión de Gandalf Wizard

Estado: ratificado por el propietario el 2026-11-17 junto con la [constitución v1.0.0](02-DOCS/wiki/sdd/constitution.md).

Leer la constitución, la especificación aplicable y las decisiones antes del diff. Iniciar el informe con el número de hallazgos y su severidad; cada hallazgo debe incluir archivo, criterio incumplido, consecuencia y evidencia.

Buscar especialmente capacidades declaradas sin prueba, permisos ampliados, secretos, confusión entre controles consultivos y deterministas, decisiones de producto no aprobadas y código fuente de RSC introducido en Gandalf. Comprobar que un relevo conserva tareas y evidencia en `02-DOCS/`.

Revisar los archivos fuente de conocimiento y `.rsc.json`; para copias generadas en `.rsc/`, `.agents/skills/`, `.vibe/skills/` y `.codex/rsc/`, comprobar procedencia e igualdad con la base canónica. Revisar el fork en su repositorio independiente. Limitar observaciones de estilo a tres; priorizar defectos verificables.

Requieren intervención humana los cambios de constitución, permisos, secretos, integraciones externas y publicación remota. Esta lista es consultiva; no se ha configurado un `risk.raise` específico ni una protección remota de ramas. El stack y las rutas de código de producto siguen sin decidirse.
