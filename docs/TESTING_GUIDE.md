# Vesper — Guía Completa de Testing

## Requisitos previos

```bash
# Verificar Go (necesario para compilar)
go version  # debe ser >= 1.25

# Verificar Python (bridge)
python3 --version  # debe ser >= 3.11

# Verificar binario
ls -la vesper  # debe ser ~26MB
```

---

## 1. Test rápido (30 segundos)

```bash
# Desde la raíz del proyecto
cd tests && bash integration_test.sh
# Esperado: 7/7 PASS
```

---

## 2. Test del binario individual

```bash
# Ayuda
./vesper --help

# Listar módulos (deben ser 162)
./vesper modules categories
./vesper modules list

# Simular despliegue

# Ver víctimas
./vesper victims list
```

---

## 3. Test con Python Bridge (requiere bridge.py)

```bash
# Terminal 1: Arrancar el bridge
cd modules/bridge
python3 bridge.py --host 127.0.0.1 --port 9100
# Esperado: "Bridge listening on 127.0.0.1:9100"

# Terminal 2: Arrancar el C2 con dashboard
cd ../..
./vesper dashboard --port 9090
# Esperado: "Dashboard running on http://localhost:9090"
# Abrir http://localhost:9090 en navegador
```

---

## 4. Test del ciclo completo kill chain

```bash
# Modo automático (el orquestador decide solo)
./vesper campaign start --name "Test-Campaign" --target "10.0.0.0/24" --auto

# Ver progreso (en otra terminal o vía API)
curl http://localhost:9090/api/campaigns
curl http://localhost:9090/api/agents
curl http://localhost:9090/api/decisions

# Esperado:
# - Decisiones generadas cada 10s por AutoMode
# - Fases avanzando: recon → weaponization → delivery → exploitation → ...
# - Dispatcher envía módulos a agentes conectados
# - WorldGraph se actualiza con datos reales
```

---

## 5. Test de módulos individuales vía API

```bash
# Listar todos los 162 módulos
curl http://localhost:9090/api/modules | python3 -m json.tool | head -40

# Desplegar un módulo a un agente
curl -X POST http://localhost:9090/api/modules/push \
  -H "Content-Type: application/json" \

# Ver sesiones activas
curl http://localhost:9090/api/sessions
```

---

## 6. Test de handlers Python individuales

```bash
cd modules/bridge/handlers

python3 -c "
print('Scanned:', r.get('total_scanned'), 'Sensitive:', len(r.get('sensitive_files',[])))
"

# Test v2.10
python3 -c "
"

# Test Block Z
python3 -c "
"
```

---

## 7. Test con Docker lab (requiere Docker)

```bash
# Si tienes Docker instalado:
cd core/c2
docker-compose up -d
# Esperado: 4 contenedores (c2-server, agent-linux-1, agent-linux-2, test-runner)

# Ver logs
docker-compose logs -f c2-server

# Ejecutar tests dentro del contenedor
docker exec -it vesper-test-runner python3 /tests/integration_test.py

# Parar todo
docker-compose down
```

---

## 8. Test de compilación ofuscada

```bash
bash scripts/build_obfuscated.sh
# Esperado: vesper_obfuscated creado (~26MB)
ls -la vesper_obfuscated
```

---

## 9. Test de firma digital

```bash
# Firmar el binario
bash scripts/sign_binary.sh vesper
# Esperado: "Signature VERIFIED ✓"
ls -la vesper.sig
```

---

## 10. Test end-to-end completo (manual)

```bash
# Paso 1: Compilar
go build ./cmd/vesper/

# Paso 2: Arrancar bridge
cd modules/bridge && python3 bridge.py --host 127.0.0.1 --port 9100 &

# Paso 3: Arrancar dashboard
cd ../.. && ./vesper dashboard --port 9090 &

# Paso 4: Verificar salud
sleep 2
curl http://localhost:9090/api/health
# Esperado: {"status":"ok"}

# Paso 5: Iniciar campaña automática
./vesper campaign start --name "E2E-Test" --auto

# Paso 6: Esperar 30 segundos y verificar progreso
sleep 30
curl http://localhost:9090/api/campaigns | python3 -m json.tool

# Paso 7: Ver decisiones generadas
curl http://localhost:9090/api/decisions | python3 -m json.tool | head -30

# Paso 8: Ver módulos ejecutados
curl http://localhost:9090/api/modules | python3 -c "import sys,json; mods=json.load(sys.stdin); print(f'{len(mods)} modules available')"

# Paso 9: Test de despliegue

# Paso 10: Matar procesos
kill %1 %2
```

---

## Troubleshooting

| Error | Solución |
|-------|----------|
| `go: command not found` | Instalar Go 1.25+ |
| `bridge.py: connection refused` | Verificar que el bridge está corriendo en 127.0.0.1:9100 |
| `AppState not initialized` | Ejecutar `vesper dashboard` o `vesper tui` primero |
| `no agents available` | Normal en modo test — el Dispatcher usa fallback |
| `binary not found` en tests | Ejecutar tests desde `tests/` con `cd tests && bash integration_test.sh` |
| Docker no arranca | Sin sudo en WSL2. Usar tests sin Docker (pasos 1-9) |
