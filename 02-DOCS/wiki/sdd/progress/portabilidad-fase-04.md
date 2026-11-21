---
type: progress
title: Avance de la prueba de portabilidad entre Codex y OpenCode
timestamp: 2026-11-21T16:05:00Z
topic: sdd
slug: portabilidad-fase-04
---

# Avance de la prueba de portabilidad

[Épica #58](https://github.com/CodeMonkReaper/gandalf-wizard/issues/58) · [Especificación](../specs/portabilidad-fase-04.md) · [Plan y tareas](../plans/portabilidad-fase-04.md)

## T001 · 2026-11-21 — Extraer el caso compartido y fijar la memoria

- **Estado:** completo, revisión aprobada. `rsc risk`: `tier:1`, 1 revisor, sin aprobación humana (base `origin/master`).
- **Rama:** `feat/portabilidad-fase-04-t001`, base `5db04b3`.
- **Acuerdo:** publicado y aprobado en [#59](https://github.com/CodeMonkReaper/gandalf-wizard/issues/59#issuecomment-7064845549).
- **Cambios:** paquete nuevo `internal/portability` con `reference.go` (definición, catálogo, entrada, destinos), `memory.go` (copia fija con revisión y puerto que rechaza otra consulta, otra huella o una copia sin revisión) y `fingerprint.go` (huellas del caso, con la misma identidad SHA-256 que persiste el motor); `reference_test.go` con `TestPortableReference`. `internal/cli/workflow_acceptance_test.go` consume el caso: escribe la definición desde `portability.DefinitionJSON()` y no cambia ninguna aserción.
- **Rojo inicial:** `go test ./internal/portability` falló con `no non-test Go files ... [build failed]`.
- **Verde:** `TestPortableReference` PASS (9 subtests) y `TestWorkflowAcceptance` PASS (16 subtests, incluido el proceso real). `go vet ./...` sin salida y `go test ./...` con 20 paquetes `ok`.
- **Fijación:** seis huellas del caso como constantes en la prueba (definición, catálogo, entrada, memoria, destinos y caso). La de la definición coincide con el `definition_sha256` que persiste el motor para los mismos bytes. Un cambio del caso sin subir la revisión rompe la prueba.
- **Ronda 1 de revisión (contexto frío):** REQUEST CHANGES, 6 hallazgos (H1 y H2 MEDIA; H3–H6 BAJA). La extracción se verificó fiel campo por campo y las aserciones no se debilitaron; los dos MEDIA afectaban a la capacidad probatoria de las huellas. Corregidos los seis.
- **Ronda 2:** APPROVE CON RESERVAS. El revisor reejecutó sus cinco ataques y los cinco fallan ahora. Quedaron tres condiciones menores, ya aplicadas: huellas y recuentos exactos en este informe de verificación (N1), identidad de la aceptación anclada a `DefinitionJSON()` (R1) y destinos dentro de la huella del caso (N2).
- **Evidencia:** [verificación de T001](../verifications/portabilidad-fase-04-t001.md).
- **Deuda registrada:** la regla `no_secrets` del gate de síntesis no tiene prueba de bloqueo, ni antes ni después de T001. Se atiende en la matriz real (T008) o en un Issue propio; fuera del alcance de T001. `NewFixedMemory` valida coherencia interna de la copia, no pertenencia al caso; hoy el único consumidor pasa `FixedMemory()`.

## Siguiente

T002 ([#60](https://github.com/CodeMonkReaper/gandalf-wizard/issues/60)): manifiestos por destino y comprobación previa de elegibilidad. Depende de T001 y su nombre de prueba comprometido es `TestPortablePreflight`.
