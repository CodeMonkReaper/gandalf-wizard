---
type: spec
title: Consulta de memoria de proyecto en ROMERO
description: Recuperar desde Gandalf recuerdos autorizados y reconocer qué evidencia respalda cada resultado.
tags: [sdd, spec, foundation, romero]
timestamp: 2026-11-17T22:12:00Z
topic: sdd
slug: consulta-memoria-romero
status: approved
branch: chore/rsc-development-environment
---

# Consulta de memoria de proyecto en ROMERO

**Especificación aprobada.** El propietario aprobó la versión aclarada de Foundation 0.1 con sus nueve casos de aceptación. [Decisiones](../decisions.md) · [Constitución v1.0.0](../constitution.md)

## Problema y por qué importa

Al retomar un proyecto importa saber quién respalda una explicación. Cuando alguien pregunta por qué se tomó una decisión, necesita poder distinguir el recuerdo registrado, su origen y el grado de certeza que le atribuye ROMERO.

## Coste de no construirlo

La persona que usa Gandalf tendría que buscar el antecedente por su cuenta y trasladar su referencia y su contexto al asistente. El coste de ese trabajo manual aún no se ha medido.

## La alternativa más barata

Consultar la wiki del proyecto o pedir la referencia a quien documentó la decisión. Si todo el antecedente ya está en la wiki, eso basta. Esta consulta aporta valor cuando la evidencia está en la memoria autorizada de ROMERO.

## Objetivos

