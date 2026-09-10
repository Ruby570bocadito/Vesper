# Vesper — API Reference (v3.2)

## Endpoints

All endpoints are served by the Go API server at `http://localhost:8443` (configurable via `--api-port`).
The dashboard proxies `/api` → Go server and `/ws` → WebSocket hub.

### REST API

#### Agents

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/agents` | List all agents. Query: `?campaign_id=X` |
| `GET` | `/api/agents/:id` | Get agent details |
| `POST` | `/api/agents/:id/kill` | Kill an agent. Body: `{"reason":"..."}` |

#### Campaigns

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/campaigns` | List all campaigns |
| `POST` | `/api/campaigns` | Create campaign. Body: `{"name","target_scope","goal","profile","auto_approve"}` |
| `GET` | `/api/campaigns/:id` | Get campaign details |
| `POST` | `/api/campaigns/:id/pause` | Pause a running campaign |
| `POST` | `/api/campaigns/:id/resume` | Resume a paused campaign |

#### Recon

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/hosts` | List discovered hosts. Query: `?campaign_id=X` |
| `GET` | `/api/services` | List discovered services. Query: `?campaign_id=X` |
| `GET` | `/api/vulnerabilities` | List vulnerabilities. Query: `?campaign_id=X` |
| `POST` | `/api/recon/scan` | Trigger scan. Body: `{"target":"10.0.0.1","mode":"quick"}` |

#### AI / Decisions

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/ai/chat` | Chat with AI. Body: `{"prompt":"analyze target..."}` |
| `GET` | `/api/decisions` | List pending decisions. Query: `?campaign_id=X` |
| `POST` | `/api/decisions/:id/approve` | Approve a decision |
| `POST` | `/api/decisions/:id/reject` | Reject a decision |

#### Metrics

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/metrics` | Get campaign KPIs. Query: `?campaign_id=X` |
| `GET` | `/api/blue/metrics` | Get BlueForge detection metrics |

#### Payloads

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/payload/generate` | Generate implant payload. Body: `{"os","arch","format","lhost","lport","amsi","unhook","encoder"}` |

#### Phantom (Browser Mesh)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/phantom/status` | Get phantom mesh status |
| `GET` | `/api/phantom/nodes` | List phantom browser nodes |
| `POST` | `/api/phantom/:action` | Execute phantom action (inject, steal, etc.) |

#### Config

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/config/ai` | Get AI configuration |
| `PUT` | `/api/config/ai` | Update AI config. Body: `{"model","temperature"}` |

#### Admin

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/health` | Health check: `{"status":"ok","version":"3.2","uptime":3600}` |
| `GET` | `/api/modules` | List all registered modules |
| `POST` | `/api/modules/push` | Push module to agent |
| `GET` | `/api/sessions` | List active sessions |
| `GET` | `/api/creds` | List captured credentials |

### WebSocket API

Connect to `ws://localhost:8443/ws?campaign_id=X`

**Event types broadcast by the server:**

| Event | Payload |
|-------|---------|
| `agent.checkin` | `{agent_id, hostname, os, ip, timestamp}` |
| `agent.dead` | `{agent_id, reason, timestamp}` |
| `campaign.started` | `{campaign_id, name, phase}` |
| `campaign.paused` | `{campaign_id}` |
| `campaign.resumed` | `{campaign_id}` |
| `decision.made` | `{decision_id, tactic, technique, mitre_id, confidence}` |
| `host.discovered` | `{ip, hostname, os, ports}` |
| `vuln.found` | `{cve, severity, target_ip, service}` |
| `recon.scan_complete` | `{target, hosts_found, vulns_found}` |
| `recon.scan_error` | `{target, error}` |
| `phase.changed` | `{campaign_id, from, to, progress}` |
| `blue.alert` | `{tool, alert_type, timestamp}` |
| `exploit.success` | `{target, exploit, cve}` |
| `exploit.failure` | `{target, exploit, error}` |
| `credential.captured` | `{username, domain, source}` |

### gRPC Services

The C2 server exposes three gRPC services at the configured port:

**AgentService** (`vesper.v1.AgentService`)
```
rpc CheckIn(CheckInRequest) returns (CheckInResponse)
rpc CommandStream(stream AgentMessage) returns (stream ServerMessage)
rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse)
rpc Exfiltrate(stream ExfilChunk) returns (ExfilAck)
```

**C2Service** (`vesper.v1.C2Service`)
```
rpc ListAgents(ListAgentsRequest) returns (ListAgentsResponse)
rpc GetAgent(GetAgentRequest) returns (AgentInfo)
rpc KillAgent(KillAgentRequest) returns (KillAgentResponse)
rpc CreateCampaign(CreateCampaignRequest) returns (Campaign)
rpc GetCampaign(GetCampaignRequest) returns (Campaign)
rpc ListCampaigns(ListCampaignsRequest) returns (ListCampaignsResponse)
rpc PauseCampaign(PauseCampaignRequest) returns (Campaign)
rpc ResumeCampaign(ResumeCampaignRequest) returns (Campaign)
rpc DecisionFeed(stream DecisionUpdate) returns (stream DecisionAck)
rpc GetMetrics(MetricsRequest) returns (MetricsResponse)
```

**BridgeService** (`vesper.v1.BridgeService`)
```
rpc ExecuteModule(ModuleRequest) returns (ModuleResponse)
rpc AIAnalyze(AIAnalyzeRequest) returns (stream AIAnalyzeResponse)
rpc ReconStream(ReconRequest) returns (stream ReconResponse)
rpc HealthCheck(HealthCheckRequest) returns (HealthCheckResponse)
```

## Python Bridge Handlers

### Registry Groups

| Group | Handlers | File |
|-------|----------|------|
| `phase_1_4` | byovd_loader, dkom, amsi_patch, etw_patch, syscall_proxy, hollowing, unhook_ntdll, evasion_misc, otp, sandbox_detect, network_covert, persist_scheduled, persist_wmi, persist_registry, stego_config, x25519_wireguard, quic_tunnel, webrtc_p2p, beacon_dns, beacon_https, beacon_smb, obfuscate_code, packer_upx, crypter_xor, embed_payload, rsrc_hide, connect_back, bind_shell, pivot_socks5, relaying, ai_target, ai_phishing, ai_deepfake, ai_vishing, c2_waterfall, c2_cloudfront | `phase_1_4.py` |

### Calling a Handler

From Go:
```go
    "root": "/home",
    "max_files": 500,
})
```

From Python:
```python
result = handle_scan({"root": "/home", "max_files": 500})
```

All handlers accept `params: dict` and return `dict` with at minimum `{"success": bool}`. Handlers respect `{"simulation": true}` (default) to avoid destructive operations.
