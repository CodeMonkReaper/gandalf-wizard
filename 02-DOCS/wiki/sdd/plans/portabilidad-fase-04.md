---
type: plan
title: Plan técnico de portabilidad entre Codex y OpenCode
description: Contratos de ejecución y evidencia para la prueba de fase 04.
tags: [sdd, plan]
topic: sdd
slug: portabilidad-fase-04
status: approved
approved: 2026-11-21
date: 2026-11-21
---

# Plan técnico de portabilidad entre Codex y OpenCode

Diseño aprobado en modo manual. [Especificación aprobada](../specs/portabilidad-fase-04.md) · [Constitución v1.2.0](../constitution.md)

## 0. Restricciones comunes

- Usar una única definición versionada de `analisis-propuesta` 1.0.0 y su objetivo `OBJ-PROP-001`.
- Los dos destinos parten del mismo commit, entrada, contexto de prueba y versiones de metodología, skill y reglas.
- Go 1.27.3. Desarrollo con pruebas primero. Comprobar formato, análisis estático y suite Go según la configuración del proyecto.
- Las autorizaciones de servicios externos siguen el procedimiento del proyecto.

## 1. Contexto y alcance

La prueba local de Runtime 0.3 ya define el análisis de propuesta. Extraer esa definición a un recurso compartido por la prueba existente y la prueba de portabilidad. Limitar la comparación a un flujo de texto, dos ejecutores y los criterios P1–P6.

## 2. Arquitectura

```text
Definición + catálogo + entrada → comprobación previa → manifiesto por destino
Copia ROMERO → MemoryPort → motor común → RouterPort → adaptador → CLI real
                               ↓
                    controles, estado y aprobación → informe comparativo
```

Gandalf conserva el grafo, los controles deterministas y el estado recuperable. Cada adaptador interno traduce las tareas de texto y aplica la configuración al ejecutor externo elegido. La ejecución queda fijada a ese destino durante toda la corrida, incluida la reanudación. Esto permite atribuir una diferencia al ejecutor sin mantener dos implementaciones de los criterios. El informe distinguirá los controles ejercidos por Gandalf de las capacidades observadas en cada CLI.

## 3. Interfaces y contratos

- **Preparación.** La misma definición y las capacidades observadas por versión producen dos manifiestos. Cada uno consigna dependencias, límites, configuración efectiva, permisos y evidencia de compatibilidad. El adaptador implementa la comprobación previa de elegibilidad del motor. Si una capacidad obligatoria falta o carece de evidencia, se identifica el paso y se detiene la corrida antes de enviar entradas.
- **Memoria.** La consulta acordada devuelve el contenido de la copia ROMERO fijada. Una consulta distinta o una huella de contenido inesperada impiden usar la copia.
- **Ejecución.** Cada adaptador recibe el texto del paso con sus entradas resueltas y devuelve el texto producido o un error atribuible al destino. Registra versión, resultado y telemetría disponible fuera del contenido que evalúan los controles. Una cancelación se propaga al proceso.
- **Reanudación.** La corrida recupera el estado de su directorio. Antes de continuar comprueba identidad del destino y del manifiesto. Un cambio requiere una corrida nueva. La aprobación y el rechazo se registran mediante el mecanismo humano existente.

### Contrato común de los adaptadores

T002 entrega a T003 y T004 una solicitud común (destino, manifiesto, corrida, paso, intento y texto resuelto). Ambos adaptadores devuelven texto o error clasificado (capacidad ausente, cancelación o fallo del proceso) y emiten por separado eventos para T007. Cada evento identifica corrida, paso, intento, destino, tipo, fuente y referencia a evidencia redactada. Para un acceso, registrar actor (Gandalf o CLI), operación, recurso lógico y resultado observado. La fuente por destino será el evento nativo o una lectura instrumentada del recurso durante el paso. Identificar la cobertura real de cada fuente.

## 4. Datos y recorrido

Preparar dos directorios desde el mismo commit. Identificar por huella el flujo, la entrada, la copia de memoria y el catálogo. Conservar los manifiestos y la diferencia explicada entre configuraciones. Aislar el estado, los artefactos y los permisos efectivos de cada corrida. Correlacionar eventos por corrida, paso e intento. Antes de compartir el informe, aplicar la redacción de secretos también a errores y metadatos. Registrar el consumo como observado, estimado con fuente o sin dato.

