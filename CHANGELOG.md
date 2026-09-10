# Changelog

Todas las fechas son ISO-8601. Formato basado en [Keep a Changelog](https://keepachangelog.com/es-ES/1.1.0/).

## [Unreleased] — ciclo v1.1 (calidad)

### Añadido
- **Scanner TCP nativo** (`internal/recon`): `vesper recon scan -t <ip>` ahora
  realiza un connect-scan real de 32 puertos comunes con captura de banner
  cuando el bridge está offline — sustituye a la tabla inventada
  `22/SSH, 80/HTTP, 443/HTTPS open` que se mostraba antes sin escanear nada
- **`vesper recon dns` real**: enumera A/AAAA, MX, NS y TXT con el resolver
  del sistema (antes imprimía "DNS enumeration complete" sin hacer ninguna
  consulta)
- **Banner Vesper unificado**: el bloque ANSI Shadow con gradiente ocaso se
  renderiza igual en consola, dashboard, `--help` y `vesper version`; la línea
  de identidad lleva la versión (`v1.0.0`) tomada de una única constante
  (`version.go`)
- **Línea de POSTURA** en el banner de la consola: `● LAB-ONLY` (ámbar) o
  `● AUTHORIZED ENGAGEMENT` (verde) siempre visible
- Banner PNG generado desde la salida real del CLI (`docs/images/banner.png`,
  script reproducible en `scripts/gen_banner_png.py`) incrustado en ambos README
- Tests de `internal/api` con httptest (26 tests): flujos de sesión (login/logout,
  cookie, query token), JWT (login de dashboard, expiración, tampering de payload),
  agentes (registro, filtro por campaña, kill), campañas (creación, consulta,
  pause/resume), CORS y rate limiting
- Jobs nuevos en CI: `race` (`go test -race ./...`), `lint` (golangci-lint con
  `.golangci.yml` documentado) y `coverage` (cobertura Go+Python con badge
  auto-committed en `docs/coverage.svg`)

### Eliminado — segunda pasada de honestidad
- `phase_1_4.py`: **37 handlers que respondían `success: true` hardcodeado**
  para capacidades inexistentes (BYOVD, hipervisor Blue Pill, gusano
  ultrasónico QPSK, contagio RF/SS7, deepfake vishing…). El propio docstring
  lo admitía: "stubs returning hardcoded success values"
- Endpoints `/api/phantom/*` + vista `BrowserMesh.vue` + store `phantom.js` del
  dashboard: servían ceros permanentes de una "malla de navegadores" que no
  existe en ninguna parte del código
- Fabricación de sesiones en la consola: `use <módulo> → run` con el bridge
  caído respondía "Exploit sent" y **registraba un agente online falso**
  (`exploit-N`) en el estado compartido que el dashboard mostraba como real;
  ahora un bridge caído significa "módulo NO ejecutado" y ninguna sesión se
  inventa
- Constructor de payloads de la consola que dormía 1.8s, anunciaba "Size:
  2.4 MB" y un fichero en `dist/` que nunca escribió; ahora remite al
  compilador real (`vesper payload generate`, `go build` real)
- `lab up/down/status` de la consola imprimía una tabla de contenedores
  hardcodeada sin tocar Docker; ahora ejecuta `docker compose` real
- Catálogo de ~180 módulos ficticios en `state.go` (v26–v210, blockz, hydra,
  `ransomware/*`, CVE inventados tipo `CVE-2026-XXXX`): sustituido por un
  catálogo honesto de 14 entradas verificables (10 módulos inline del bridge +
  4 grupos de handlers + comandos nativos Go); los stubs se etiquetan como
  tales
- Comandos `deploy`, `modules`, `victims` y `c2 listen` (cobra): "desplegaban"
  módulos fantasma añadiendo strings a mapas en memoria y `c2 listen` bindeaba
  `0.0.0.0:8443` sin pasar por la geofence
- `internal/appstate/deployment.go` (`DeploymentManager`/`VictimProfile`):
  teatro de despliegue sin efecto real
- TUI BubbleTea (`tui.go`): código muerto nunca alcanzable con datos
  hardcodeados (IPs 172.20.0.x, teclas [U]/[D]/[S] sin handler); elimina las
  dependencias `bubbletea` y `lipgloss` de `go.mod`
- Mapeo táctica→módulo del dispatcher que recomendaba payloads imaginarios:
  ahora solo mapea a módulos reales (`recon`, `cred_dump/dump`) y reporta
  honestamente "no module mapped" para el resto; test de regresión evita que
  el catálogo ficticio vuelva
- 4 paquetes huérfanos en `plugins/` (hivemind, autofactory, vishing,
  rf_contagion) sin ningún importador

### Corregido
- `vesper version` mostraba solo `"Vesper v1.0.0"` plano: el fast-path de
  `main.go` interceptaba el subcomando y el display completo con banner y
  tabla BUILD INFO era inalcanzable; ahora `--version`/`-v` responden al
  instante sin tocar disco y `vesper version` muestra el display completo
  (sin avisos de postura, que no aplican a una consulta de identidad)
- `show options` de la consola mostraba placeholders msf-style (RHOSTS,
  LPORT=4444…) que ningún módulo leía; el option set canónico es `TARGET` (con
  alias) y todo lo que el operador fija con `set` se pasa al módulo tal cual
- `run` en consola despachaba todo por `exploit/run` (handler inexistente);
  ahora resuelve `grupo/función` contra el registry del bridge y los módulos
  inline por su nombre
- Duplicidad de ruta `/api/campaigns` tras la limpieza de phantom (ServeMux
  habría hecho panic al arrancar)
- `SetupAuth()` sobrescribía el handler final dejando el **rate limiter fuera de
  la cadena** (código muerto en runtime): ahora la cadena real es
  CORS → rate limit → (JWT) → mux en ambos modos
- Eliminados los comandos fantasma de la remodelación: `ransomware`,
  `propagate` y `deploy` en la consola y `lateral scan|propagate` en cobra —
  llamaban a handlers del bridge que ya no existen y su salida era teatral
- `payload Generate` y `module push` ahora devuelven 400 ante JSON malformado
  (antes ignoraban el error de decode)
- Loader del bridge: ya no intenta cargar 8 módulos de handlers eliminados
  (`ransomware*`); el health report renombra `ransomware_handlers` →
  `handler_groups`

### Calidad
- golangci-lint: de **98 hallazgos a 0** — mejor-effort explícito (`_ =`) en
  limpiezas best-effort, errores de killchain registrados en auto-mode,
  eliminado código muerto (23 símbolos), deprecaciones `lipgloss.Style.Copy`
  resueltas, `grpc.DialContext` conservado con justificación documentada

## [1.0.0] — 2026-09-10 — "Vesper"

Primera versión bajo el nombre **Vesper**. Esta es una **remodelación
completa** del proyecto anterior (X404X): el árbol se reconstruyó alrededor
de lo que compila, se testea y se documenta de verdad.

### Añadido
- `cmd/vesper/safety.go` + `modules/bridge/safety.py`: postura solo-laboratorio
  real — sandbox `VESPER_LAB_ROOT` para toda E/S del bridge, puerta de
  autorización (`--yes-i-am-authorized` / `VESPER_AUTHORIZED=1`), kill switch
  en runtime (`VESPER_KILLSWITCH=1`, `kill_switch EMERGENCY_STOP`), protección
  de bind (fuerza 127.0.0.1 sin autorización)
- Endpoint `/ws/terminal` funcional: el terminal xterm.js del dashboard ejecuta
  comandos de la consola real vía WebSocket
- `modules/bridge/tests/test_safety.py`: contrato de seguridad del sandbox
- `register_routes()` uniforme en los 5 módulos de handlers restantes
- README bilingüe (EN/ES) con GIFs de demo reales

### Cambiado
- **21 `go.mod` + `go.work` → 1 módulo raíz**: `go build ./...` funciona desde
  la raíz por primera vez en la historia del proyecto
- Protobufs regenerados (protoc 27.3 / protoc-gen-go 1.36): paquete
  `vesper.v1`, `go_package` consistente con el módulo (antes `core/proto/gen`,
  una ruta que nunca existió)
- `cmd/implant`: eliminados los modos de payload destructivos; solo beacon
  con jitter y backoff
- `internal/c2server`: corregido el drift proto (`DecisionUpdate` no tiene
  `approved`; ahora: auto-aprobado salvo `requires_approval`)
- UI del CLI reconstruida (`cmd/vesper/ui.go`): paleta ANSI completa, tablas,
  paneles, banners — el fichero original se perdió en un cleanup con regex
- `requirements.txt` recortado a lo que el bridge importa de verdad;
  extras movidos a `requirements-optional.txt`
- Documentación reescrita desde la realidad medida (handlers, rutas, tests)

### Eliminado (lo que no se podía defender)
- `internal/ransomware` completo (132 ficheros, ~34.600 LOC, 59% del Go del
  repo; 4 sub-paquetes no compilaban) y sus 8 módulos-wrappers en el agente
- 8 handlers Python de "ransomware" — dos de ellos **escribían en ficheros
  reales del árbol** al correr los tests (backdoor plantado en fuentes,
  notas de rescate sobre READMEs)
- `mobile/` (38 LOC abandonadas con C2 hardcodeado caído), `internal/fusion`,
  `internal/bridge` (stub muerto), `pkg/shared/database` (Python huérfano)
- 8 directorios de plugins vacíos (submódulos nunca inicializados) y `.gitmodules`
- `ROADMAP.md` ficción, CHANGELOG no cronológico con capacidades inexistentes,
  `test/security/run_evasion.sh` (solo imprimía nombres de funciones)

### Corregido
- CLI no compilaba (símbolos `cSuccess/cPrimary/ansiR/printErr` inexistentes)
- Secuencias ANSI truncadas en `install.sh` y runners de `test/`
- Tests que ensuciaban el repositorio → sandbox temporal en toda la suite Python;
  el CI falla si algún test muta un fichero trackeado
- CI teatral (jobs contra rutas inexistentes con `|| true`) → CI real: compila
  ambos binarios, corre todas las suites y verifica que el repo queda limpio

## [0.x] — Proyecto anterior (X404X)

Historial arqueológico disponible en `git log` (repositorio original
`Ruby570bocadito/X404X`). Los números de versión 2.x/3.x de aquel changelog
no se reconocen como releases de Vesper.
