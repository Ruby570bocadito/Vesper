# Changelog

Todas las fechas son ISO-8601. Formato basado en [Keep a Changelog](https://keepachangelog.com/es-ES/1.1.0/).

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