## 5. Estrategia de prueba

| Criterio | Prueba y evidencia esperada en cada destino |
| --- | --- |
| P1 | Comparar las huellas de entrada y los manifiestos generados. Registrar la versión del proceso real y comprobar sus permisos efectivos en el directorio de prueba. |
| P2 | Retirar una capacidad requerida en una configuración de prueba. Verificar diagnóstico por paso y cero envíos. Contrastar la matriz del caso positivo con el ejecutor instalado. |
| P3 | Ejecutar el caso válido en ambas CLI. Trazar la entrega del contexto y de los riesgos a la síntesis. Validar riesgos no vacíos, JSON con decision y riesgos, y ausencia de secretos con los mismos controles. |
| P4 | Probar corrección y agotamiento tras dos iteraciones con salidas inválidas controladas. Para evidencia real, usar una entrada adversa común y conservar las respuestas originales. Si el ejecutor nunca produce el fallo, registrar la rama como pendiente de evidencia real. |
| P5 | Detener el proceso en espera de revisión y reanudar. Comparar huellas de artefactos y recuentos de invocaciones antes y después. Aprobar una corrida y rechazar otra. Comprobar los dos estados finales. |
| P6 | Reconstruir cada veredicto desde el informe hasta el evento y el artefacto. Revisar accesos, procedencia del consumo y redacción con un marcador sintético. |

Los dobles de proceso permiten probar errores, cancelación y límites de manera repetible. Para declarar portabilidad, cada celda P1–P6 debe enlazar evidencia suficiente del destino real. El informe distinguirá fallo observado, impedimento y evidencia pendiente.

## 6. Orden y dependencias

1. Fijar el caso común y comprobar que sigue pasando la aceptación de Runtime 0.3.
2. Inventariar las capacidades y restricciones de las versiones instaladas. Con esa evidencia, definir manifiestos y pruebas de contrato por destino.
3. Conectar los adaptadores al punto de entrada de workflow y verificar selección fija, aislamiento y recuperación. Ejecutar la regresión de los comandos existentes.
4. Tras las autorizaciones aplicables, ejecutar la matriz real y revisar el informe contra P1–P6.

## 7. Riesgos y decisiones abiertas

| Prioridad | Señal | Respuesta |
| --- | --- | --- |
| Alta | Un destino requiere permisos más amplios o carece de una capacidad obligatoria. | Registrar versión, paso y capacidad como impedimento. |
| Alta | La CLI hereda herramientas, credenciales o acceso a archivos ajenos a la prueba. | Comprobar los límites efectivos antes de enviar datos. Dos directorios distintos por sí solos no prueban aislamiento. |
| Media | El modelo corrige su respuesta ante la entrada adversa antes de activar el control. | Conservar la evidencia obtenida y distinguir la cobertura local de la observación real de P4. |
| Media | Cambian la versión, el catálogo o la copia ROMERO durante una comparación. | Invalidar el par para esa comparación y repetir con revisiones fijadas. |

El propietario aprobó demostrar el flujo gobernado por Gandalf sobre ambas CLI. La ejecución autónoma del grafo completo por cada CLI sería un alcance distinto.

## Tasks

Todas las tareas están pendientes. Para T001–T007, el nombre de prueba indicado es un compromiso de implementación. Ejecutar `go test -count=1 -v ./... -run '^NombreDePrueba$'` con el nombre de la fila y exigir su PASS explícito. Para cada comportamiento nuevo, conservar también el fallo inicial. Los adaptadores T003 y T004 pueden avanzar por separado una vez fijado su contrato.

