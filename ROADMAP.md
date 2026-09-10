# Roadmap — Vesper

Corto y real: solo cosas con un plan concreto de implementación.
(Marco de trabajo: proyecto de portfolio + TFG, tiempo limitado.)

## v1.1 — Consolidación

- [ ] Tests para `internal/api` (rutas auth/campaigns/agents con httptest)
- [ ] Dashboard: tests de componentes con Vitest + sustituir `/ws/terminal` del store por el endpoint real ya implementado
- [ ] `golangci-lint` en CI (ahora solo `go vet` + `gofmt`)
- [ ] Cobertura visible: badge con `go test -cover` agregado

## v1.2 — Laboratorio

- [ ] `make lab-up` verificado en CI (job con Docker: compose up + smoke + down)
- [ ] Escenario E2E grabable: campaña → agente → handler de bridge → informe
- [ ] `docs/DEPLOYMENT.md` regenerado con capturas del lab real

## v1.3 — Puente

- [ ] Migrar handlers de `bridge.py` (monolito ~800 líneas) al contrato `register_routes()`
- [ ] gRPC como transporte primario (el JSON-RPC TCP queda como fallback)
- [ ] Timeout y límite de memoria por invocación de handler

## ideas estacionadas (sin compromiso)

- Modo "defensa" que reproduce los handlers desde el punto de vista azul
  (detección + reglas Sigma equivalentes) — encaja con `internal/defense`
- Export de decisiones del orquestador a MITRE ATT&CK Navigator
