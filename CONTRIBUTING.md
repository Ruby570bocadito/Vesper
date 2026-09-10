# Contributing to Vesper (v3.2)

## Code of Conduct

Vesper is an academic project for cybersecurity education. All contributions must align with the ethical guidelines:

1. Only use in authorized environments (labs, CTFs, penetration tests with written permission).
2. Do not submit exploits against live targets.
3. All AI operations use local models (Ollama) — no data exfiltration.

## Getting Started

```bash
git clone https://github.com/Ruby570bocadito/Vesper.git
cd Vesper
go build -o vesper ./cmd/vesper/
pip install -r requirements.txt
```

## Project Structure (Monorepo)

```
Vesper/
├── cmd/vesper/           # CLI entry point (Cobra + TUI + Console)
├── internal/            # Go core packages
│   ├── agent/           # Unified Go implant + bridge client
│   ├── api/             # REST API + WebSocket hub
│   ├── appstate/        # Campaign/agent state management
│   ├── bridge/          # WASM bridge loader (Wazero)
│   ├── c2server/        # gRPC C2 server
│   ├── crypto/          # X25519 + SPIFFE mTLS
│   ├── defense/         # BlueForge ATT&CK engine
│   ├── dispatch/        # Module dispatcher
│   ├── orchestrator/    # Decision engine
│   ├── ransomware/      # Ransomware modules (12 packages)
│   └── registry/        # Dynamic module registry
├── pkg/
│   ├── proto/           # Protobuf definitions + generated stubs
│   └── shared/          # Types, database, config
├── modules/bridge/      # Python bridge (gRPC server + 107 handlers)
├── plugins/             # Specialized modules (11 plugins)
├── web/                 # Vue 3 dashboard SPA
├── test/                # Test harness (7 phases F1-F7)
└── docs/                # Documentation
```

## Adding a Module

### Go Module (internal/)

```go
package ransomware

type MyModule struct{}

func (m *MyModule) Name() string { return "my_module" }
func (m *MyModule) Phase() string { return "exploitation" }
func (m *MyModule) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
    return map[string]interface{}{"status": "success"}, nil
}
```

Register in `internal/ransomware/engine.go` or via the registry.

### Python Bridge Handler (modules/bridge/handlers/)

```python
def register_routes(registry: dict) -> None:
    registry["my_module"] = {
        "my_handler": handle_my_handler,
    }

def handle_my_handler(params: dict) -> dict:
    simulation = params.get("simulation", True)
    result = {"success": True}
    if not simulation:
        # real operation
        pass
    return result
```

Register by calling `register_routes()` in `modules/bridge/bridge.py`.

## Testing

```bash
# Go tests
go test ./internal/... -timeout 60s

# Python bridge tests
python3 -m unittest modules.bridge.tests.test_handlers -v

# Smoke test (107 handlers)
python3 modules/bridge/tests/test_all_handlers.py

# Master test suite (all 7 phases)
bash test/run_all.sh
```

## Pull Request Process

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing`)
3. Add tests for new functionality
4. Run `bash test/run_all.sh` to verify everything passes
5. Update documentation if needed
6. Submit PR with description of changes

## License

MIT License — see [LICENSE](LICENSE) for details.
