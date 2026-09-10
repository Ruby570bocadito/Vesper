# Vesper — Architecture Documentation (v3.2)

## System Overview

Vesper is a semi-autonomous Red Team platform covering the complete cyber kill chain:
Reconnaissance → Weaponization → Delivery → Exploitation → Installation → C2 → Actions on Objective → Exfiltration.

Built as a monorepo with Go backend, Python gRPC bridge, Vue 3 frontend, and 154+ modules across 45 categories.

```
                         ┌─────────────────────────────────┐
                         │        USER INTERFACES           │
                         │  CLI · Console · TUI · Dashboard  │
                         │  (Vue 3 SPA + WebSocket live)    │
                         └────────┬───────────────┬─────────┘
                                  │ HTTP REST + WS│
                         ┌────────▼───────────────▼─────────┐
                         │          ORCHESTRATOR             │
                         │  Campaign / Decision / WorldGraph │
                         │  Rules Engine + AI Engine         │
                         │  tactic→module mapping per phase  │
                         └────────┬─────────────────────────┘
                                  │ dispatch module calls
                         ┌────────▼─────────────────────────┐
                         │        C2 SERVER (Go/gRPC)       │
                         │  AgentService · C2Service        │
                         │  BridgeService · WebSocket hub   │
                         │  REST API (:8443) · Health       │
                         └────────┬─────────────────────────┘
                                  │ gRPC (X25519+XChaCha20)
                         ┌────────▼─────────────────────────┐
                         │        UNIFIED AGENT (Go)        │
                         │  internal/agent/                 │
                         │  BridgeClient · Connector        │
                         └──┬──────┬───────┬──────┬─────────┘
                            │      │       │      │
                     ┌──────┐ ┌────┐ ┌────┐ ┌──────────┐
                     │Go    │ │Go  │ │Go  │ │Python    │
                     │Ransom│ │Priv│ │C2  │ │Bridge    │
                     │ware  │ │Esc │ │    │ │gRPC      │
                     └──────┘ └────┘ └────┘ └─────┬─────┘
                                                   │
                         ┌─────────────────────────┼─────────┐
                         │                         │         │
                    ┌────▼────┐ ┌────────────┐ ┌──▼─────┐ ┌──▼──────────┐
                    │107      │ │Worm        │ │Apex+   │ │BlueForge    │
                    │Bridge   │ │Propagation │ │Specter │ │ATT&CK       │
                    │Handlers │ │40+ exploits│ │AI      │ │Coverage     │
                    └─────────┘ └────────────┘ └────────┘ └─────────────┘
```

## gRPC Service Architecture (v3.2)

All internal communication uses gRPC with protobuf-defined schemas:

**AgentService** (agent ↔ C2):
- `CheckIn(CheckInRequest) → CheckInResponse` — agent registration
- `CommandStream(stream AgentMessage) → stream ServerMessage` — bidirectional tasking
- `Heartbeat(HeartbeatRequest) → HeartbeatResponse` — keepalive
- `Exfiltrate(stream ExfilChunk) → ExfilAck` — data exfiltration stream

**C2Service** (orchestrator/console ↔ C2):
- `ListAgents`, `GetAgent`, `KillAgent` — agent lifecycle
- `CreateCampaign`, `GetCampaign`, `ListCampaigns`, `PauseCampaign`, `ResumeCampaign` — campaign management
- `DecisionFeed(stream DecisionUpdate) → stream DecisionAck` — bidirectional AI decisions
- `GetMetrics(MetricsRequest) → MetricsResponse` — C2 health metrics

**BridgeService** (Go ↔ Python):
- `ExecuteModule(ModuleRequest) → ModuleResponse` — call any Python handler
- `AIAnalyze(AIAnalyzeRequest) → stream AIAnalyzeResponse` — AI streaming responses
- `ReconStream(ReconRequest) → stream ReconResponse` — recon data streaming
- `HealthCheck(HealthCheckRequest) → HealthCheckResponse` — bridge health

All communication uses X25519 ECDH key exchange + XChaCha20-Poly1305 AEAD encryption over gRPC.

## Directory Structure (Monorepo)