| ID | [P] | Tarea | Comprobación de cierre | Depende de | Criterios | Riesgo |
| --- | --- | --- | --- | --- | --- | --- |
| T001 | | Extraer el caso compartido y fijar la memoria | TestPortableReference comprueba huellas iguales, contenido versionado, rechazo de una consulta o huella distinta. TestWorkflowAcceptance pasa con el recurso compartido. | — | P1, P3 | 1 |
| T002 | | Definir manifiestos y comprobar elegibilidad | TestPortablePreflight identifica por versión, destino y paso la capacidad ausente y registra cero envíos. Con capacidades suficientes produce manifiestos comparables y explica cada diferencia de permisos, insumos y límites. Entregar el contrato común del §3 y comprobar sus ejemplos de éxito y error. Contrastar el inventario con observaciones locales por versión. | T001 | P1, P2 | 1 |
| T003 | [P] | Implementar el adaptador Codex | TestCodexTextContract comprueba con un doble de proceso la entrega de entradas, extracción de texto, cancelación, error y separación de telemetría. Consumir y producir el contrato del §3. Cambios confinados al paquete del adaptador Codex. | T002 | P2, P3, P6 | 1 |
| T004 | [P] | Implementar el adaptador OpenCode | TestOpenCodeTextContract comprueba las mismas condiciones del contrato con un doble de proceso OpenCode. Consumir y producir el contrato del §3. Cambios confinados a su propio paquete. | T002 | P2, P3, P6 | 1 |
| T005 | | Integrar ambos destinos con el motor y sus controles | TestPortableWorkflow ejecuta desde la CLI en directorios separados. Comprueba recepción de contexto y riesgos, JSON válido, corrección exitosa y fallo tras dos iteraciones agotadas sin llegar a la espera de aprobación. | T003, T004 | P1–P4 | 1 |
| T006 | | Fijar identidad y recuperar la corrida interrumpida | TestPortableResume reinicia el proceso en espera de aprobación, comprueba huellas y contadores estables, y rechaza un destino o manifiesto alterado. Dos corridas separadas prueban aprobación y rechazo explícitos. | T005 | P5 | 1 |
| T007 | | Generar el informe de evidencia por destino | TestPortableEvidence consume eventos del §3, vincula una lectura conocida con su paso, actor y fuente, y deja P6 sin acreditar si solo recibe permisos. Reconstruye cada resultado desde eventos y artefactos, distingue consumo observado, estimado o ausente y verifica que un secreto sintético no aparece en el informe, errores ni metadatos. | T005, T006 | P6 | 1 |
| T008 | | Verificar la matriz en entornos reales | Con las autorizaciones aplicables, registrar versiones, manifiestos, entradas equivalentes, permisos efectivos y evidencia de los dos procesos reales. En cada destino rastrear la lectura conocida hasta el informe y registrar impedimento si falta la fuente. Completar las doce celdas P1–P6 por destino con resultado y referencia. Registrar toda rama sin evidencia, incluido P4 si corresponde. Declarar portabilidad solo con doce cumplimientos. | T007 | P1–P6 | 0 |

**Revisión y entrega.** Previsión de RSC con política de origin/master y rutas representativas de las pruebas CLI, manifiesto, adaptadores, integración CLI, checkpoint, renderizado e informe documental, respectivamente. El nivel 0 de T008 corresponde al archivo del informe. La ejecución real requiere los permisos previstos en la especificación. Recalcular riesgo sobre los cambios efectivos. Presupuesto por revisión de 400 líneas y 12 archivos. Dividir al superar ese presupuesto. Antes de la matriz real, exigir formato limpio con `gofmt -l .`, análisis con `go vet ./...` y regresión con `go test ./...`.

## Seguimiento en GitHub

[Épica #58](https://github.com/CodeMonkReaper/gandalf-wizard/issues/58). [T001 (#59)](https://github.com/CodeMonkReaper/gandalf-wizard/issues/59) · [T002 (#60)](https://github.com/CodeMonkReaper/gandalf-wizard/issues/60) · [T003 (#61)](https://github.com/CodeMonkReaper/gandalf-wizard/issues/61) · [T004 (#62)](https://github.com/CodeMonkReaper/gandalf-wizard/issues/62) · [T005 (#63)](https://github.com/CodeMonkReaper/gandalf-wizard/issues/63) · [T006 (#64)](https://github.com/CodeMonkReaper/gandalf-wizard/issues/64) · [T007 (#65)](https://github.com/CodeMonkReaper/gandalf-wizard/issues/65) · [T008 (#66)](https://github.com/CodeMonkReaper/gandalf-wizard/issues/66).
