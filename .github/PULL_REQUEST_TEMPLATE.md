## Resumen

<!-- Describe brevemente el objetivo del cambio y qué entrega este PR. -->

## Issues vinculados

<!-- Usa Closes #<N> de forma explícita por cada Issue que este PR cierra al fusionarse (la mención en prosa no cierra Issues en GitHub). -->
- Closes #

## Puerta de cierre técnico

- [ ] Cada Issue vinculado tiene publicado su **Informe de Cierre Técnico** (revisión técnica de lo implementado + tabla de errores del desarrollo y soluciones aplicadas).
- [ ] Documentos de progreso, decisiones y evidencia en `02-DOCS/wiki/` actualizados.

## Testing y evidencia ejecutada

<!-- Registra únicamente comandos realmente ejecutados después de la última edición y sus resultados reales (nunca marques como pasado lo que no corrió). -->
- `gofmt -l .`:
- `go vet ./...`:
- `go test -count=1 ./...`:
- Pruebas específicas / aceptación:

## Riesgo y `review-after`

- `rsc risk`: `tier 0` / `tier 1` / `tier 2`
- Revisión ejecutada (fresca / panel de 3 revisores):
- Entradas `review-after` (si aplica):

## Cómo revertir

<!-- Indica cómo revertir de forma segura si aparece una regresión tras el merge. -->
- `git revert <merge-sha>`
