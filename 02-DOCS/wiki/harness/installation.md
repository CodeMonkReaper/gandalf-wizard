---
type: reference
title: Instalación RSC para Gandalf Wizard
description: Fuente fijada, operación segura, continuidad y límites verificados.
tags: [harness, installation, vibe, codex, antigravity]
timestamp: 2026-11-17T21:22:41Z
topic: harness
---

# Entorno RSC

Instalado con el onboarding oficial del fork, aceptado por el propietario:

- Fuente: `github:CodeMonkReaper/rsc-harness-vibe#2b37e98870eaaca9980bf903381d5ae5aacd5280`.
- Upstream MIT: `ericrisco/rsc-harness`, commit `260396d2cc71746bd3335fd191cc3f5ffc3e4567`. Fork local independiente: `C:/Users/luis_/orca/projects/rsc-harness-vibe`; remotos `upstream` y `origin` separados.
- Plan: `12fae90dc27e149c2fa4099d8e4113c84f45d93f934bd07cd01b3bb84012e859`.
- technical; software; complex; branches + PR; emoji on; targets vibe, codex, antigravity; 33 skills.
- Resultado: `RSC_ONBOARDING_READY`. Este estado no ratifica la constitución.

## Operación y reproducción

En PowerShell, dentro del proyecto, fijar la fuente durante el onboarding. En otra máquina generar primero un plan y aceptar únicamente su ID revisado; no reutilizar un ID si cambió la evidencia del proyecto.

```powershell
$env:RSC_FORK_REF = 'github:CodeMonkReaper/rsc-harness-vibe#2b37e98870eaaca9980bf903381d5ae5aacd5280'
npm exec --yes "--package=$env:RSC_FORK_REF" -- rsc onboard --technical-level technical --project-kind software --software-scope complex --workflow branches --emoji on --target vibe,codex,antigravity --goal '<objetivo aprobado>'
```

Para un clon con `.rsc.json` ya aprobado, reconstruir los recursos locales con `sync`. La siguiente función es una comodidad de sesión PowerShell, no una instalación global:

```powershell
function rsc-fork {
  npm exec --yes --package=github:CodeMonkReaper/rsc-harness-vibe#2b37e98870eaaca9980bf903381d5ae5aacd5280 -- rsc @args
}
rsc-fork sync --target vibe,codex,antigravity
rsc-fork doctor --target vibe,codex,antigravity --json
rsc-fork capabilities --target vibe,codex,antigravity --json
```

Gestión existente: `add <skill>`, `uninstall <skill>`, `repair --dry-run`, `backups` y `restore <id> --dry-run`; ejecutar `restore <id>` solo tras revisar el alcance del snapshot. Los backups locales están bajo `.rsc/backups/` y no viajan por Git. Las copias instaladas se regeneran; el conocimiento editable pertenece a `02-DOCS/`.

`.rsc.json` conserva `forkSource`. El fork suprime su auto-update a npm upstream para proyectos que declaran Vibe y rechaza `upgrade`. Para actualizar: revisar y probar cambios en el repositorio del fork, publicar únicamente con autorización y adoptar un nuevo commit de forma explícita. No ejecutar el paquete upstream `@ericrisco/rsc` para mantener esta instalación.

La preparación local está en `chore/rsc-development-environment`, desde `master`, sin commits nuevos ni publicación de Gandalf. Once pruebas del módulo de backups de RSC pasaron, incluida restauración real en temporales y rechazo de rutas que salen del proyecto; no se restauró Gandalf. Las 99 copias de skills instaladas se compararon mediante SHA-256 con la base: cero divergencias.

## Recursos compartidos y continuidad

`.rsc/skills/` contiene el catálogo instalado canónico. RSC genera copias administradas en Windows: `.vibe/skills/`, `.codex/rsc/` y `.agents/skills/`. Antigravity y Codex descubren `.agents/skills/`; Vibe usa `.vibe/skills/`. `AGENTS.md` raíz es compartido. Las copias locales y los perfiles generados se ignoran en Git y se reconstruyen mediante `sync`.

