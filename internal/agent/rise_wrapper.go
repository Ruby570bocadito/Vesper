// Package agent provides the Rise-Privilege integration.
//
// The RiseWrapper compiles and invokes the Rise-Privilege binary
// as a subprocess, parsing its JSON output for integration with
// the PostExploitPipeline.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ruby570bocadito/vesper/pkg/shared/logger"
)

// RiseWrapper wraps the Rise-Privilege binary for programmatic use.
type RiseWrapper struct {
	log     *logger.Logger
	binPath string
}

// RiseResult is the parsed JSON output from Rise-Privilege.
type RiseResult struct {
	Rooted   bool          `json:"rooted"`
	Findings []RiseFinding `json:"findings"`
	Vectors  []RiseVector  `json:"vectors"`
}

type RiseFinding struct {
	Type   string `json:"type"`
	Detail string `json:"detail"`
	Risk   string `json:"risk"`
}

type RiseVector struct {
	Name     string `json:"name"`
	Risk     string `json:"risk"`
	Command  string `json:"command"`
	Category string `json:"category"`
}

// NewRiseWrapper creates a Rise-Privilege wrapper.
func NewRiseWrapper(log *logger.Logger) *RiseWrapper {
	rw := &RiseWrapper{log: log}

	// Find the Rise-Privilege binary
	searchPaths := []string{
		"core/privesc/Rise-Privilege",
		"core/privesc/rise-privilege",
		"/opt/vesper/plugins/privesc/Rise-Privilege",
		"Rise-Privilege",
		"rise-privilege",
	}
	for _, p := range searchPaths {
		abs, _ := filepath.Abs(p)
		if _, err := os.Stat(abs); err == nil {
			rw.binPath = abs
			break
		}
	}

	// Try to compile if not found
	if rw.binPath == "" {
		rw.tryCompile()
	}

	return rw
}

func (rw *RiseWrapper) tryCompile() {
	privescDir := "core/privesc"
	if _, err := os.Stat(privescDir); err != nil {
		return
	}

	rw.log.Info("compiling Rise-Privilege...")
	cmd := exec.Command("go", "build", "-o", "Rise-Privilege", ".")
	cmd.Dir = privescDir
	if output, err := cmd.CombinedOutput(); err != nil {
		rw.log.Warnf("Rise-Privilege compile failed: %v\n%s", err, string(output))
		return
	}

	rw.binPath = filepath.Join(privescDir, "Rise-Privilege")
	rw.log.Infof("Rise-Privilege compiled: %s", rw.binPath)
}

// IsAvailable returns whether the Rise-Privilege binary is available.
func (rw *RiseWrapper) IsAvailable() bool {
	return rw.binPath != ""
}

// Scan runs a scan-only operation (safe, no exploitation).
func (rw *RiseWrapper) Scan(ctx context.Context, vector string) (*RiseResult, error) {
	return rw.run(ctx, "--json", "--vector", vector)
}

// Exploit runs a full scan + exploit operation.
func (rw *RiseWrapper) Exploit(ctx context.Context, risk string) (*RiseResult, error) {
	args := []string{"--exploit", "--json", "--one-shot"}
	if risk != "" {
		args = append(args, "--risk", risk)
	}
	return rw.run(ctx, args...)
}

// FullChain runs all phases and attempts auto-root.
func (rw *RiseWrapper) FullChain(ctx context.Context, lhost, lport string) (*RiseResult, error) {
	args := []string{"--exploit", "--json", "--one-shot"}
	if lhost != "" {
		args = append(args, "--lhost", lhost)
	}
	if lport != "" {
		args = append(args, "--lport", lport)
	}
	return rw.run(ctx, args...)
}

func (rw *RiseWrapper) run(ctx context.Context, args ...string) (*RiseResult, error) {
	if !rw.IsAvailable() {
		return nil, fmt.Errorf("Rise-Privilege binary not found")
	}

	rw.log.Infof("running Rise-Privilege: %s %v", rw.binPath, args)

	cmd := exec.CommandContext(ctx, rw.binPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		rw.log.Warnf("Rise-Privilege error: %v\n%s", err, string(output))
	}

	var result RiseResult
	if err := json.Unmarshal(output, &result); err != nil {
		rw.log.Warnf("failed to parse Rise-Privilege output: %v", err)
		return &result, nil
	}

	if result.Rooted {
		rw.log.Infof("Rise-Privilege: root obtained!")
	}

	return &result, nil
}

// BinPath returns the path to the Rise-Privilege binary.
func (rw *RiseWrapper) BinPath() string {
	return rw.binPath
}