```
Vesper/
├── cmd/                    # Go binaries (vesper CLI, deployment)
├── internal/               # Go core packages
│   ├── agent/              # Implant agent + bridge client
│   ├── api/                # REST API server + WebSocket hub
│   ├── appstate/           # Campaign/agent state management
│   ├── bridge/             # WASM bridge loader (Wazero)
│   ├── c2server/           # gRPC C2 server (AgentService + C2Service)
│   ├── crypto/             # X25519, SPIFFE mTLS, signing
│   ├── defense/            # BlueForge ATT&CK coverage engine
│   ├── dispatch/           # Module dispatcher (registry → handler)
│   ├── orchestrator/       # Decision engine (Rules + AI)
│   └── registry/           # Dynamic module registry
├── pkg/
│   ├── proto/              # Protobuf definitions (agent, bridge, c2, common)
│   │   └── gen/            # Generated Go gRPC stubs
│   └── shared/             # Shared types, database models
├── modules/
│   └── bridge/             # Python gRPC bridge server
│       └── tests/          # Python tests (21 unit + smoke)
├── plugins/                # Specialized modules
│   ├── ai/                 # Specter + Apex AI engines
│   ├── worm/               # Network worm + MITRE mapper
│   ├── operations/         # Argos C2 (Python gRPC)
│   ├── pulse-c2/           # Encrypted protobuf C2 (Go+Python)
│   ├── blue/               # BlueForge defense suite
│   ├── kernel/             # Vault kernel-level operations
│   ├── privesc/            # Privilege escalation
│   ├── breach/             # Exploitation entry points
│   ├── recon/              # Reconnaissance modules
│   └── relay/              # C2 relay + IoT vision
├── web/                    # Vue 3 dashboard SPA
│   └── src/
│       ├── views/          # 9 tab-based views
│       ├── components/     # KillChain, ActivityFeed, etc.
│       └── stores/         # Pinia state management
├── test/                   # Test harness (7 phases F1-F7)
│   ├── go/                 # Go test scripts (10 packages)
│   ├── python/             # Python test scripts
│   ├── integration/        # Integration tests
│   ├── e2e/                # End-to-end kill chain + campaign
│   ├── security/           # Evasion/AV tests
│   ├── benchmark/          # Performance benchmarks
│   ├── ci/                 # GitHub Actions workflow
│   └── run_all.sh          # Master test suite runner
├── docs/                   # Documentation
│   ├── USAGE.md            # Full usage manual (1,393 lines)
│   ├── MODULES.md          # 107 bridge handler reference
│   ├── COMMANDS.md         # CLI command reference (843 lines)
│   ├── MODULES.md            # Complete module catalog
│   ├── ARCHITECTURE.md     # This file
│   ├── API_REFERENCE.md    # REST/gRPC API reference
│   ├── DEPLOYMENT.md       # Deployment guide
│   ├── CREATIVITY.md       # Academic: creative innovations
│   ├── MEMORIA_TFG.md      # Academic: thesis memory
│   ├── KILL_CHAIN_MATRIX.md # Kill chain tactic→module matrix
│   ├── BENCHMARKS.md       # Performance benchmarks
│   ├── COMMANDS.md          # CLI command reference
│   └── TESTING_GUIDE.md    # Testing instructions
└── reports/                # Campaign output reports
```

## Component Map

| Component | Language | Package | Phase |
|-----------|----------|---------|-------|
| CLI / Console | Go | `cmd/` | All |
| Web Dashboard | Vue 3 + JS | `web/` | All |
| REST API | Go | `internal/api/` | All |
| WebSocket Hub | Go | `internal/api/` | All |
| C2 Server (gRPC) | Go | `internal/c2server/` | C2 |
| Orchestrator | Go | `internal/orchestrator/` | All |
| Decision Engine | Go | `internal/orchestrator/` | Decision |
| Dispatcher | Go | `internal/dispatch/` | Module |
| Unified Agent | Go | `internal/agent/` | All |
| Bridge Client | Go | `internal/agent/` | IPC |
| Crypto / SPIFFE | Go | `internal/crypto/` | Shared |
| Protobuf | .proto | `pkg/proto/` | Shared |
| Python Bridge (gRPC) | Python | `modules/bridge/` | IPC |
| Worm Propagation | Python | `plugins/worm/` | Lateral |
| Specter AI | Python | `plugins/ai/specter/` | AI |
| Apex Automation | Python | `plugins/ai/apex/` | AI |
| Argos C2 | Python+gRPC | `plugins/operations/` | C2 |
| Pulse-C2 | Go+Python | `plugins/pulse-c2/` | C2 |
| BlueForge Defense | Go | `internal/defense/` | Metrics |
| Kernel Operations | C+Go | `plugins/kernel/` | Install |
| Privilege Escalation | Go | `plugins/privesc/` | Exploit |
| Breach Exploits | C+Python | `plugins/breach/` | Delivery |
| Recon Modules | Python | `plugins/recon/` | Recon |
| Relay C2 | Python | `plugins/relay/` | C2 |
| Blue Team | Python | `plugins/blue/` | Defense |

