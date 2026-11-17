---
type: stack
title: Convenciones Go de Gandalf Wizard
description: Reglas de diseño, HTTP REST, manejo de errores, pruebas y controles de calidad en Go 1.27.3.
tags: [stack, go, conventions, testing]
timestamp: 2026-11-17T22:52:00Z
topic: stack
---

# Convenciones Go — Gandalf Wizard

- **Versión y módulo:** Go `1.27.3`, `go.mod` en la raíz del repositorio (`module` independiente del repositorio externo de ROMERO).
- **Layout inicial:** paquetes internos bajo `internal/<dominio>/` (para Foundation 0.1: `internal/romero/`). Interfaces declaradas del lado consumidor (`RomeroReadPort`), constructores que devuelven structs concretos.
- **Contexto y concurrencia:** `context.Context` como primer parámetro en llamadas de I/O o puertos; nunca almacenado en structs.
- **Manejo de errores:** errores centinela para identidad (`ErrAccessDenied`, `ErrServiceInterrupted`, `ErrDetailInterrupted`) y errores tipados para datos (`ScopeError`), envueltos con `fmt.Errorf("...: %w", err)` y verificados con `errors.Is` / `errors.As`. Nunca incluir tokens, secretos ni IDs denegados en mensajes o logs.
- **Cliente REST estándar (`net/http`):** usar `net/http` de la librería estándar con `http.Client` y `Timeout` explícito, cabecera `Authorization: Bearer <token>` sin logueo de credenciales, y límite de lectura de cuerpo (`io.LimitReader`, máx. `4 MiB`) al consumir la API pública de ROMERO.
- **Pruebas:** librería estándar `testing` con casos *table-driven* y subtests (`t.Run`), dobles deterministas (`RomeroFakeAdapter`) y `net/http/httptest` para contratos HTTP.
- **Controles obligatorios (`02-DOCS/wiki/sdd/config.yaml`):**
  - Formato: `gofmt -l .` (debe devolver salida vacía)
  - Análisis estático: `go vet ./...`
  - Suite de pruebas: `go test ./...`
