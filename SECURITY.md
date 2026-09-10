# Security Policy — Vesper

## The safety model (what the code actually enforces)

Vesper is a red-team **education platform**. Its safety posture is enforced
in code, not in documentation:

| Control | Enforcement point | Default |
|---|---|---|
| Lab-only posture | `cmd/vesper/safety.go` → sets `VESPER_LAB_ONLY=1` for child processes | **ON** |
| File sandbox | `modules/bridge/safety.py` — `resolve_in_lab()` refuses any path outside `VESPER_LAB_ROOT` | ON |
| Authorization gate | CLI flag `--yes-i-am-authorized` or env `VESPER_AUTHORIZED=1` | required for live ops |
| Bind protection | unauthorized runs override `server.host` to `127.0.0.1` | ON |
| Runtime kill switch | env `VESPER_KILLSWITCH=1` (refuses to start) or console command `kill_switch EMERGENCY_STOP` | available |
| Destructive handlers | `run_cleanup` log-wipe/timestamps/persistence-removal require authorization; sandboxed copies otherwise | ON |
| Dashboard auth | startup warning when `auth_token` is empty | ON |
| CI mutation guard | the `bridge` CI job fails if tests modify any tracked file | ON |

## What Vesper will NOT do

- No ransomware, no destructive payloads, no "psychological operations"
  modules (all removed in the 1.0.0 remodel)
- No persistence installation outside the lab sandbox without authorization
- No telemetry, no phone-home, no hard-coded infrastructure
- Tests never write to repository files (enforced by CI)

## Reporting a vulnerability

Open a GitHub Security Advisory ("Report a vulnerability" on the Security
tab) or contact the maintainer directly. Please include reproduction steps
and affected commit.

## Responsible use

Use only against systems you own or have **written** authorization to test.
The default posture is lab-only; bypassing it against third-party systems is
illegal in most jurisdictions and against the spirit of this project.
