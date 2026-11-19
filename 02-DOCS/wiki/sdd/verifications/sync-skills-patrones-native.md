---
type: verification
title: Descubrimiento nativo de skills y reglas en Codex y Vibe
description: Evidencia T001 de las rutas de proyecto y su precedencia observable.
tags: [sdd, verification, sync-skills-patrones]
topic: sdd
---

# Descubrimiento nativo de skills y reglas

**T001 · PASS el 19 de noviembre de 2026.** Codex CLI `0.169.0` y Vibe CLI `3.1.2` descubrieron la skill de proyecto bajo `.agents/skills/<id>/SKILL.md` y recibieron la regla de `AGENTS.md` en Windows amd64, build `10.0.26300.0`, con Go `1.27.3`. La prueba cubre la viabilidad de AC-03 y la precedencia pertinente para AC-05. [Plan técnico](../plans/sync-skills-patrones.md) · [Especificación](../specs/sync-skills-patrones.md)

## Perfil observado

| Destino | Confianza del proyecto durante el ensayo | Skill con dos candidatos del mismo nombre | Regla con los dos archivos presentes |
| --- | --- | --- | --- |
| Codex CLI `0.169.0` | `trust_level = "trusted"` para el proyecto en un `CODEX_HOME` temporal | Descubrió la de `.agents/skills/`. La de `.vibe/skills/` estuvo ausente del contexto | `AGENTS.override.md` sustituyó a `AGENTS.md` |
| Vibe CLI `3.1.2` | `--trust` para la invocación desde el proyecto y `VIBE_HOME` temporal | Seleccionó la de `.vibe/skills/` sobre la de `.agents/skills/` | Seleccionó `AGENTS.md`. `AGENTS.override.md` estuvo ausente del contexto |

Sin los candidatos que cambian la precedencia, ambos recibieron la descripción de la skill y la regla completa de los archivos previstos. En el proyecto vacío, ambos omitieron esos dos datos. La prueba abre cada CLI desde la raíz del proyecto temporal. [Descubrimiento de Codex](https://learn.chatgpt.com/docs/build-skills#where-codex-loads-local-skills) · [Instrucciones de Codex](https://learn.chatgpt.com/docs/agent-configuration/agents-md) · [Descubrimiento de Vibe](https://docs.mistral.ai/vibe/code/cli/skills#skill-locations)

## Reproducción y huellas

En la raíz de Gandalf, habilitar el ensayo de los ejecutables instalados. El comando ejecuta tres casos para cada destino.

```powershell
$env:GANDALF_SYNC_NATIVE_TEST = '1'
go test -count=1 -v ./internal/syncnative -run '^TestSyncNativeProbe$'
```

La prueba comprueba la versión antes de iniciar cada sesión. Resuelve los ejecutables en `PATH`. `GANDALF_SYNC_CODEX_BIN` y `GANDALF_SYNC_VIBE_BIN` permiten indicar la ruta del ejecutable nativo instalado cuando sea necesario.

| Archivo sintético | SHA-256 |
| --- | --- |
| `.agents/skills/gandalf-native-conformance/SKILL.md` | `4dd48f43743e14a51e67199eb16dfc2365b4739710234dc927c27df21175d87e` |
| `.agents/skills/gandalf-native-conformance/references/sample.md` | `969d7290a0fd3db4c845d5c81d37d57316737df6d0233debe5a17942e62fedf3` |
| `AGENTS.md` | `295ff7a5ba8bfe12de49b2cbc59aef2db24c1ba314043df0abf6ddd67f0fcb7d` |

La ejecución inicial falló en las aserciones de descubrimiento y precedencia con la captura vacía. La ejecución con los binarios instalados pasó los seis subcasos. Cada sesión entregó una petición al proveedor local y recibió la respuesta sintética esperada.

## Aislamiento y alcance de la evidencia

Cada sesión usa archivos sintéticos, un perfil temporal y un entorno definido por lista de variables. El proveedor escucha únicamente en la interfaz local y rechaza peticiones con cabeceras de autenticación. Cada CLI recibe un proxy HTTP(S) local que devuelve error y carece de reenvío. En las primeras corridas, el proxy registró tres denegaciones por sesión de Vibe. La revisión del código instalado mostró que Vibe 3.1.2 podía consultar el almacén de credenciales del sistema durante el inicio, aun con un perfil temporal. La prueba fija VIBE_TEST_DISABLE_KEYRING=1 antes de lanzar Vibe. Tras añadir el bloqueo, pasaron los seis casos y el proxy registró cero intentos auxiliares en ambos destinos. La ejecución inicial carece de evidencia sobre acceso efectivo al almacén. La evidencia conserva los contadores y las huellas. Las peticiones y respuestas quedan dentro de la ejecución de la prueba. El aislamiento de red se basa en la configuración de los procesos y el proxy, sin una verificación de firewall del sistema operativo.

La skill se observa por su descripción inequívoca en el contexto del modelo. La comprobación del recurso declarativo registra su huella en disco. T008 verificará los dos destinos después de aplicar la selección mediante Gandalf, junto con la repetición y restauración.

## Aceptación de extremo a extremo (T008 · 2026-11-19)

**T008 · PASS el 19 de noviembre de 2026.** Se ejecutaron `TestSyncAcceptance` (`internal/cli`) y `TestSyncNativeAcceptance` (`internal/syncnative`, con `GANDALF_SYNC_NATIVE_TEST=1`) aplicando el ejemplo de referencia (`revision-cambios@1.0.0`, `evidencia-de-pruebas@1.0.0` y `revision-por-pares@1.0.0`) mediante la CLI de Gandalf sobre dos copias temporales independientes (`copyA` y `copyB`).

- **Convergencia (`copyA == copyB`):**
  - `.agents/skills/revision-cambios/SKILL.md`: `84e9ae63a8dc96fe8f7974bee44c6b2d4f77aa4eb5005b3242064bf952701cc9`
  - `.agents/skills/revision-cambios/references/checklist.md`: `db415feb86c5566a507f0620074d98b7e2f05ff9532e984a6cc00b13e2914edf`
  - `AGENTS.md`: `7f5b38c4fb8d33c0e7fe36e8d4174fd27b08e9864f130a0e3740731ca42a4f9a`
- **Descubrimiento nativo tras `sync apply` y ausencia tras `sync restore`:**
  - **Codex CLI `0.169.0`** (`windows/amd64`): **PASS** tras aplicar y tras restaurar (`proxy_denials=0`).
  - **Vibe CLI `3.1.3`** (`windows/amd64`): **PASS** tras aplicar y tras restaurar (`proxy_denials=0`).
- **Privacidad y recuperación:** `TestSyncAcceptance` verificó además fuente mutada tras registro (`AC-01`), vista previa de solo lectura (`AC-02`), repetición `noop` (`AC-04`), restauración de alta y actualización (`AC-06`), conflicto por edición posterior + reconciliación (`AC-07`) y ausencia del marcador de secreto en `stdout`/`stderr` (`AC-08`).
