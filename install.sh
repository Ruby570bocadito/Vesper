#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════════════════════
# Vesper Interactive Installer v1.0
# Semi-Autonomous Red Team Platform
# ═══════════════════════════════════════════════════════════════════════════════
set -euo pipefail

# ── Colors (match console palette) ────────────────────────────────────────────
R="\033[0m"
B="\033[1m"
D="\033[2m"
I="\033[3m"

cPri="\033[38;5;99m"   # violet
cSuc="\033[38;5;46m"   # neon green
cWrn="\033[38;5;220m"  # gold
cDgr="\033[38;5;196m"  # red
cInf="\033[38;5;39m"   # sky blue
cMtd="\033[38;5;240m"  # dark gray
cWht="\033[38;5;255m"  # white
cCyn="\033[38;5;51m"   # cyan
cOrg="\033[38;5;208m"  # orange

# gradient
g1="\033[38;5;57m"
g2="\033[38;5;63m"
g3="\033[38;5;99m"
g4="\033[38;5;135m"
g5="\033[38;5;171m"
g6="\033[38;5;207m"

LOG="/tmp/vesper-install.log"
: > "$LOG"

# ── Helpers ───────────────────────────────────────────────────────────────────
log() { echo "[$(date '+%H:%M:%S')] $*" >> "$LOG"; }
ok()   { echo -e "  ${cSuc}${B}✓${R} ${cWht}$*${R}"; log "[OK] $*"; }
info() { echo -e "  ${cInf}●${R} $*"; log "[..] $*"; }
warn() { echo -e "  ${cWrn}${B}⚠${R}  ${cWrn}$*${R}"; log "[WW] $*"; }
err()  { echo -e "  ${cDgr}${B}✗${R}  ${cDgr}$*${R}"; log "[EE] $*"; }
div()  { echo -e "  ${cMtd}────────────────────────────────────────────────────${R}"; }
title(){ echo -e "\n  ${cPri}${B}━━  $*  $(printf '━%*s' $((50 - ${#*})) '')${R}\n"; }

# ── Banner (exact console art from ui.go) ─────────────────────────────────────
print_banner() {
  echo
  echo -e "${g1}  ▄▄▄   ▄▄▄  ▄▄   ▄▄   ▄▄▄▄▄▄   ▄▄   ▄▄  ▄▄▄   ▄▄▄ ${R}"
  echo -e "${g2}  ▀██▄ ▄██▀  ██   ██  ██▀  ▀██  ██   ██  ▀██▄ ▄██▀ ${R}"
  echo -e "${g3}    ▀███▀    ███████  ██    ██  ███████    ▀███▀   ${R}"
  echo -e "${g4}  ▄██▀ ▀██▄       ██  ██▄  ▄██       ██  ▄██▀ ▀██▄ ${R}"
  echo -e "${g5}  ██▀   ▀██       ██   ▀████▀        ██  ██▀   ▀██ ${R}"
  echo -e "${g6}  ▀▀     ▀▀       ▀▀                 ▀▀  ▀▀     ▀▀ ${R}"
  echo
  echo -e "  ${cPri}${B}Semi-Autonomous Red Team Platform${R}  ${cMtd}•${R}  ${cMtd}v1.0.0${R}  ${cMtd}•${R}  ${cMtd}Go 1.24${R}"
  echo -e "  ${cMtd}────────────────────────────────────────────────────${R}"
  echo
}

# ── Read input ───────────────────────────────────────────────────────────────
ask_yn() {
  local prompt="$1" default="${2:-y}"
  local yn
  while true; do
    >&2 printf "  ${cCyn}?${R} ${prompt} [${default}] "
    read -r yn
    yn="${yn:-$default}"
    case "$yn" in
      [yY]|[yY][eE][sS]) return 0 ;;
      [nN]|[nN][oO]) return 1 ;;
    esac
  done
}

ask_choice() {
  local prompt="$1" default="${2:-}" choices=()
  shift 2
  for opt; do choices+=("$opt"); done
  >&2 echo -e "  ${cCyn}?${R} ${prompt}"
  for i in "${!choices[@]}"; do
    >&2 echo -e "    $((i+1))) ${choices[$i]}"
  done
  local sel
  >&2 printf "    choose [1-%d]%s " "${#choices[@]}" "${default:+ [$default]}"
  read -r sel
  sel="${sel:-$default}"
  echo "$sel"
}

