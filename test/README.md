# Vesper — Suite de Testing Integral

## Estructura de directorios

```
test/
├── README.md                         # Este archivo
├── run_all.sh                        # Script maestro — ejecuta toda la suite
│
├── go/                               # Runners para tests Go
│   ├── run_core.sh                   # tests internal/core/...
│   ├── run_ransomware.sh             # tests internal/ransomware/...
│   ├── run_blockz.sh                 # tests internal/ransomware/blockz/...
│   ├── run_v210.sh                   # tests internal/ransomware/v210/...
│   ├── run_v26.sh                    # tests internal/ransomware/v26/...
│   ├── run_v27.sh                    # tests internal/ransomware/v27/...
│   ├── run_v28.sh                    # tests internal/ransomware/v28/...
│   ├── run_v29.sh                    # tests internal/ransomware/v29/...
│   ├── run_v30.sh                    # tests internal/ransomware/v30/...
│   └── run_hydra_vectors.sh          # tests internal/ransomware/hydra_vectors/...
│
├── python/                           # Runners para tests Python
│   ├── run_bridge.sh                 # tests modules/bridge/tests/
│   └── run_plugins.sh                # tests plugins/*/tests/
│
├── integration/                      # Tests de integración
│   └── run_integration.sh            # Bridge + C2 + Dispatcher
│
├── e2e/                              # Tests end-to-end
│   ├── run_killchain.sh              # Ciclo kill chain completo
│   └── run_campaign.sh               # Campaña multi-agente
│
├── security/                         # Tests de seguridad y evasión
│   └── run_evasion.sh                # AMSI, sandbox, syscalls, etc.
│
├── benchmark/                        # Benchmarks de rendimiento
│   └── run_benchmarks.sh             # Encrypt, scanner, mutation, RPC
│
└── ci/                               # Pipeline CI/CD
    └── workflow.yml                  # GitHub Actions workflow
```

## Fases de implementación

| Fase | Contenido | Archivos | Estado |
|------|-----------|----------|--------|
| F1 | Tests Go Core (appstate, dispatch, bridge, c2, api, registry, defense) | `internal/*/*_test.go` | ✅ |
| F2 | Tests Go Ransomware (90+ módulos ofensivos) | `internal/ransomware/**/*_test.go` | ✅ |
| F3 | Tests Python Bridge (12 handlers + protocolo) | `modules/bridge/tests/*.py` | ✅ |
| F4 | Tests de Integración (Go ↔ Python, Dispatcher ↔ Registry) | `test/integration/*.sh` | ✅ |
| F5 | Tests End-to-End (kill chain, campañas) | `test/e2e/*.sh` | ✅ |
| F6 | Tests de Seguridad y Evasión | `test/security/*.sh` (cubierto en F2) | ✅ |
| F7 | Benchmarks de Rendimiento | `test/benchmark/*.sh` | ✅ |
| F8 | CI/CD Pipeline (GitHub Actions) | `test/ci/workflow.yml` | ✅ |

## Cobertura por paquete

```
internal/
├── appstate/       → state_test.go       (unit + mock)
├── dispatch/       → dispatcher_test.go  (unit + mock interfaces)
├── bridge/         → bridge_test.go      (unit + mock TCP)
├── c2server/       → server_test.go      (unit + mock gRPC)
├── api/            → api_test.go         (unit + httptest)
├── registry/       → registry_test.go    (unit + mock Module)
├── defense/        → defense_test.go     (unit)
├── crypto/         → crypto_test.go      (existente, mantener)
├── agent/          → agent_test.go       (existente, expandir)
└── orchestrator/   → orchestrator_test.go (existente, expandir)

internal/ransomware/
├── evasion_test.go           (anti_reversing + antianalysis + antiforensics)
├── polymorph_test.go         (polymorph + jit_polymorphism + dna_mutation)
├── byovd_test.go            (byovd_loader + dkom)
├── persistence_test.go      (wer_persistence + bootkit + bootkit_uefi)
├── wfp_test.go              (wfp_dns_poison + v29/wfp_driver)
├── c2_channels_test.go      (multi_channel_c2 + blockchain_c2)
├── propagation_test.go      (propagation + network_poison + chronos_ntp + scada)
├── cloud_id_test.go         (cloud_exploit + imdsv2 + kerberos + identity + trust)
├── cross_platform_test.go   (supply_chain + multiplatform_worm + bluetooth + lolbin + loader + stager)
├── engine_test.go           (engine + engine_extended + types)
├── destruction_test.go      (destruction + hardware_kill)
├── psycho_test.go           (extortion + psychological + survivor_game)
├── hydra_test.go            (hydra + scanner)
├── raas_test.go             (raas_inverse)
├── blockz/blockz_test.go          (14 engines BlockZ)
├── v210/v210_test.go              (Apocalipsis + Phantom Evasion)
├── v26/v26_test.go                (POMDP + AI + SMM + Omega)
├── v27/v27_test.go                (Blue Pill + UEFI + PCIe + Phishing)
├── v28/v28_test.go                (24 módulos Malice)
├── v29/v29_test.go                (24 módulos Network Immolation)
├── v30/v30_test.go                (AD + Payroll)
└── hydra_vectors/hydra_vectors_test.go  (8 Hydra vectors)
```

## Cómo usar

```bash
# Ejecutar toda la suite
bash test/run_all.sh

# Ejecutar solo tests Go core
bash test/go/run_core.sh

# Ejecutar solo tests ransomware
bash test/go/run_ransomware.sh

# Ejecutar solo tests Python bridge
bash test/python/run_bridge.sh

# Ejecutar benchmarks
bash test/benchmark/run_benchmarks.sh

# Ver cobertura
bash test/go/run_core.sh --cover
go test -coverprofile=coverage.out ./internal/ransomware/...
go tool cover -html=coverage.out
```
