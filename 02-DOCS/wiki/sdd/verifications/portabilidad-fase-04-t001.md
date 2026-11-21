---
type: verification
title: Verificación de T001 — caso compartido y memoria fijada (portabilidad fase 04)
description: Comandos ejecutados, resultados observados y rondas de revisión de T001.
tags: [sdd, verification, portabilidad-fase-04, t001]
timestamp: 2026-11-21T16:05:00Z
topic: sdd
slug: portabilidad-fase-04
verdict: PASS
---

# Verificación de T001 — `portabilidad-fase-04`

**Fecha:** 2026-11-21
**Rama:** `feat/portabilidad-fase-04-t001` (base `5db04b3`)
**Issue:** [#59](https://github.com/CodeMonkReaper/gandalf-wizard/issues/59) · **Épica:** [#58](https://github.com/CodeMonkReaper/gandalf-wizard/issues/58)
**Criterios cubiertos:** P1, P3
**Veredicto:** **PASS**
**Entorno:** Windows `amd64` · Go `1.27.3`

## 1. Comprobación de cierre del Acuerdo

| Comando | Resultado observado |
| --- | --- |
| `go test -count=1 -v ./... -run '^TestPortableReference$'` | **PASS** — 9 subtests |
| `go test -count=1 -v ./... -run '^TestWorkflowAcceptance$'` | **PASS** — 16 subtests, incluidos `AC-10` y `process_interruption, resume, contention and cooperative cancel` (proceso real) |
| `gofmt -l .` | Sin salida sobre copias normalizadas a LF. En local marca todos los `.go` por `core.autocrlf=true`; el mismo efecto aparece en archivos intactos como `internal/router/catalog.go`, así que no es un hallazgo del cambio |
| `go vet ./...` | Sin salida (exit 0) |
| `go test ./...` | **20 paquetes `ok`**, ningún fallo |

Los nueve subtests de `TestPortableReference`: huellas iguales y fijadas; la definición escrita es la que el motor identifica; un cambio del caso cambia la huella; contenido versionado; el motor rechaza una definición con campos desconocidos; la copia fija responde la consulta acordada; rechazo de una consulta distinta; rechazo de una huella distinta; rechazo de una copia sin revisión.

## 2. Fallo inicial conservado (rojo)

Antes de escribir el paquete, la prueba del caso compartido no compilaba:

```text
github.com/CodeMonkReaper/gandalf-wizard/internal/portability: no non-test Go files in .../internal/portability
FAIL    github.com/CodeMonkReaper/gandalf-wizard/internal/portability [build failed]
```

## 3. Huellas fijadas del caso (revisión `1`)

Valores completos, copiados de las constantes de `internal/portability/reference_test.go`. Para reproducirlos, el propio subtest de la prueba los imprime cuando no coinciden; no se abrevian aquí para que la evidencia no dependa de una errata.

| Parte | Huella SHA-256 |
| --- | --- |
| Definición | `598e197b52ffa23f9b73fe699f404d1af6f9deae639ede82af0266c5e895dc78` |
| Catálogo | `89ffddd17ffc5476ba997ce69e608505413c655d1a4197ea84c8892b6be2bb68` |
| Entrada | `ff8b65d6786ded24db7dce80dbbb5657b7e9bb1d1b707c5ef567972d246a10b2` |
| Memoria | `1664844657a974f07e1e5a90e3622059f19f4add4f896e17f5058023046bb32e` |
| Destinos | `bd5909e9fb79f5ac7c92c8b9b3a532f71fa37bae4d047fb25ee433cc3cbc196c` |
| Caso | `b5a37cc7fc7aaeee7b249e9201970a8358c17da75ef3d41401411cd5ee719149` |

La huella de la definición es **la misma que persiste el motor**: `schema.LoadAndValidate` sobre el archivo canónico devuelve `definition_sha256` idéntico a la huella fijada. La prueba lo comprueba sobre el archivo escrito, no por lectura del código. La prueba de aceptación escribe el caso desde `portability.DefinitionJSON()`, así que ejecuta exactamente los bytes que la huella identifica.

## 4. Qué queda fijado y dónde

| Parte | Definición |
| --- | --- |
| Definición | `analisis-propuesta@1.0.0`, objetivo `OBJ-PROP-001@1.0.0`, concurrencia `ReferenceConcurrency` |
| Catálogo | Selección declarada de los pasos `router_task` (`complexity`, `risk`) |
| Entrada | Texto resuelto de la propuesta |
| Memoria | Copia `decisiones previas propuesta` de ROMERO `rev-3f46f9a`, con puerto que rechaza otra consulta, otra huella o una copia sin revisión |
| Destinos | `codex` y `opencode` |

## 5. Rondas de revisión

Nivel `tier:1` según `rsc risk` (1 revisor fresco de contexto, sin aprobación humana; base `origin/master`).

| Ronda | Veredicto | Hallazgos | Acción |
| --- | --- | --- | --- |
| 1 | REQUEST CHANGES | 6: H1 y H2 MEDIA, H3–H6 BAJA | Corregidos los seis (ver §6) |
| 2 | APPROVE CON RESERVAS | 5 cerrados, H4 parcial, 2 nuevos BAJA (N1, N2) | Corregidos N1, N2 y R1 (resto de H4) |

En la ronda 2 el revisor reejecutó sus cinco ataques de la ronda 1 sobre copias en `/tmp` y **los cinco fallan ahora**: memoria alterada, selección del catálogo alterada, catálogo invertido, huellas constantes y concurrencia cambiada. También comprobó por ejecución que la huella fijada coincide con la identidad del motor, tanto para el archivo del caso como para el que escribe la aceptación.

## 6. Hallazgos y corrección aplicada

| ID | Severidad | Hallazgo | Corrección |
| --- | --- | --- | --- |
| H1 | MEDIA | Las "huellas iguales" se comparaban consigo mismas; una implementación con constantes pasaba los cinco subtests | Huellas esperadas fijadas como constantes y subtest de bloqueo: un cambio del caso cambia la huella. Repetido el ataque (huella constante): `--- FAIL` |
| H2 | MEDIA | `DefinitionFingerprint` normalizaba con `json.Unmarshal`, descartaba campos desconocidos y colisionaba con una definición que el motor rechaza | Huella = SHA-256 de los bytes, sin normalizar: la misma identidad que persiste el motor. Un archivo con campos desconocidos lo rechaza `schema.LoadAndValidate`, y la prueba lo demuestra |
| H3 | BAJA | La revisión de la copia ROMERO no entraba en ninguna huella ni se validaba | `MemoryFingerprint` cubre consulta, texto y revisión con codificación de longitud; `NewFixedMemory` rechaza una copia sin revisión (`ErrUnversionedCopy`) |
| H4 | BAJA | Un `2` literal acoplaba la prueba a `ReferenceConcurrency` | La prueba usa `portability.ReferenceConcurrency` y comprueba el límite declarado |
| H5 | BAJA | El enlace de la especificación apuntaba al archivo antiguo; sin evidencia registrada | Enlace actualizado a `internal/portability/reference.go`; este informe y el [registro de avance](../progress/portabilidad-fase-04.md) enlazados desde el índice. El plan aprobado no se modifica: su tabla de tareas no tiene columna de estado, igual que en las épicas anteriores |
| H6 | BAJA | Archivo sin seguimiento ajeno a T001 en el árbol | Excluido del commit de T001; no se toca (`02-DOCS/wiki/ftd/codex-hooks-context-limit.md`) |
| N1 | BAJA | Este informe registraba dos huellas abreviadas erróneas y contaba 17 subtests donde hay 16 | Huellas completas en §3, recuentos verificados con `grep -cE '^    --- PASS'` |
| N2 | BAJA | La huella del caso no cubría los destinos | `Destinations` entra en la huella del caso; un tercer destino cambia el valor fijado |
| R1 | BAJA | La aceptación reconstruía la definición y su coincidencia con el caso dependía de `ReferenceConcurrency == 2` | La aceptación escribe el caso desde `portability.DefinitionJSON()`: un solo escritor canónico. Las definiciones alteradas de `AC-03`, `AC-06` y `AC-09` siguen mutando su copia |

### Deuda observada, no imputable a T001

Quitar la regla `no_secrets` del gate de síntesis deja verdes las dos suites, también en `origin/master`. P3 no tiene hoy prueba de bloqueo por secretos. Se registra para la matriz real (T008), no se corrige aquí: queda fuera del alcance congelado de T001.

**Corrección posterior:** `NewFixedMemory` compara además con la huella de la copia fijada. La prueba de regresión modifica por separado el texto y la revisión, recalcula la huella y exige el rechazo. Antes de la corrección ambos casos se aceptaban.
