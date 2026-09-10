# Vesper — Manual Operacional

> Plataforma semi-autónoma de operaciones red team · Go 1.26 + Python 3.11 + Vue 3
> Postura solo-laboratorio por defecto · resultados nunca simulados

> Este manual describe solo lo que el código hace de verdad. Cada comando de
> esta página fue ejecutado y verificado sobre el repositorio real. Las
> capacidades que aún no existen se marcan como tales en lugar de documentarse
> con salidas inventadas.

---

## Índice

1. [Requisitos e Instalación](#1-requisitos-e-instalación)
2. [Postura de Seguridad](#2-postura-de-seguridad)
3. [Consola Interactiva](#3-consola-interactiva)
4. [Modo CLI](#4-modo-cli)
5. [Campañas y Orquestador](#5-campañas-y-orquestador)
6. [Dashboard](#6-dashboard)
7. [Módulos del Bridge](#7-módulos-del-bridge)
8. [Laboratorio Docker](#8-laboratorio-docker)
9. [Generación de Payloads](#9-generación-de-payloads)
10. [Solución de Problemas](#10-solución-de-problemas)
11. [Estructura del Proyecto](#11-estructura-del-proyecto)

---

## 1. Requisitos e Instalación

| Dependencia | Versión | Para qué |
|-------------|---------|----------|
| Go | 1.26+ | Compilar `vesper` y el implant |
| Python | 3.11+ | Bridge de post-explotación |
| grpcio | 1.64+ | Runtime del bridge (`pip install -r requirements.txt`) |
| Node.js | 22+ | Solo para compilar el dashboard web |
| Docker + Compose | v2 | Solo para el laboratorio |

```bash
# Instalación
./install.sh            # o manual:
go build -o dist/vesper ./cmd/vesper
go build -o dist/implant ./cmd/implant
pip install -r requirements.txt

# Verificación
./dist/vesper version   # banner + BUILD INFO
./dist/vesper help
```

## 2. Postura de Seguridad

Vesper arranca **siempre** en modo solo-laboratorio. La postura es visible en
todo momento (línea `POSTURE` del banner de la consola):

```
  POSTURE  ● LAB-ONLY — sandboxed file ops, loopback bind only
```

| Mecanismo | Activación | Efecto |
|-----------|------------|--------|
| Sandbox de ficheros | Automático (LAB-ONLY) | `VESPER_LAB_ONLY=1`: toda E/S del bridge queda dentro de `lab_root/` (`resolve_in_lab()` rechaza rutas externas) |
| Bind protegido | Automático (LAB-ONLY) | `server.host` no-loopback se reescribe a `127.0.0.1` |
| Autorización | `--yes-i-am-authorized` o `VESPER_AUTHORIZED=1` | Habilita operaciones reales no-lab (banner verde) |
| Kill switch | `VESPER_KILLSWITCH=1` (antes de arrancar) | El binario rechaza arrancar (exit 130) |
| Kill switch runtime | `kill_switch EMERGENCY_STOP` en la consola | Parada limpia inmediata (exit 130) |

## 3. Consola Interactiva

`vesper` (sin argumentos) abre la consola estilo msf:

```
  ██╗   ██╗███████╗███████╗██████╗ ███████╗██████╗ ...
  semi-autonomous red team platform  v1.0.0

  POSTURE  ● LAB-ONLY — sandboxed file ops, loopback bind only

  ╭─ INTERACTIVE CONSOLE ╮
  │  Type help for a list of commands.                    │
  │  Use use <module> to load a module.                   │
  ╰───────────────────────────────────────────────────────╯

  Sessions 0     Hosts 0     Campaigns 0     Bridge ● connected
```

Comandos principales:

| Comando | Descripción |
|---------|-------------|
| `use <module>` / `search <term>` / `info <module>` | Cargar, buscar y detallar módulos |
| `show options` / `set <opt> <val>` / `unset` | Configurar el módulo cargado |
| `run` / `exploit` | Ejecutar el módulo **vía bridge** (sin bridge falla honestamente: no simula resultados) |
| `campaign start\|list\|status\|pause\|resume` | Ciclo de vida de campañas |
| `killchain` | Progreso visual de la kill chain |
| `hosts` / `services` / `vulns` / `creds` | Hallazgos de la sesión |
| `sessions` / `session -i <id>` | Sesiones de agentes registrados |
| `suggest` / `ai` / `accept <#>` / `reject <#>` | Motor de decisiones |
| `lab up\|down\|status` | Docker compose del laboratorio (real) |
| `kill_switch <code>` | Parada de emergencia |
| `exit` / `[Ctrl+C]` | Salir |

Flujo verificado (bridge conectado):

```
[vesper] › use recon
  [+] Module: recon
      Network reconnaissance (nmap when available, lab targets only)

[vesper] (recon) › show options
  Option  Value      Description
  ──────  ─────────  ─────────────────────
  TARGET  (not set)  Target IP/hostname
  MODE    basic      Scan mode: basic | stealth

[vesper] (recon) › set TARGET 127.0.0.1
  TARGET       → 127.0.0.1

[vesper] (recon) › run
  [*] Executing recon
  + Module executed via bridge.
```

Con el bridge offline, `run` **no inventa salida**:

```
  ✗ Bridge offline — module NOT executed.
  ~ Vesper never simulates results. Start the bridge with `vesper --dashboard` and retry.
```

## 4. Modo CLI

### Reconocimiento

```bash
# Scanner TCP nativo (real: connect + banner grabbing, 32 puertos comunes)
vesper recon scan -t 127.0.0.1
#   + Scan complete — 2/32 ports open
#     Port  State  Service  Banner
#     6379  open   redis    -ERR ERR unknown command...

# Con bridge conectado usa el módulo recon del bridge (nmap si está instalado)
# DNS real (A/AAAA, MX, NS, TXT vía resolver del sistema)
vesper recon dns -d example.com
```

> `vesper recon osint` aún no está implementado (roadmap v1.2) y lo dice
> explícitamente en su salida.

### Otros grupos

```bash
vesper campaign start --name "Lab-Demo" --target 127.0.0.1
vesper agent list
vesper listeners add --type tcp --port 8443   # bind real (usa --host 127.0.0.1)
vesper lab up | down | status                  # docker compose real
```

## 5. Campañas y Orquestador

Las campañas avanzan por la kill chain (Recon → Weaponize → … → Objectives)
dirigidas por el orquestador. El dispatcher mapea tácticas ATT&CK a módulos
**reales**:

| Táctica | Módulo despachado |
|---------|-------------------|
| Reconnaissance / Discovery | `recon` (bridge) |
| Credential Access / Collection | `cred_dump/dump` (bridge) |
| Cualquier otra | `""` — se registra `no module mapped` y la fase avanza sin fingir ejecución |

Cada decisión y despacho queda en el audit log (`LogAudit`), con resultados
`success` / `failed` / `bridge_error` reales.

## 6. Dashboard

```bash
vesper --dashboard     # API REST + WebSocket + C2 en 127.0.0.1:9090
cd web && npm run build   # frontend (opcional)
```

- `GET /api/health` — estado + uptime
- `GET/POST /api/campaigns` — campañas
- `GET /api/sessions`, `/api/creds`, `/api/modules` — estado en vivo
- `/ws` y `/ws/terminal` — eventos en tiempo real y terminal xterm.js contra la consola real
- Auth: JWT de dashboard (`dashboard.auth_token` en config) + rate limit + CORS

El listado de módulos del dashboard (`/api/modules`) muestra el catálogo
honesto: solo capacidades que existen en el repositorio.

## 7. Módulos del Bridge

El bridge Python (`modules/bridge/bridge_grpc.py`) expone:

**Módulos inline** (registro `ModuleRegistry`): `recon`, `ai_analyze` (Ollama
opcional), `privesc`*, `persist`*, `worm`*, `blue`, `evasion`*, `report`,
`exfil`*, `health` — los marcados con * son stubs que devuelven ceros de forma
honesta (nunca `success` fingido).

**Grupos de handlers** (`modules/bridge/handlers/`):

| Grupo | Funciones |
|-------|-----------|
| `attacks` | `responder` (LLMNR/NBT-NS, solo lab), `webscan` (SQLi/forms/endpoints), `cloud`, `cleanup`, `obfuscate` |
| `cred_dump` | `dump` (almacén sandboxeado en `lab_root/`) |
| `bloodhound` | `collect` (ingest BloodHound) |
| `reporting` | `attack_layer` (capa MITRE ATT&CK Navigator desde datos de campaña) |

Invocación desde Go: `Bridge.Call(ctx, "cred_dump", "dump", params)` o, desde
la consola, `use cred_dump/dump → set TARGET … → run`.

## 8. Laboratorio Docker

```bash
vesper lab up        # docker compose -f lab/docker-compose.yml up -d
vesper lab status    # docker compose ps
vesper lab down
```

`lab/docker-compose.yml` define la red aislada `172.20.0.0/24` (attacker,
targets, dashboard, ollama). Existe además `lab/docker-compose.edr.yml` (SIEM
Elasticsearch/Kibana + objetivo Windows) para validar detección.

> Los hosts víctimas con vulnerabilidades deliberadas (DVWA, Juice Shop, Redis
> sin auth…) están planificados para v1.2 en el roadmap.

## 9. Generación de Payloads

```bash
vesper payload generate --os linux --arch amd64
vesper payload generate --os windows --arch amd64 --c2 10.0.0.1:8443
```

Compila el agente real (`cmd/implant`) con `go build` cross-compilado.
El comando `builder` de la consola no compila: te remite a este comando real.

## 10. Solución de Problemas

| Síntoma | Causa | Solución |
|---------|-------|----------|
| `Bridge offline — module NOT executed` | grpcio no instalado o bridge caído | `pip install -r requirements.txt`; el bridge arranca solo con el estado |
| `context deadline exceeded` al arrancar el bridge | grpcio ausente en el sistema | Instalar dependencias Python |
| `Docker compose failed` | Docker no disponible | Instalar Docker Compose v2 o usar modo CLI sin lab |
| El dashboard muestra `● disconnected` | Bridge sin iniciar | Arrancar `vesper --dashboard` |
| Bind rechazado en 0.0.0.0 | Postura LAB-ONLY | Es lo esperado; usa `--yes-i-am-authorized` solo con autorización escrita |

## 11. Estructura del Proyecto

```
Vesper/
├── cmd/
│   ├── vesper/          CLI: consola, dashboard, subcomandos cobra
│   └── implant/         Agente beacon (payload real)
├── internal/
│   ├── agent/           BridgeClient (gRPC) y registro de agentes
│   ├── api/             API REST + WebSocket del dashboard
│   ├── appstate/        Estado compartido + catálogo honesto de módulos
│   ├── c2server/        Servidor C2
│   ├── crypto/          Primitivas criptográficas
│   ├── defense/         Métricas BlueForge
│   ├── dispatch/        Dispatcher decisión→módulo (mapeo honesto)
│   ├── orchestrator/    Orquestador de campañas + world graph
│   ├── plugins/         Cargador de plugins .so
│   ├── recon/           Scanner TCP nativo (connect + banner)
│   └── registry/        Registro de módulos Go
├── modules/bridge/      Bridge Python (gRPC) + handlers reales
├── pkg/proto/           Contratos gRPC (vesper.v1)
├── pkg/shared/          config, types, logger
├── web/                 Dashboard Vue 3 + Pinia + xterm.js
├── lab/                 docker-compose.yml + compose EDR
├── docs/                Este manual, arquitectura, API, comandos
└── scripts/             Utilidades (incluye generador del banner)
```

### Documentación adicional

| Documento | Contenido |
|-----------|-----------|
| `README.md` / `README.es.md` | Visión general y quick start |
| `docs/ARCHITECTURE.md` | Arquitectura de componentes |
| `docs/API_REFERENCE.md` | Endpoints REST/WS |
| `docs/COMMANDS.md` | Referencia de comandos |
| `docs/TESTING_GUIDE.md` | Cómo ejecutar los tests |
| `ROADMAP.md` | Plan trimestral (v1.1 → v2.0) |
| `CHANGELOG.md` | Historial honesto de cambios |
| `SECURITY.md` | Política de seguridad y divulgación |
