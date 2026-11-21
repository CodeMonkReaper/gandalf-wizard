---
type: reference
title: Flujo, etiquetas y plantillas de GitHub Issues
description: Convenciones de Issues, taxonomía de etiquetas, plantilla de Pull Request y puertas de acuerdo y de cierre técnico obligatorias.
tags: [harness, github, issues, workflow, ftd, asdd]
timestamp: 2026-11-21T14:55:00Z
topic: harness
---

# Flujo de trabajo con GitHub Issues y Pull Requests

La [constitución v1.2.0](../sdd/constitution.md#L33) exige que toda feature, tarea o cambio sustancial cuente con un Issue abierto desde el inicio y se integre mediante rama y Pull Request vinculado con autorización humana. Esta guía fija cómo operamos GitHub Issues en [CodeMonkReaper/gandalf-wizard](https://github.com/CodeMonkReaper/gandalf-wizard).

Cada Issue atraviesa dos puertas obligatorias. La **Puerta de Acuerdo** (§4) congela qué se hará y cómo se probará antes de la primera línea de código. La **Puerta de Cierre Técnico** (§5) demuestra que se hizo. Sin la primera no se abre el trabajo; sin la segunda no se pasa al Issue siguiente.

---

## 1. Tipos de Issue y cuándo se abren

| Plantilla | Tipo | Cuándo se abre | Convención de título |
|---|---|---|---|
| [01-epic-asdd.yml](../../../.github/ISSUE_TEMPLATE/01-epic-asdd.yml) | Épica ASDD | Al iniciar `specify` para una feature o épica nueva. | `epic(<área>): <descripción> (<slug>)` |
| [02-task-asdd.yml](../../../.github/ISSUE_TEMPLATE/02-task-asdd.yml) | Sub-tarea ASDD | Tras aprobar `plan` + `tasks`, una por cada `T001`..`T00N`. | `<slug> T00X — <descripción>` |
| [03-ftd-change.yml](../../../.github/ISSUE_TEMPLATE/03-ftd-change.yml) | Cambio FTD | Al autorizar un cambio acotado en el carril FTD. | `ftd(<área>): <descripción>` |
| [04-bug-debug.yml](../../../.github/ISSUE_TEMPLATE/04-bug-debug.yml) | Bug / Regresión | Al reproducir un defecto o regresión en `debug`. | `fix(<área>): <descripción>` |

Los Issues en blanco están deshabilitados en [config.yml](../../../.github/ISSUE_TEMPLATE/config.yml). Para trabajos de documentación pura (`type:docs`, rama `docs/<slug>`), abrir el Issue desde la plantilla [03-ftd-change.yml](../../../.github/ISSUE_TEMPLATE/03-ftd-change.yml) ajustando la etiqueta a `type:docs` (o mediante `gh issue create`).

---

## 2. Taxonomía de etiquetas

Cada Issue combina etiquetas de cuatro dimensiones ortogonales:

| Dimensión | Etiquetas | Uso |
|---|---|---|
| **Tipo** | `type:epic`, `type:task`, `type:ftd`, `type:bug`, `type:docs` | Clasificación del Issue. |
| **Carril** | `lane:asdd`, `lane:ftd` | Método de ejecución vigente. |
| **Riesgo** | `tier:0`, `tier:1`, `tier:2` | Clasificación calculada por `rsc risk`. |
| **Estado** | `status:spec`, `status:ready`, `status:in-progress`, `status:blocked`, `status:in-review` | Estado operativo actual del Issue. |

### Transiciones estándar

- **Puerta de Acuerdo (todos los tipos):** ningún Issue pasa a `status:in-progress` sin su Acuerdo publicado y aprobado (§4). Vale también para las épicas antes de `implement`.
- **Épica ASDD:** `status:spec` (en `specify`/`clarify`/`plan`) → Acuerdo (§4) → `status:in-progress` (en `implement`) → `status:in-review` (al abrir el PR de cierre).
- **Sub-tarea / FTD / Bug:** `status:ready` (al crearse) → Acuerdo (§4) → `status:in-progress` (al tomarse) → `status:in-review` (al publicar su cierre técnico / abrir PR).
- **Bloqueos:** si aparece una entrada `[blocking]` en la Decision Queue o un bloqueo externo, añadir `status:blocked` y retirarlo al desbloquearse.

---

## 3. Convención de ramas y Pull Requests

- **Prefijos de rama:** `feat/<slug>` (épicas ASDD), `chore/<slug>` (tareas de harness, mantenimiento o FTD de soporte), `fix/<slug>` (bugs), `docs/<slug>` (documentación pura).
- **Plantilla de PR:** [PULL_REQUEST_TEMPLATE.md](../../../.github/PULL_REQUEST_TEMPLATE.md).
- **Cierre automático:** todo PR que cierre una épica, FTD o bug debe incluir `Closes #<N>` de forma explícita por cada Issue cerrado. La mención en prosa dentro de un párrafo no cierra Issues en GitHub.
- **Autorización humana (P5):** `git push`, `gh pr create` y el merge requieren aprobación explícita del propietario. Nunca incluir coautoría de IA ni pies de generación en commits (P8).

---

## 4. Puerta de Acuerdo del Issue (entrada, obligatoria)

**Regla vinculante (control procedimental):** ningún Issue (`T00X`, FTD o bug) pasa de `status:ready` a `status:in-progress` sin publicar antes en ese Issue su **Acuerdo de Issue**, aprobado por el propietario. Es la puerta simétrica a la de Cierre Técnico (§5): el Acuerdo fija qué se hará y cómo se probará; el Cierre demuestra que se hizo.

*Límite explícito (Constitución P6):* igual que la puerta de cierre, este es un control procedimental/consultivo. Hoy no existe un workflow de GitHub Actions ni un hook remoto que bloquee técnicamente la transición.

### Plantilla obligatoria del comentario de acuerdo

````markdown
## Acuerdo de Issue — #<N> (<slug / tarea>)

### 1. Contrato contra el plan
- **Tarea y plan:** `T00X` de [<plan>](<ruta>), aprobado el <fecha>.
- **Criterios de aceptación que cubre:** `P<n>`…
- **Dependencias:** `#<M>` cerrado | ninguna.

### 2. Alcance congelado
- **Entra:** <paquetes, archivos y contratos que esta tarea toca>.
- **No entra:** <lo que queda para el Issue siguiente, nombrado>.
- **Fija para las tareas siguientes:** <interfaz, tipo, formato o ruta>.

### 3. Comprobación de cierre literal
```bash
<comando exacto que se ejecutará al cerrar>
```
- **Resultado esperado:** <salida o aserción observable>.
- **Si no puede ejecutarse tal cual:** <motivo y qué lo sustituye>.

### 4. Riesgo
- **`rsc risk` previsto:** `tier:<n>` · **Revisión:** <fresca de contexto frío | panel de tres | ninguna>.

### 5. Veredicto
`ACORDADO` (habilita `status:in-progress`) | `ACLARAR` (<pregunta concreta al propietario>) | `FUERA DE ALCANCE` (se abre Issue propio).
````

### Reglas de vigencia

- El Acuerdo se escribe **una sola vez**, antes de la primera línea de código. No hace falta reunión: el veredicto del propietario en el comentario es la autorización.
- Si el alcance o el comando cambian durante el trabajo, **un comentario nuevo sustituye** al anterior indicando qué cambió y por qué. El silencio no es un cambio.
- `ACLARAR` deja el Issue en `status:blocked`, nunca en `status:in-progress`.
- El Acuerdo **no declara autorizaciones humanas**: las rigen §3 y la Constitución P5. Una autorización poco habitual (servicio externo, credencial) se anota en «Alcance» como nota.
- La selección del modelo ejecutor de Vibe tampoco forma parte del Acuerdo: se consigna en el campo «Ejecutor / modelo Vibe previsto» del Issue, conforme a las instrucciones del proyecto.

---

## 5. Puerta de Cierre Técnico (obligatoria antes de pasar al siguiente Issue)

**Regla vinculante (control procedimental):** ningún Issue (`T00X`, FTD o bug) se da por terminado ni habilita el inicio del siguiente Issue sin publicar antes en ese Issue su **Informe de Cierre Técnico**.

Este informe garantiza trazabilidad de qué cambió exactamente, qué controles lo demostraron y qué errores ocurrieron durante el desarrollo junto con su solución real. *Límite explícito (Constitución P6):* esta puerta es un control procedimental/consultivo regido por la disciplina del coordinador y la revisión humana del PR; actualmente no existe un workflow de GitHub Actions ni un hook remoto que bloquee técnicamente la transición de Issues.

### Plantilla obligatoria del comentario de cierre

```markdown
## Cierre técnico — #<N> (<slug / tarea>)

### 1. Revisión técnica de lo implementado
- **Commits:** `<sha>`
- **Archivos tocados/creados:** `<lista con responsabilidad breve>`
- **Contratos y decisiones aplicadas:** `<qué fijó o cambió esta tarea>`
- **Casos borde y riesgos residuales:** `<qué cubre y qué queda en review-after o para el siguiente Issue>`

### 2. Verificación y revisión ejecutada
- **Comandos y `done-check`:** `<comandos reales y resultado observado; nunca omitir si algo no corrió>`
- **Riesgo (`rsc risk`) y review:** `<tier, revisor fresco/panel, hallazgos abiertos/resueltos>`
- **Delegación Vibe (si aplicó):** `<modelo solicitado/efectivo, run ID, qué hizo Vibe y qué verificó el coordinador>`

### 3. Errores del desarrollo y solución aplicada
| Error / bloqueo observado | Causa raíz verificada | Solución aplicada | Prevención / nota |
|---|---|---|---|
| `<síntoma, fallo de test, entorno o herramienta; o "Ninguno">` | `<por qué ocurrió>` | `<cambio exacto y prueba que lo validó>` | `<test de regresión, regla o límite documentado>` |

### 4. Transición
- **Veredicto:** `LISTO PARA SIGUIENTE ISSUE` (siguiente: `#<M>`) | `BLOQUEADO` (motivo en Decision Queue)
```

---

## 6. Reglas operativas en Windows / PowerShell

- **Preservar Markdown en `gh`:** usar siempre here-strings con comillas simples (`@'...'@`) enviados por pipe a `gh issue create --body-file -` o `gh issue comment <N> --body-file -`. Nunca usar `@"..."@` porque PowerShell interpreta o corrompe backticks y variables (`$`).
- **Higiene de secretos:** ningún Issue, comentario, commit o PR puede contener tokens, rutas privadas de credenciales ni salidas sin redactar (`TestRepoSecurityGate`).
- **Hook `pre-push` en worktrees:** ejecutar `git push` desde la raíz del repositorio principal cuando se use un worktree enlazado para evitar interferencias de `GIT_DIR` en `TestRepoSecurityGate` (lección registrada en [2026-11-20-retro-sync-skills-patrones.md](../sdd/sessions/2026-11-20-retro-sync-skills-patrones.md#L21)).
- **Selección por modelo en Vibe:** en Issues, anuncios y cierres técnicos se indica únicamente el **modelo** elegido desde `rsc-vibe models` (nunca perfiles internos como `light`, `balanced`, `debug` o `heavy`); el coordinador traduce internamente el permiso del bridge al despachar con `--model <id>`.
