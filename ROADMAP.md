# Roadmap — Vesper

Corto y real: solo cosas con un plan concreto de implementación.
(Horizonte: un trimestre. Filosofía: poco y sólido antes que mucho y fingido.
Doble objetivo: portfolio visible + evidencias exportables para auditorías de laboratorio.)

## Fase 0 — Publicar v1.0.0 (inmediato)

- [ ] Push del commit de remodelación + rename del repo a `Vesper`
- [ ] CI verde sobre el nuevo commit
- [ ] GitHub Release v1.0.0: binarios linux amd64/arm64 + notas + zip
- [ ] Topics y social preview del repo (`red-team`, `c2`, `golang`, `security`)

## v1.1 — Calidad (semanas 1–4)

Área: calidad. Objetivo: que "está testeado" sea verificable por un extraño
sin leer una línea de código.

- [ ] Tests de `internal/api` con httptest: rutas auth/campaigns/agents (JWT incluido)
- [ ] `go test -race` como job propio en CI
- [ ] `golangci-lint` en CI (config mínima: errcheck, govet, staticcheck, gosimple)
- [ ] Badge de cobertura: `go test -cover` + `pytest --cov` agregados en un job → badge en README
- [ ] Dashboard: tests de componentes con Vitest + conectar el store al `/ws/terminal` real
      (el endpoint ya existe; el store sigue en modo mock)
- [ ] Release v1.1.0 + GIF corto: terminal xterm.js ejecutando la consola real

## v1.2 — Lab reproducible + recon real (semanas 5–8)

Áreas: lab Docker + funcionalidad. Objetivo: que cualquiera levante el lab
y que la reconozca sea tráfico de red de verdad, no una tabla hardcodeada.

- [ ] `lab/docker-compose.yml` con servicios vulnerables reales y healthchecks:
      web app (DVWA o Juice Shop), Redis sin auth, MySQL, SMB (Samba)
- [ ] Job de CI con Docker: compose up → smoke de conectividad → down
- [ ] Quickstart "5 minutos": clone → `make setup` → `make lab-up` → sesión demo scripted
- [ ] `internal/recon`: scanner TCP nativo en Go (conectividad, banners, fingerprint
      básico de servicio) expuesto en el console como `recon scan <target>`,
      testeable contra el lab
- [ ] Bridge: migrar `bridge.py` (~800 líneas) al contrato `register_routes()` +
      timeout y límite de memoria por invocación de handler
- [ ] E2E grabable: campaña → recon real contra el lab → handler de bridge → informe
- [ ] Release v1.2.0 + GIF nuevo: recon real contra el lab desde el console

## v2.0 — Auditoría (semanas 9–12)

Áreas: dashboard + evidencias. Objetivo: que una campaña produzca un
entregable de auditoría, no solo logs en la terminal.

- [ ] Reporte HTML de campaña: timeline, hosts, hallazgos, decisiones del orquestador
      y mapeo ATT&CK — generado desde `appstate`, escrito en el sandbox
- [ ] Export MITRE ATT&CK Navigator (JSON layer) de las decisiones de la campaña
- [ ] Dashboard: `CampaignTimeline.vue` y `VulnerabilityHeatmap.vue` conectados a
      datos reales del API (no fixtures) + export JSON/CSV de evidencias
- [ ] `docs/DEPLOYMENT.md` regenerado con capturas del lab real
- [ ] Release v2.0.0 + demo GIF final: campaña completa contra el lab → reporte HTML

## Criterios de corte (lo que NO entra este trimestre)

- LLM/ML (Ollama, scoring con modelos): fuera hasta que lo anterior esté sólido
- Modo defensa azul (detección + reglas Sigma equivalentes sobre `internal/defense`):
  idea buena, sin hueco este trimestre
- gRPC como transporte primario del bridge (el JSON-RPC TCP queda como fallback):
  se evalúa al cerrar v1.2 con métricas reales de latencia
- Cualquier feature que no se pueda demostrar en un GIF o en un reporte
