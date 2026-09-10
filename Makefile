# Vesper — Makefile
# ===================
# make setup      — Install all dependencies (Go + Python + Node)
# make build      — Build all components
# make test       — Run all tests
# make demo       — 10-second safe demo (build + console banner)
# make lab-up     — Start Docker lab environment
# make lab-down   — Stop Docker lab environment
# make proto      — Generate gRPC code from .proto files
# make clean      — Remove build artifacts
# make lint       — Run linters (Go + Python)
# make release    — Build release binaries

.PHONY: setup build test demo lab-up lab-down proto clean lint release help

GO_PACKAGES := ./cmd/... ./internal/... ./pkg/...

# === Setup ===
setup: setup-go setup-python setup-node
	@echo "[+] Setup complete"

setup-go:
	@echo "[*] Downloading Go dependencies..."
	go mod download

setup-python:
	@echo "[*] Installing Python dependencies..."
	pip install -r requirements.txt

setup-node:
	@echo "[*] Installing Node.js dependencies..."
	cd web && npm ci

# === Build ===
build: build-vesper build-agent
	@echo "[+] Build complete"

build-vesper:
	@echo "[*] Building vesper CLI..."
	go build -o dist/vesper ./cmd/vesper
	@echo "  [+] dist/vesper"

build-agent:
	@echo "[*] Building implant beacon..."
	go build -o dist/implant ./cmd/implant
	@echo "  [+] dist/implant"

# === Test ===
test: test-go test-python
	@echo "[+] All tests passed"

test-go:
	@echo "[*] Running Go tests..."
	go test $(GO_PACKAGES) -cover -timeout 120s

test-python:
	@echo "[*] Running Python bridge tests..."
	cd modules/bridge && python3 -m pytest tests/ -q --tb=short

# === Demo ===
demo: build
	@echo "[*] Safe demo — console banner + version, no live operations"
	./dist/vesper version

# === Lab ===
lab-up:
	@echo "[*] Starting Docker lab..."
	docker compose -f lab/docker-compose.yml up -d
	@echo "[+] Lab running at:"
	@echo "    Attacker:  docker exec -it vesper-attacker bash"
	@echo "    Target 1:  docker exec -it vesper-target1 bash"
	@echo "    Dashboard: http://localhost:3000"

lab-down:
	@echo "[*] Stopping Docker lab..."
	docker compose -f lab/docker-compose.yml down

lab-status:
	docker compose -f lab/docker-compose.yml ps

# === Proto ===
proto:
	@echo "[*] Generating protobuf code..."
	protoc --proto_path=pkg/proto \
		--go_out=pkg/proto/gen --go_opt=module=github.com/ruby570bocadito/vesper/pkg/proto/gen \
		--go-grpc_out=pkg/proto/gen --go-grpc_opt=module=github.com/ruby570bocadito/vesper/pkg/proto/gen \
		pkg/proto/common.proto pkg/proto/c2.proto pkg/proto/agent.proto pkg/proto/bridge.proto
	@echo "[+] Proto generation complete"

# === Lint ===
lint: lint-go lint-python
	@echo "[+] Lint complete"

lint-go:
	@echo "[*] Vet + format check..."
	go vet $(GO_PACKAGES)
	test -z "$$(gofmt -l cmd internal pkg)" || (gofmt -l cmd internal pkg && exit 1)

lint-python:
	@echo "[*] Linting Python code..."
	ruff check modules/ scripts/ 2>/dev/null || echo "  [!] ruff not found"

# === Clean ===
clean:
	@echo "[*] Cleaning build artifacts..."
	rm -rf dist/ release/ lab_root/
	find . -type d -name __pycache__ -exec rm -rf {} + 2>/dev/null || true
	find . -type f -name "*.pyc" -delete 2>/dev/null || true
	@echo "[+] Clean complete"

# === Release ===
release: clean build
	@echo "[*] Creating release archive..."
	mkdir -p release
	cp dist/* release/ 2>/dev/null || true
	cp config.yaml release/
	tar -czf vesper.tar.gz release/
	@echo "[+] Release: vesper.tar.gz"

# === Help ===
help:
	@echo "Vesper — Build Commands"
	@echo "========================="
	@echo "  make setup        Install all dependencies"
	@echo "  make build        Build all components"
	@echo "  make test         Run all tests"
	@echo "  make demo         Safe 10-second demo"
	@echo "  make lab-up       Start Docker lab environment"
	@echo "  make lab-down     Stop Docker lab environment"
	@echo "  make lab-status   Show lab container status"
	@echo "  make proto        Generate gRPC code"
	@echo "  make lint         Run linters"
	@echo "  make clean        Remove build artifacts"
	@echo "  make release      Build release binaries"
	@echo "  make help         Show this help message"