Cada asistente retoma desde [el índice](../index.md), la constitución, los artefactos de la tarea y el último checkpoint en `sdd/sessions/`. Registrar fase, tarea, rama/worktree, evidencia, bloqueos y siguiente acción antes del relevo. No compartir conversaciones privadas. Usar `worktrees` de RSC cuando se necesite aislamiento; el worktree preexistente `fireworm` no se ha modificado ni equipado automáticamente.

El contexto maestro permanece en su ruta original. No hay conexiones ROMERO, Context7 ni otros servidores MCP añadidos por esta instalación. MCP es una capacidad de los asistentes; cada conexión futura necesita autorización.

## Capacidades comprobadas y límites

- **Skills e instrucciones:** descubrimiento real observado en sesiones temporales de Vibe 3.1.2, Codex 0.169.0 y Antigravity CLI `agy` 1.5.10. La aplicación gráfica de Antigravity no fue probada. `doctor` y `capabilities` de Gandalf confirman 33 skills locales por target.
- **Subagentes:** cinco perfiles instalados en Vibe y Codex; Vibe reconoce `developer` como subagente y rechaza seleccionarlo como principal. La invocación efectiva por el modelo no quedó demostrada. Antigravity no recibe perfiles RSC; su revisión por roles es consultiva en esta integración.
- **Hooks/guards Vibe:** hook local `pre_tool`, `strict=true`, adaptador del guard RSC para Bash y `process_start`. Pruebas de paso/bloqueo del adaptador y prueba real del protocolo nativo con un comando inocuo interceptado en temporal. No se ha observado el bloqueo del guard RSC en una sesión real de Gandalf. No es una barrera general de PowerShell, scripts arbitrarios o herramientas MCP. El proyecto debe ser de confianza para que Vibe cargue los hooks locales; no se ha cambiado la confianza global.
- **ASDD/FTD y seguridad:** metodología compartida mediante skills y documentos. No hay paridad de guards deterministas entre los tres asistentes. El workflow `branches` no configura protección remota ni impide por sí solo todo commit en `master`.
- **Memoria:** Codex tiene wiring automático RSC, sujeto a la confianza de hooks del usuario. `doctor` informa `unsupported` para memoria de ciclo de vida en Vibe y Antigravity; usar documentos y checkpoints compartidos. No afirmar que todos tienen memoria automática.
- **Suite del fork:** pruebas focalizadas y validación del manifiesto pasan. La suite completa original terminó con fallos en Windows (rutas, permisos de symlink e invocaciones POSIX, entre otros); no se ha demostrado una suite completa verde ni aislado cada fallo frente a upstream.

## Decisiones de seguridad aplicadas

Después del onboarding se detectaron permisos automáticos de Git y sincronización remota de conocimiento habilitados por defecto en RSC. Se desactivaron mediante `git-permissions off` y `knowledge-sync off`, conforme a las restricciones expresas del propietario. Se conservó el resto de controles. El manifiesto contiene `gitPermissions: false` y el opt-out `knowledge-sync`; Codex ya no tiene el archivo RSC de reglas que permitía commit/push/PR sin preguntar.

Las políticas nativas y globales de cada asistente siguen siendo responsabilidad del usuario; esta instalación no demuestra que cualquier sesión externa pida aprobación. No se ejecutaron operaciones de publicación de Gandalf, instalaciones globales ni cambios de configuración global.

Evidencia local: [verificación JSON](../sdd/verifications/2026-11-17-entorno-rsc.json). La [constitución v1.0.0](../sdd/constitution.md) y REVIEW.md fueron ratificados por el propietario tras revisión el 2026-11-17. Próximo paso: especificar el primer alcance y decidir los parámetros necesarios antes de implementar.