ask_text() {
  local prompt="$1" default="${2:-}"
  >&2 echo -e "  ${cCyn}?${R} ${prompt}${default:+ [$default]}"
  read -r val
  echo "${val:-$default}"
}

# ── Detect OS ─────────────────────────────────────────────────────────────────
detect_os() {
  title "System Detection"
  local os="" arch="" distro="" pkg_cmd="" pkg_install=""
  case "$(uname -s)" in
    Linux)
      os="linux"
      if grep -qi microsoft /proc/version 2>/dev/null; then
        os="wsl"
        info "Detected: Windows WSL2"
      else
        info "Detected: Linux"
      fi
      if command -v apt-get &>/dev/null; then
        distro="debian"; pkg_cmd="apt-get"; pkg_install="apt-get install -y"
      elif command -v dnf &>/dev/null; then
        distro="fedora"; pkg_cmd="dnf"; pkg_install="dnf install -y"
      elif command -v pacman &>/dev/null; then
        distro="arch"; pkg_cmd="pacman"; pkg_install="pacman -S --noconfirm"
      else
        distro="unknown"; pkg_cmd=""; pkg_install=""
      fi
      ;;
    Darwin)
      os="macos"
      info "Detected: macOS"
      if command -v brew &>/dev/null; then
        pkg_cmd="brew"; pkg_install="brew install"
      fi
      ;;
    *)
      os="unknown"; warn "Unknown OS: $(uname -s)"
      ;;
  esac
  arch="$(uname -m)"
  OS="$os"; ARCH="$arch"; DISTRO="$distro"; PKG_CMD="$pkg_cmd"; PKG_INSTALL="$pkg_install"
  ok "OS: ${os} | Arch: ${arch} | Distro: ${distro:-N/A}"
}

# ── Check prerequisites ───────────────────────────────────────────────────────
check_prereqs() {
  local mode="$1" missing=()
  if [ "$mode" = "native" ]; then
    command -v go &>/dev/null || missing+=("Go 1.22+")
    command -v npm &>/dev/null || missing+=("Node.js / npm")
    command -v python3 &>/dev/null || missing+=("Python3")
  fi
  if [ "$mode" = "docker" ]; then
    command -v docker &>/dev/null || missing+=("Docker")
    docker compose version &>/dev/null 2>&1 || docker-compose --version &>/dev/null 2>&1 || missing+=("Docker Compose")
  fi
  echo "${#missing[@]}"
}

# ── Detect existing installation ──────────────────────────────────────────────
detect_existing() {
  local found=0
  if [ -f "./vesper" ] || [ -f "./dist/vesper" ]; then found=1; fi
  if [ -f "./vesper.db" ]; then found=1; fi
  if systemctl is-enabled vesper &>/dev/null 2>&1; then found=1; fi
  echo "$found"
}

