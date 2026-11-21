---
type: verification
title: Escritura confinada y recuperación por archivo en Windows
description: Evidencia de T002 con procesos reales y archivos temporales.
tags: [sdd, verification, sync-skills-patrones]
timestamp: 2026-11-19T16:42:49Z
topic: sdd
---

# Escritura confinada y recuperación por archivo

T002 ensaya las primitivas de archivos previstas para el escritor de Gandalf. Los casos ejecutados pasan y T002 superó la revisión en ronda 3 (`VERDICT: pass`) tras cerrar Q1 y Q2. La suite se ejecutó con Go 1.27.3 y Windows amd64 sobre NTFS local. [Plan técnico](../plans/sync-skills-patrones.md) · [Subissue #30](https://github.com/CodeMonkReaper/gandalf-wizard/issues/30)

## Reproducción

El filtro ejecuta las pruebas de ruta, reemplazo, permisos, exclusión y terminación de procesos.

```powershell
go test -count=1 -v ./internal/syncfs -run '^TestSyncFilesystem$'
```

## Resultados

- Las rutas fuera de la raíz, los enlaces, los componentes de más de 255 unidades UTF-16 y los tipos de archivo no admitidos se rechazaron. La prueba conservó el contenido externo. Una regresión inicial permitió mover directorios intermedios abiertos con acceso solo a atributos. La corrección usa acceso de lectura al directorio y la prueba comprueba que el anclaje impide moverlos durante la operación.
- Una huella obsoleta o un cambio de identidad bloqueó la escritura. Reemplazar con contenido equivalente dejó intacto el archivo. Un destino con permisos distintos del temporal rechazó la actualización. Un archivo con un flujo NTFS adicional conservó el contenido principal y el flujo auxiliar tras el rechazo.
- Un proceso independiente recibió el estado ocupado mientras había un escritor. Tras terminar por la fuerza al titular, otro proceso obtuvo la exclusión. Adquirir y soltar la exclusión dejó intacto el directorio.
- Terminar el proceso antes del reemplazo dejó los bytes previos y eliminó el archivo temporal de ese ensayo. Terminarlo después dejó los bytes previstos. En ambos casos se pudo restablecer el contenido anterior.

## Límites de la evidencia

La prueba cubre terminación de procesos en momentos controlados. El bloqueo coordina escritores de Gandalf con independencia del identificador de operación. La publicación re-verifica la identidad del destino junto al reemplazo, elimina el enlace verificado por handle con semántica POSIX y renombra sin bandera de reemplazo: una entrada externa sustituta entre la comprobación y la publicación produce `ErrConflict` y conserva sus bytes (Q1, `c525306`). Una terminación entre desarmar la eliminación automática y publicar puede dejar el temporal, que ahora sobrevive con el identificador de la operación en el nombre (`.gandalf-sync-<operación>-<id>.tmp`) y se comprueba desde `Change` con un proceso pausado tras armar (Q2, `53de71b`, `c525306`). La privacidad del respaldo y la recuperación completa se vuelven a probar al integrar T005 y T006.