Recuperar recuerdos pertinentes al proyecto y mostrar qué dice cada uno, qué referencia lo identifica y qué respaldo declara ROMERO. El alcance procede de los IDs 244, 255 y 260–263 y de la evidencia sugerida para Foundation 0.1. [Contexto maestro histórico, secciones 4.XV y 7](https://github.com/CodeMonkReaper/gandalf-wizard/blob/4aa2986080d343e500c39e5866fdcbb5f7406508/gandalf-wizard-contexto-maestro.md)

## Fuera de alcance

Capturar o modificar recuerdos, calificarlos, resolver contradicciones, cambiar permisos y sincronizar configuraciones entre anfitriones quedan para otras especificaciones. La consulta utiliza el contrato público de ROMERO. La selección de transporte y stack corresponde al plan técnico.

## Usuarios y contexto

Una persona trabaja en un proyecto identificado y quiere recuperar un antecedente desde Gandalf. Su identidad y los permisos que ROMERO reconoce delimitan la consulta. La decisión arquitectónica es un ejemplo de antecedente.

## Qué podrá hacer el usuario

Formular una pregunta sobre el proyecto activo sin conocer el identificador del recuerdo. Si recibe varias coincidencias autorizadas, podrá distinguirlas por su resumen y su referencia.

## Qué resultado obtiene

**Ejemplo aprobado.** En el proyecto ficticio Faro, la persona pregunta «¿Para qué conservamos los informes de prueba?». Para un recuerdo con origen visible, esta es la salida esperada. Los identificadores son alias de los datos de prueba.

| Resumen atribuido a ROMERO | Estado declarado | Referencia del recuerdo | Referencia de la fuente original |
| --- | --- | --- | --- |
| Conservamos los informes para comparar entregas. | Observado (`observed`) | M1 | F1 |

El cliente publicado de ROMERO distingue el resumen y el identificador del recuerdo de la referencia a su fuente primaria. La respuesta compacta de búsqueda carece de esta última. El detalle contempla un identificador de fuente. Una referencia de origen puede ser un identificador, sin que exista un enlace navegable. [Búsqueda](https://github.com/CodeMonkReaper/romero/blob/3f46f9a322b3b44b946889d8474ac835b98bf9fb/pkg/romeroclient/types.go#L207-L241) · [Detalle](https://github.com/CodeMonkReaper/romero/blob/3f46f9a322b3b44b946889d8474ac835b98bf9fb/pkg/romeroclient/types.go#L67-L89)

## Qué debe respetar

La consulta requiere un ámbito de proyecto inequívoco y permisos vigentes. Gandalf presenta el contenido recuperado como evidencia atribuida. Las acciones, herramientas y políticas siguen sujetas a sus autorizaciones. Gandalf solo solicita operaciones de lectura y protege las credenciales. [Contexto maestro histórico, secciones 1 y 8](https://github.com/CodeMonkReaper/gandalf-wizard/blob/4aa2986080d343e500c39e5866fdcbb5f7406508/gandalf-wizard-contexto-maestro.md)

## Comportamiento

Si falta el proyecto activo o hay varios proyectos posibles, Gandalf pide identificar uno antes de consultar ROMERO. Una búsqueda completada puede devolver recuerdos visibles o cero coincidencias. Si ROMERO señala que la búsqueda fue degradada, la respuesta conserva ese aviso, incluso cuando devuelve cero resultados.

Ante una denegación, Gandalf comunica únicamente lo que el servicio permite revelar. Si una interrupción impide comprobar todos los resultados, la salida indica «Consulta incompleta» cuando ninguno quedó confirmado, o «Consulta parcial» junto con las entradas ya comprobadas. Una lectura válida que llega sin referencia de origen permite mostrar el recuerdo e indicar esa carencia.

La consulta normal incluye tanto la decisión vigente como la sustituida cuando ambas son pertinentes y visibles en el proyecto. Cada una se muestra por separado. Gandalf identifica cuál fue sustituida según la información de ROMERO y conserva la calificación de evidencia de cada recuerdo. Una discrepancia o una fecha anterior por sí sola no acredita una sustitución.

## Criterios de aceptación

| Caso | Condición y acción | Resultado observable propuesto |
| --- | --- | --- |
| 1. Recuerdo trazable | Preguntar en Faro por el antecedente del ejemplo, con acceso a M1 y a su referencia de origen. | Aparecen el resumen, el estado, M1 y F1 de la referencia de prueba. |
| 2. Varios recuerdos | La consulta devuelve dos recuerdos visibles con interpretaciones distintas, uno clasificado como hipótesis. | Cada resumen conserva su propia referencia y calificación. Se puede atribuir cada interpretación al recuerdo correspondiente. |
| 3. Origen incompleto | ROMERO permite mostrar el recuerdo, pero su fuente original carece de una referencia visible. | Se conserva la referencia del recuerdo y se indica «Fuente de origen no disponible». |
| 4. Ámbito y permisos | Repetir la misma pregunta en Faro con un recuerdo coincidente en otro proyecto. Repetir con acceso a Faro denegado. | En el primer caso solo aparecen recuerdos de Faro, incluso si la persona puede leer ambos proyectos. En el segundo, la salida preserva la confidencialidad de los recuerdos protegidos, incluidos contenido, identificadores y existencia. |
| 5. Vacío, degradación y fallo | Comparar una búsqueda completada con cero coincidencias, otra completada en modo degradado y otra interrumpida antes de obtener respuesta. | Se distinguen «Sin coincidencias visibles», el aviso de búsqueda degradada con los resultados disponibles y «Consulta incompleta», respectivamente. |
| 6. Lectura sin efectos solicitados | Un recuerdo de prueba incluye una sugerencia de realizar una acción ajena a la consulta. | La sugerencia se presenta como contenido atribuido. Las operaciones solicitadas por Gandalf se limitan a leer los recuerdos autorizados. |
| 7. Proyecto ausente o ambiguo | Hacer la pregunta sin proyecto activo y repetirla con dos posibles proyectos sin seleccionar uno. | En ambos casos Gandalf pide identificar el proyecto y realiza cero consultas de memoria a ROMERO. |
| 8. Interrupción parcial | Obtener M1 con F1 y sufrir una interrupción al comprobar el detalle de otro resultado. | Se muestra M1 con F1 y «Consulta parcial». La salida omite los datos e identificadores de la entrada pendiente. |
| 9. Decisión sustituida | En Faro, la misma pregunta coincide con dos decisiones autorizadas. ROMERO identifica una como vigente y la otra como sustituida. | La consulta normal muestra ambas por separado e identifica la sustituida. Cada entrada conserva su referencia, su calificación de evidencia y la procedencia visible. Se aplican los límites del caso 4 a ambas. |

## Cómo lo demuestra

Comparar las salidas con los nueve casos usando datos ficticios de dos proyectos. Registrar por caso la identidad de prueba, el ámbito, la salida esperada y observada, y las operaciones solicitadas, sin credenciales ni contenido privado. Una simulación permitirá comprobar la presentación de fallos. La aceptación de la integración requerirá además recuperar el recuerdo conocido y comprobar el aislamiento en una instancia de ROMERO autorizada.

## Puntos que siguen abiertos

- **Decisión diferida** — identificar la versión y el entorno autorizados para la prueba de integración, los datos ficticios y los permisos de prueba antes de ejecutarla.

## Clarifications

- 2026-11-17 — El propietario confirmó con «si» el borrador, el ejemplo y la presentación de los recuerdos por separado. Esta confirmación resuelve la suposición inicial reflejada en el caso 2.
- 2026-11-17 — Consultado sobre qué mostrar cuando una decisión reemplaza a otra, el propietario eligió «Ambas, indicando cuál fue sustituida». El caso 9 permite comprobar que la consulta normal conserva el antecedente y distingue su vigencia del grado de certeza.
- 2026-11-17 — El propietario aprobó la especificación aclarada con sus nueve casos de aceptación («apruebo @02-DOCS/wiki/sdd/specs/consulta-memoria-romero.md»).