# ── Install system packages ───────────────────────────────────────────────────
install_sys_deps() {
  local mode="$1"
  title "Installing System Dependencies"
  if [ -z "$PKG_CMD" ]; then
    warn "No package manager found. Install dependencies manually."
    return
  fi

  local pkgs=()
  if [ "$mode" = "native" ]; then
    case "$DISTRO" in
      debian) pkgs=(git build-essential python3 python3-pip python3-venv nodejs npm) ;;
      fedora) pkgs=(git gcc python3 python3-pip python3-venv nodejs npm) ;;
      arch)   pkgs=(git base-devel python python-pip python-virtualenv nodejs npm) ;;
    esac
  fi
  if [ "$mode" = "docker" ]; then
    case "$DISTRO" in
      debian) pkgs=(git curl) ;;
      fedora) pkgs=(git curl) ;;
      arch)   pkgs=(git curl) ;;
    esac
  fi

  if [ ${#pkgs[@]} -gt 0 ]; then
    info "Installing: ${pkgs[*]}"
    if [ "$PKG_CMD" = "brew" ]; then
      for pkg in "${pkgs[@]}"; do $PKG_INSTALL "$pkg" 2>&1 | tee -a "$LOG" &>/dev/null; done
    else
      sudo $PKG_CMD update 2>&1 | tee -a "$LOG" &>/dev/null || true
      sudo $PKG_INSTALL "${pkgs[@]}" 2>&1 | tee -a "$LOG"
    fi
    ok "System packages installed"
  fi
}

# ── Install Go (if missing) ───────────────────────────────────────────────────
ensure_go() {
  if command -v go &>/dev/null; then
    ok "Go $(go version | awk '{print $3}')"
    return
  fi
  warn "Go not found. Installing..."
  local ver="1.24.0"
  local go_tar="go${ver}.linux-amd64.tar.gz"
  curl -fsSL "https://go.dev/dl/${go_tar}" -o "/tmp/${go_tar}" 2>&1 | tee -a "$LOG"
  sudo tar -C /usr/local -xzf "/tmp/${go_tar}" 2>&1 | tee -a "$LOG"
  export PATH="/usr/local/go/bin:$PATH"
  if ! grep -q '/usr/local/go/bin' ~/.bashrc 2>/dev/null; then
    echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
  fi
  ok "Go ${ver} installed"
}

# ── Install Node (if missing) ──────────────────────────────────────────────────
ensure_node() {
  if command -v npm &>/dev/null; then
    ok "Node.js $(node -v 2>/dev/null || echo '?')"
    return
  fi
  warn "Node.js not found. Installing..."
  if command -v curl &>/dev/null; then
    curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash - 2>&1 | tee -a "$LOG"
    sudo apt-get install -y nodejs 2>&1 | tee -a "$LOG"
    ok "Node.js installed"
  fi
}

# ── Build native ──────────────────────────────────────────────────────────────
build_native() {
  title "Building Vesper (Native)"

  # Go backend
  info "Building Go binary..."
  go build -o dist/vesper ./cmd/vesper/ 2>&1 | tee -a "$LOG"
  ok "Backend built → dist/vesper"

  # Frontend
  if [ -d "web" ]; then
    info "Building Vue frontend..."
    cd web
    npm install 2>&1 | tee -a "$LOG"
    npm run build 2>&1 | tee -a "$LOG"
    cd ..
    ok "Frontend built → web/dist/"
  fi

  # Python venv
  if [ -f "requirements.txt" ]; then
    info "Setting up Python venv..."
    python3 -m venv venv 2>&1 | tee -a "$LOG"
    source venv/bin/activate
    pip install -r requirements.txt 2>&1 | tee -a "$LOG"
    deactivate
    ok "Python dependencies installed"
  fi
}

# ── Docker install ────────────────────────────────────────────────────────────
install_docker_mode() {
  title "Deploying with Docker"
  if [ -f "lab/docker-compose.yml" ]; then
    docker compose -f lab/docker-compose.yml up -d 2>&1 | tee -a "$LOG"
    ok "Docker lab started"
  else
    warn "lab/docker-compose.yml not found. Starting basic services..."
    if [ -f "docker-compose.yml" ]; then
      docker compose up -d 2>&1 | tee -a "$LOG"
      ok "Docker stack started"
    fi
  fi
}

# ── Install Ollama ────────────────────────────────────────────────────────────
install_ollama_fn() {
  if command -v ollama &>/dev/null; then
    ok "Ollama already installed"
  else
    info "Installing Ollama..."
    curl -fsSL https://ollama.com/install.sh | sh 2>&1 | tee -a "$LOG"
    ok "Ollama installed"
  fi

  # Start if not running
  if ! pgrep -x ollama &>/dev/null; then
    info "Starting Ollama daemon..."
    nohup ollama serve &>/dev/null &
    sleep 2
  fi

  # Model selection
  echo -e "  ${cCyn}?${R} Select Ollama model to pull:"
  local models=(
    "llama3.2     (7B, recommended)"
    "llama3.1     (8B)"
    "mistral      (7B)"
    "phi3         (3.8B, lightweight)"
    "codellama    (7B, code)"
    "qwen2.5      (7B)"
    "mixtral      (8x7B, heavy)"
    "gemma2       (9B)"
    "Other (type manually)"
  )
  for i in "${!models[@]}"; do
    echo -e "    $((i+1))) ${models[$i]}"
  done
  local mod_names=("llama3.2" "llama3.1" "mistral" "phi3" "codellama" "qwen2.5" "mixtral" "gemma2")
  local sel_model
  read -r -p "$(echo -e "    ${cMtd}choose [1-9]${R} [1] ")" sel_choice
  sel_choice="${sel_choice:-1}"
  if [ "$sel_choice" -ge 1 ] && [ "$sel_choice" -le 8 ] 2>/dev/null; then
    sel_model="${mod_names[$((sel_choice-1))]}"
  else
    read -r -p "$(echo -e "    ${cCyn}?${R} Model name: ")" sel_model
    sel_model="${sel_model:-llama3.2}"
  fi

  info "Pulling ${sel_model} (this may take a while)..."
  ollama pull "$sel_model" 2>&1 | tee -a "$LOG"
  ok "Model ${sel_model} ready"
  OLLAMA_MODEL="$sel_model"
}

# ── Plugins ───────────────────────────────────────────────────────────────────
install_plugins_fn() {
  title "Plugin Installation"
  local plugins=()
  ask_yn "Install ${cCyn}AI Specter${R} plugin?" && plugins+=("ai/specter")
  ask_yn "Install ${cCyn}Pulse C2${R} plugin?" && plugins+=("pulse-c2")
  ask_yn "Install ${cCyn}Blue Team${R} plugin?" && plugins+=("blue")
  ask_yn "Install ${cCyn}Privesc${R} plugin?" && plugins+=("privesc")
  ask_yn "Install ${cCyn}Kernel${R} plugin?" && plugins+=("kernel")
  ask_yn "Install ${cCyn}Worm${R} plugin?" && plugins+=("worm")

  if [ ${#plugins[@]} -eq 0 ]; then
    info "No plugins selected"
    return
  fi

  for p in "${plugins[@]}"; do
    local dir="plugins/$p"
    if [ -d "$dir" ]; then
      info "Building plugin: $p"
      if [ -f "$dir/Makefile" ]; then
        (cd "$dir" && make build 2>&1 | tee -a "$LOG") || warn "Plugin $p build failed"
      elif [ -f "$dir/setup.sh" ]; then
        (cd "$dir" && bash setup.sh 2>&1 | tee -a "$LOG") || warn "Plugin $p setup failed"
      elif [ -f "$dir/install.sh" ]; then
        (cd "$dir" && bash install.sh 2>&1 | tee -a "$LOG") || warn "Plugin $p install failed"
      else
        warn "No build script for $p, skipping"
      fi
      ok "Plugin $p processed"
    else
      warn "Plugin directory $dir not found"
    fi
  done
}

# ── Systemd service ───────────────────────────────────────────────────────────
setup_service() {
  title "Autostart Service"
  if ! ask_yn "Configure Vesper to start automatically on boot?"; then
    return
  fi

  local bin_path
  bin_path="$(cd "$(dirname "$0")" && pwd)/dist/vesper"
  if [ ! -f "$bin_path" ]; then
    bin_path="$(cd "$(dirname "$0")" && pwd)/vesper"
  fi
  if [ ! -f "$bin_path" ]; then
    warn "Binary not found, skipping service setup"
    return
  fi

  case "$OS" in
    linux|wsl)
      local svc="/etc/systemd/system/vesper.service"
      info "Creating systemd unit: $svc"
      sudo tee "$svc" >/dev/null <<UNIT
[Unit]
Description=Vesper Red Team Platform
After=network.target

[Service]
Type=simple
ExecStart=${bin_path} dashboard
Restart=on-failure
RestartSec=5
WorkingDirectory=$(dirname "$bin_path")
StandardOutput=journal
StandardError=journal
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
UNIT
      sudo systemctl daemon-reload 2>&1 | tee -a "$LOG"
      sudo systemctl enable vesper 2>&1 | tee -a "$LOG"
      if ask_yn "Start service now?"; then
        sudo systemctl start vesper 2>&1 | tee -a "$LOG"
      fi
      ok "Systemd service configured (vesper)"
      SERVICE_TYPE="systemd"
      ;;
    macos)
      local plist="$HOME/Library/LaunchAgents/com.vesper.plist"
      info "Creating launchd plist: $plist"
      mkdir -p "$HOME/Library/LaunchAgents"
      cat > "$plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.vesper</string>
    <key>ProgramArguments</key>
    <array>
        <string>${bin_path}</string>
        <string>dashboard</string>
    </array>
    <key>WorkingDirectory</key>
    <string>$(dirname "$bin_path")</string>
    <key>KeepAlive</key>
    <true/>
    <key>RunAtLoad</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/vesper-stdout.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/vesper-stderr.log</string>
</dict>
</plist>
PLIST
      launchctl load "$plist" 2>&1 | tee -a "$LOG"
      ok "LaunchAgent configured (com.vesper)"
      SERVICE_TYPE="launchd"
      ;;
  esac
}

# ── PATH setup ─────────────────────────────────────────────────────────────────
setup_path() {
  title "Global Access (PATH)"
  if ! ask_yn "Add vesper to PATH so you can run it from anywhere?"; then
    return
  fi

  local bin_path="$(pwd)/vesper"
  if [ ! -f "$bin_path" ]; then
    bin_path="$(pwd)/dist/vesper"
  fi
  if [ ! -f "$bin_path" ]; then
    warn "Binary not found, skipping PATH setup"
    return
  fi

  # Try symlink to /usr/local/bin first
  if [ -d "/usr/local/bin" ] && [ -w "/usr/local/bin" ] 2>/dev/null; then
    ln -sf "$bin_path" /usr/local/bin/vesper
    ok "Symlink created: /usr/local/bin/vesper"
    return
  fi

  # Fallback: add to .bashrc / .zshrc
  local rc_file=""
  if [ -f "$HOME/.zshrc" ]; then
    rc_file="$HOME/.zshrc"
  elif [ -f "$HOME/.bashrc" ]; then
    rc_file="$HOME/.bashrc"
  elif [ -f "$HOME/.bash_profile" ]; then
    rc_file="$HOME/.bash_profile"
  fi

  if [ -n "$rc_file" ]; then
    local dir="$(dirname "$bin_path")"
    if ! grep -q "vesper" "$rc_file" 2>/dev/null; then
      echo "" >> "$rc_file"
      echo "# Vesper" >> "$rc_file"
      echo "export PATH=\"\$PATH:$dir\"" >> "$rc_file"
      ok "Added to PATH in ${rc_file}"
      info "Run: source ${rc_file}  to apply"
    else
      ok "PATH already configured in ${rc_file}"
    fi
  else
    warn "No shell rc file found. Add manually: export PATH=\"\$PATH:$(dirname "$bin_path")\""
  fi
}

# ── Summary ────────────────────────────────────────────────────────────────────
print_summary() {
  clear
  print_banner
  echo -e "  ${cSuc}${B}✓  Installation Complete!${R}"
  div
  echo
  echo -e "  ${cWht}📊 Dashboard:${R}     ${cPri}http://localhost:${DASH_PORT:-3000}${R}"
  echo -e "  ${cWht}🔌 API Server:${R}     ${cPri}https://localhost:${SRV_PORT:-8443}${R}"
  echo -e "  ${cWht}🤖 AI Model:${R}       ${cPri}${OLLAMA_MODEL:-llama3.2}${R}${cMtd} (Ollama)${R}"
  echo -e "  ${cWht}📁 Install Dir:${R}    ${cPri}$(pwd)${R}"
  if [ -n "${SERVICE_TYPE:-}" ]; then
    echo -e "  ${cWht}🔄 Autostart:${R}      ${cSuc}${SERVICE_TYPE} active${R}"
  fi
  echo
  div
  echo -e "  ${cMtd}Quick commands:${R}"
  echo -e "    ${cPri}vesper${R}                      ${cMtd}Launch interactive console${R}"
  echo -e "    ${cPri}vesper --console${R}             ${cMtd}Launch console (alternative)${R}"
  echo -e "    ${cPri}vesper --dashboard${R}           ${cMtd}Start API + WebSocket server${R}"
  echo -e "    ${cPri}vesper tui${R}                   ${cMtd}Launch BubbleTea TUI${R}"
  echo -e "    ${cPri}vesper campaign start${R}        ${cMtd}Start a campaign${R}"
  echo -e "    ${cPri}vesper agent --help${R}          ${cMtd}Agent deployment help${R}"
  echo -e "    ${cPri}vesper --help${R}                ${cMtd}Full CLI reference${R}"
  if [ "${SERVICE_TYPE:-}" = "systemd" ]; then
    echo -e "    ${cPri}sudo systemctl status vesper${R}  ${cMtd}Service status${R}"
    echo -e "    ${cPri}sudo journalctl -u vesper -f${R}  ${cMtd}Live service logs${R}"
  fi
  echo
  div
  echo -e "  ${cMtd}Log: ${LOG}${R}"
  echo
}

# ═══════════════════════════════════════════════════════════════════════════════
# MAIN
# ═══════════════════════════════════════════════════════════════════════════════
main() {
  # Vars
  OS=""; ARCH=""; DISTRO=""; PKG_CMD=""; PKG_INSTALL=""
  DASH_PORT="3000"; SRV_PORT="8443"
  OLLAMA_MODEL=""
  SERVICE_TYPE=""

  clear
  print_banner

  # ── Disclaimer ────────────────────────────────────────────────────────────
  echo -e "  ${cMtd}${I}Vesper is a security assessment tool. Use only on systems${R}"
  echo -e "  ${cMtd}${I}you own or have explicit permission to test.${R}"
  echo -e "  ${cMtd}${I}Unauthorized use is illegal.${R}"
  echo
  if ! ask_yn "Do you accept these terms and wish to proceed?" "n"; then
    echo -e "\n  ${cDgr}Installation cancelled.${R}\n"
    exit 1
  fi
  echo

  # ── Detect OS ─────────────────────────────────────────────────────────────
  detect_os

  # ── Detect existing installation ──────────────────────────────────────────
  if [ "$(detect_existing)" = "1" ]; then
    title "Existing Installation Detected"
    echo -e "  ${cWrn}Vesper appears to be already installed.${R}"
    local upd_mode
    upd_mode=$(ask_choice "What would you like to do?" "1" \
      "Upgrade existing installation" \
      "Reinstall from scratch" \
      "Cancel")
    case "$upd_mode" in
      1)
        info "Running upgrade..."
        if [ -d ".git" ]; then
          git pull 2>&1 | tee -a "$LOG" || warn "Git pull failed"
        fi
        ensure_go
        ensure_node
        build_native
        print_summary
        exit 0
        ;;
      2) info "Reinstalling from scratch..." ;;
      *) echo -e "\n  ${cDgr}Cancelled.${R}\n"; exit 0 ;;
    esac
  fi

  # ── Installation mode ─────────────────────────────────────────────────────
  title "Installation Mode"
  local mode
  mode=$(ask_choice "Select installation method:" "1" \
    "Docker (recommended — isolated, reproducible)" \
    "Native (direct binaries, more control)" \
    "Cancel")
  case "$mode" in
    1) MODE="docker" ;;
    2) MODE="native" ;;
    *) echo -e "\n  ${cDgr}Cancelled.${R}\n"; exit 0 ;;
  esac

  # ── Prerequisites check ───────────────────────────────────────────────────
  local missing
  missing=$(check_prereqs "$MODE")
  if [ "$missing" -gt 0 ]; then
    warn "$missing prerequisite(s) missing. Will install."
  fi

  # ── Install system deps ──────────────────────────────────────────────────────
  install_sys_deps "$MODE"

  # ── Ensure Go / Node for native ─────────────────────────────────────────────
  if [ "$MODE" = "native" ]; then
    ensure_go
    ensure_node
  fi

  # ── Config ───────────────────────────────────────────────────────────────────
  title "Configuration"
  DASH_PORT=$(ask_text "Dashboard port" "3000")
  SRV_PORT=$(ask_text "API server port" "8443")

  # ── Ollama ───────────────────────────────────────────────────────────────────
  title "AI Engine"
  if ask_yn "Install Ollama for AI capabilities?"; then
    install_ollama_fn
  fi

  # ── Plugins ──────────────────────────────────────────────────────────────────
  install_plugins_fn

  # ── Build / Deploy ──────────────────────────────────────────────────────────
  case "$MODE" in
    native) build_native ;;
    docker) install_docker_mode ;;
  esac

  # ── PATH setup (vesper from anywhere) ──────────────────────────────────────────
  setup_path

  # ── Autostart service ────────────────────────────────────────────────────────
  if [ "$MODE" = "native" ]; then
    setup_service
  fi

  # ── Summary ──────────────────────────────────────────────────────────────────
  print_summary
}

main "$@"