## Communication Flow

```
                    ┌──────────────────┐
                    │   Orchestrator   │
                    └────────┬─────────┘
                             │ dispatch.Call()
                    ┌────────▼─────────┐
                    │   C2 Server      │
                    │  (Go gRPC)       │
                    │  :8443 HTTP+WS   │
                    └────────┬─────────┘
                             │ gRPC (X25519 + XChaCha20-Poly1305)
                    ┌────────▼─────────┐
                    │  Unified Agent   │
                    │  internal/agent/ │
                    └───┬──┬──┬──┬────┘
                        │  │  │  │
              ┌─────────┘  │  │  └────────┐
              ▼            ▼  ▼           ▼
        ┌──────────┐ ┌──────────┐  ┌──────────────┐
        │ Go Module│ │ Go Module│  │Python Bridge │
        │ (ransom, │ │ (privesc,│  │ (gRPC)       │
        │  crypto) │ │  kernel)  │  └──────┬───────┘
        └──────────┘ └──────────┘         │
                               ┌──────────┼──────────┐
                               ▼          ▼          ▼
                          ┌────────┐ ┌────────┐ ┌────────┐
                          │Recon   │ │AI      │ │Worm    │
                          │Modules │ │Specter │ │Propag  │
                          └────────┘ └────────┘ └────────┘
```

## Decision Engine Flow

```
                    ┌─────────────────────────────┐
                    │       RECON DATA IN         │
                    │  (Bridge handlers, OSINT)   │
                    └─────────────┬───────────────┘
                                  │
            ┌─────────────────────┼─────────────────────┐
            ▼                     ▼                     ▼
   ┌────────────────┐    ┌────────────────┐    ┌────────────────┐
   │ Rules Engine   │    │ Pathfinding    │    │ AI Engine      │
   │ (Deterministic)│    │ (Tactic Map)   │    │ (Ollama local) │
   │ Weight: 25%    │    │ Weight: 35%    │    │ Weight: 40%    │
   └───────┬────────┘    └───────┬────────┘    └───────┬────────┘
           │                     │                     │
           └─────────────────────┼─────────────────────┘
                                 │
                      ┌──────────▼──────────┐
                      │  DECISION FUSION    │
                      │  (Weighted Rank)    │
                      └──────────┬──────────┘
                                 │
                      ┌──────────▼──────────┐
                      │  HUMAN-IN-THE-LOOP  │
                      │  (Approve / Reject) │
                      └──────────┬──────────┘
                                 │
                      ┌──────────▼──────────┐
                      │  EXECUTE ACTION     │
                      │  (via Agent → C2)   │
                      └─────────────────────┘
```

## Data Flow: Go → Python Bridge

```
Go Orchestrator
  │
  │
  ├─ Registry lookup: Go module → found? → execute Go
  │
  └─ BridgeClient (internal/agent/bridge_client.go)
       │
       ├─ gRPC call: BridgeService.ExecuteModule(ModuleRequest)
       │
       └─ Python Bridge Server (modules/bridge/bridge.py)
            │
            │
            │
            └─ Return: ModuleResponse { success: true, result: {...}, elapsed_ms: 4 }
```

## Database

The app state is managed in-memory with optional SQLite persistence via `internal/appstate/state.go`.

Key entities:
- **Campaigns**: Red team operations with kill chain phase tracking
- **Agents**: Implant tracking (hostname, OS, status, last checkin)
- **Targets**: Discovered hosts (IP, ports, services, asset value)
- **Vulnerabilities**: CVEs and misconfigurations per target
- **Credentials**: Captured passwords, hashes, tokens
- **Decisions**: AI-suggested actions with MITRE mappings
- **KillChainEntries**: Per-campaign phase transitions logged

## Security Model

1. **End-to-End Encryption**: X25519 ECDH + XChaCha20-Poly1305 AEAD per session
2. **gRPC Transport**: All service-to-service communication over gRPC with protobuf type safety
3. **SPIFFE mTLS**: Workload identity via SPIFFE SVIDs (hourly rotation)
4. **Offline AI**: Ollama runs locally — no data leaves the lab
5. **Safety Controls**: Kill switch, geofencing, auto-destruct, max infection caps
6. **Audit Trail**: Every action logged with timestamp, agent ID, campaign ID, and result
7. **Anti-Forensics**: MFT timestomping, USN journal poisoning, event log wiping
