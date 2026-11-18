# Revisión de cierre — router-02 — 2026-11-18

**0 hallazgos nuevos: 0 blockers, 0 should-fix, 0 nits (ronda 2 sobre `cef75ff`).** Veredicto: **APPROVE**.

- Rama: `feat/router-02` (25 commits sobre `master`, 0 detrás), estado `cef75ff` (limpio)
- Tier: 1 (`rsc risk --base master --json`: `tier: 1`, `classes: ["default", "docs"]`, `reviewers: 1`, `humanApproval: false`)
- Revisión en contexto frío (`421c7b8e`, `research`): diff completo `git diff master...HEAD` (`42` archivos, `8960` inserciones / `21` borrados) en ronda 1 (`34c5237`) + diff incremental `34c5237..cef75ff` en ronda 2
- Insumos: [spec](../specs/router-02.md) (AC1–AC19), [plan](../plans/router-02.md) (T001–T010), [constitución v1.2.0](../constitution.md), [REVIEW.md](../../../REVIEW.md), [récord de verificación PASS](../verifications/2026-11-18-router-02-closing.md) y [evidencia viva](../verifications/2026-11-18-router-02-live-verification.json)
- Ledger de tareas (`T001`–`T009`): 12 hallazgos únicos (`L11`–`L22`), 0 repetidos, 0 abiertos (12 `fixed`, media 1.00 ronda)

## Pases ejecutados

1. **Fidelidad a la spec** — 19/19 ACs con evidencia observable (`TestRouterAcceptance` + pruebas unitarias + verificación viva); `models list`, `route`, `run` y ampliación de `doctor` cubiertos sin alcance fuera de spec; tras corregir `Probe`, los ejecutores con `authorized: false` tampoco lanzan `--version`. Limpio.
2. **Corrección (fronteras)** — Ctrl+C (`signal.NotifyContext`) corta la ejecución sin fallback; `AttemptTimeout` (default `60s`) detiene el intento y habilita fallback; esfuerzos omitidos cuando no están declarados; deduplicación por `(executor_id, model)`; tras corregir `walkKeys` (`case []any:`), `executorKnownKeys` y `profileKnownKeys` emiten avisos ante claves desconocidas; `Registry.AdapterFor` tolera receptor `nil`. Limpio.
3. **Contratos y datos** — `schema_version: "1"`, salidas tabla y JSON de `models list`, `route`, `run` y `doctor` (`executors` con `omitempty` preservando formato anterior); separación de capas en `mergeRouterPolicy` impide autorización o vínculos de API desde proyecto; códigos `0`–`7` intactos y `8`–`10` añadidos. Limpio.
4. **Frontera de seguridad** — Prohibidos flags de autoaprobación en `antigravity` y `vibe`; ambos ejecutan en modo solo texto verificado en vivo en directorios temporales limpios y filtran variables sensibles y prefijos `ROMERO_`/`VIBE_`; `direct.Adapter` usa `RoundTripper` sin redirects ni reintentos, verifica `https` por sí mismo y no imprime cuerpos ni claves; `IsForbiddenCredentialKey` ampliada rechaza claves específicas de proveedor (p. ej. `mistral_api_key`) preservando `credential_env`; `router.Run` redacta credenciales en salida, explicación y motivos; `TestRepoSecurityGate` y hooks `.git/hooks/pre-commit`/`pre-push` protegen contra subida de secretos y sesiones. Limpio.
5. **Pruebas como evidencia** — `go vet ./...`, `go build ./...` y `go test -count=1 -cover ./...` en verde en los 11 paquetes en `cef75ff`; `TestRouterAcceptance` (`AC1`–`AC19`), `TestRepoSecurityGate`, `TestRouterInterrupt` con `CTRL_BREAK_EVENT` en proceso real, `httptest` y verificación viva de `antigravity`, `vibe` y `api-mistral` + fallback en vivo. Limpio.
6. **Constitución y encaje** — Solo stdlib en `go.mod` (`go 1.27.3`); sin código fuente RSC en Gandalf (`git diff master...HEAD -- .rsc.json .rsc .agents .vibe .codex` vacío salvo el añadido previsto en `.gitignore` y el test); 25 commits con autoría humana sin tráilers de IA (P8); `Q1`–`Q8` registrados en `review-after` en [progress/router-02.md](../progress/router-02.md). Limpio.

## Hallazgos de la revisión de cierre

### Ronda 1 (sobre `34c5237` — veredicto `CHANGES REQUESTED`, 4 hallazgos)

1. **`[should-fix]` `Probe` lanzaba `--version` en ejecutores no autorizados** (`internal/router/antigravity/adapter.go:88`, `internal/router/vibe/adapter.go:88`). **Resuelto en `cef75ff`:** condicionado a `ex.Authorized && p.Auth == router.AuthReady && p.TextOnly == router.TextOnlyEnforced`, con pruebas `probe no autorizado no lanza version` en ambos adaptadores.
2. **`[should-fix]` `walkKeys` pasaba `nil` en arreglos JSON** (`internal/config/load.go:258`), anulando `executorKnownKeys` y `profileKnownKeys`. **Resuelto en `cef75ff`:** pasa `childKnown`, con prueba en `internal/config/router_test.go`.
3. **`[should-fix]` `IsForbiddenCredentialKey` solo coincidía con 6 nombres exactos** (`internal/config/types.go:82-90`), aceptando nombres como `mistral_api_key`. **Resuelto en `cef75ff`:** ampliada por subcadenas de credencial excluyendo `credential_env`, con prueba en `internal/config/router_test.go`.
4. **`[nit]` `Registry.AdapterFor` entraba en pánico con `reg == nil`** (`internal/router/types.go:123-125`). **Resuelto en `cef75ff`:** añadida guardia `if r == nil || r.adapters == nil`.

### Ronda 2 (sobre `cef75ff` — veredicto `APPROVE`, 0 hallazgos nuevos)

Las 4 correcciones quedaron verificadas en contexto frío sin regresiones, con `gofmt -l .`, `go vet ./...` y `go test -count=1 ./...` en verde sobre los 11 paquetes.

## Veredicto

**APPROVE** — la especificación `router-02` cumple el contrato aprobado con evidencia local y viva, sin hallazgos abiertos. Lista para decisión del propietario: abrir PR de `router-02` hacia `master` (push + PR requieren autorización humana: constitución P5) o continuar antes con la segunda spec (`sync-skills-patrones`) de la épica [#15](https://github.com/CodeMonkReaper/gandalf-wizard/issues/15).
