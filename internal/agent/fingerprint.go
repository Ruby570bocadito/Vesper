package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type FingerprintScanner struct {
	KnownVulnerable []VulnFingerprint
	Matches         []FingerprintMatch
}

type VulnFingerprint struct {
	Name string
	Path string
	Func string
	Hash string
	CVE  string
}

type FingerprintMatch struct {
	Binary   string
	Function string
	Match    string
	CVE      string
	Conf     float64
}

var knownVulnHashes = []VulnFingerprint{
	{Name: "sudo", Path: "/usr/bin/sudo", Func: "main", Hash: "e3b0c442", CVE: "CVE-2021-3156"},
	{Name: "openssl", Path: "/usr/lib/libssl.so.3", Func: "SSL_read", Hash: "a1b2c3d4", CVE: "CVE-2022-3786"},
	{Name: "bash", Path: "/bin/bash", Func: "shellshock", Hash: "deadbeef", CVE: "CVE-2014-6271"},
	{Name: "apache", Path: "/usr/sbin/httpd", Func: "ap_process_request", Hash: "cafebabe", CVE: "CVE-2021-41773"},
	{Name: "log4j", Path: "/opt/*/log4j-core*.jar", Func: "JndiLookup", Hash: "log4shell", CVE: "CVE-2021-44228"},
}

func NewFingerprintScanner() *FingerprintScanner {
	return &FingerprintScanner{KnownVulnerable: knownVulnHashes}
}

func (fs *FingerprintScanner) Scan() []FingerprintMatch {
	var matches []FingerprintMatch

	for _, vf := range fs.KnownVulnerable {
		if !fs.fileExists(vf.Path) {
			continue
		}

		hash := fs.hashFile(vf.Path)
		if hash == "" {
			continue
		}

		conf := fs.fuzzyCompare(hash, vf.Hash)

		if conf > 0.5 {
			matches = append(matches, FingerprintMatch{
				Binary: vf.Path, Function: vf.Func, Match: hash[:16], CVE: vf.CVE, Conf: conf,
			})
		}
	}

	fs.scanWithSSDEEP()

	fs.Matches = matches
	return matches
}

func (fs *FingerprintScanner) fileExists(path string) bool {
	matches, _ := filepath.Glob(path)
	return len(matches) > 0
}

func (fs *FingerprintScanner) hashFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil || len(data) > 50*1024*1024 {
		return ""
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func (fs *FingerprintScanner) fuzzyCompare(hash1, hash2 string) float64 {
	minLen := len(hash1)
	if len(hash2) < minLen {
		minLen = len(hash2)
	}
	matches := 0
	for i := 0; i < minLen; i++ {
		if hash1[i] == hash2[i] {
			matches++
		}
	}
	return float64(matches) / float64(minLen)
}

func (fs *FingerprintScanner) scanWithSSDEEP() {
	if _, err := exec.LookPath("ssdeep"); err != nil {
		return
	}
	for _, vf := range fs.KnownVulnerable {
		if !fs.fileExists(vf.Path) {
			continue
		}
		out, _ := exec.Command("ssdeep", "-b", vf.Path).Output()
		_ = out
	}
}

func (fs *FingerprintScanner) Report() string {
	var report strings.Builder
	report.WriteString("Vesper FINGERPRINT SCAN REPORT\n")
	fmt.Fprintf(&report, "Matches: %d\n", len(fs.Matches))
	for _, m := range fs.Matches {
		fmt.Fprintf(&report, "  %s [%s] => %s (%.0f%%)\n", m.Binary, m.Function, m.CVE, m.Conf*100)
	}
	return report.String()
}
