package crypto

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"sync"
)

type ProtoLoader struct {
	xorKey     []byte
	obfuscated map[string]*ObfuscatedProto
	mu         sync.RWMutex
	loaded     map[string]bool
}

type ObfuscatedProto struct {
	Name         string
	EncodedData  string
	OriginalSize int
	SHA256       string
	XORKeyFrag   string
	Compression  bool
}

func NewProtoLoader(masterKey []byte) *ProtoLoader {
	return &ProtoLoader{
		xorKey:     masterKey,
		obfuscated: make(map[string]*ObfuscatedProto),
		loaded:     make(map[string]bool),
	}
}

func (pl *ProtoLoader) XorEncode(data []byte, key []byte) []byte {
	encoded := make([]byte, len(data))
	for i := range data {
		encoded[i] = data[i] ^ key[i%len(key)]
	}
	return encoded
}

func (pl *ProtoLoader) XorDecode(encoded []byte, key []byte) []byte {
	return pl.XorEncode(encoded, key)
}

func (pl *ProtoLoader) ObfuscateProto(name string, protoData []byte) (*ObfuscatedProto, error) {
	chunkKey := make([]byte, 32)
	rand.Read(chunkKey)

	hash := sha256.Sum256(protoData)
	hashHex := fmt.Sprintf("%x", hash[:])

	xored := pl.XorEncode(protoData, chunkKey)

	var finalData []byte
	compressed := false

	if len(xored) > 1024 {
		var buf bytes.Buffer
		w := gzip.NewWriter(&buf)
		if _, err := w.Write(xored); err == nil {
			w.Close()
			if buf.Len() < len(xored) {
				finalData = buf.Bytes()
				compressed = true
			}
		}
	}

	if finalData == nil {
		finalData = xored
	}

	ivEncoded := make([]byte, 16)
	rand.Read(ivEncoded)

	block, err := aes.NewCipher(pl.xorKey[:32])
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}

	stream := cipher.NewCTR(block, ivEncoded)
	stream.XORKeyStream(finalData, finalData)

	resultData := make([]byte, 16+len(finalData))
	copy(resultData[:16], ivEncoded)
	copy(resultData[16:], finalData)

	b64 := base64.StdEncoding.EncodeToString(resultData)

	keyFrag := fmt.Sprintf("%x", chunkKey[:16])

	obf := &ObfuscatedProto{
		Name:         name,
		EncodedData:  b64,
		OriginalSize: len(protoData),
		SHA256:       hashHex,
		XORKeyFrag:   keyFrag,
		Compression:  compressed,
	}

	pl.mu.Lock()
	pl.obfuscated[name] = obf
	pl.mu.Unlock()

	return obf, nil
}

func (pl *ProtoLoader) DeobfuscateProto(obf *ObfuscatedProto) ([]byte, error) {
	combined, err := base64.StdEncoding.DecodeString(obf.EncodedData)
	if err != nil {
		return nil, fmt.Errorf("b64 decode: %w", err)
	}

	if len(combined) < 16 {
		return nil, fmt.Errorf("encoded data too short")
	}

	iv := combined[:16]
	encrypted := combined[16:]

	block, err := aes.NewCipher(pl.xorKey[:32])
	if err != nil {
		return nil, err
	}

	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(encrypted, encrypted)

	var xored []byte
	if obf.Compression {
		gzReader, err := gzip.NewReader(bytes.NewReader(encrypted))
		if err != nil {
			return nil, fmt.Errorf("gzip decompress: %w", err)
		}
		defer gzReader.Close()

		xored, err = io.ReadAll(gzReader)
		if err != nil {
			return nil, fmt.Errorf("gzip read: %w", err)
		}
	} else {
		xored = encrypted
	}

	chunkKey, err := hexToBytes(obf.XORKeyFrag + obf.XORKeyFrag)
	if err != nil {
		return nil, err
	}

	decoded := pl.XorDecode(xored, chunkKey)

	hash := sha256.Sum256(decoded)
	if fmt.Sprintf("%x", hash[:]) != obf.SHA256 {
		return nil, fmt.Errorf("integrity check failed: proto hash mismatch")
	}

	return decoded, nil
}

func (pl *ProtoLoader) LoadProto(name string) ([]byte, error) {
	pl.mu.RLock()
	obf, exists := pl.obfuscated[name]
	pl.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("obfuscated proto not found: %s", name)
	}

	return pl.DeobfuscateProto(obf)
}

func (pl *ProtoLoader) RegisterProto(name string, obf *ObfuscatedProto) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	pl.obfuscated[name] = obf
	pl.loaded[name] = false
}

func (pl *ProtoLoader) EmbedProtoInMemory(name string) error {
	data, err := pl.LoadProto(name)
	if err != nil {
		return err
	}

	pl.mu.Lock()
	pl.loaded[name] = true
	pl.mu.Unlock()

	_ = data

	return nil
}

func (pl *ProtoLoader) MaskProtoStrings(data []byte, stringsToMask []string) []byte {
	masked := make([]byte, len(data))
	copy(masked, data)

	for _, s := range stringsToMask {
		pattern := []byte(s)
		mask := make([]byte, len(pattern))
		rand.Read(mask)

		for i := 0; i <= len(masked)-len(pattern); i++ {
			if bytes.Equal(masked[i:i+len(pattern)], pattern) {
				copy(masked[i:], mask)
			}
		}
	}

	return masked
}

func (pl *ProtoLoader) GenerateEmbeddedLoader(protoNames []string) string {
	loaderCode := `package main

import (
	"encoding/base64"
	"fmt"
	"os"
)

type protoEntry struct {
	Name string
	Data string
}

var protoStore = map[string]protoEntry{
`

	for _, name := range protoNames {
		pl.mu.RLock()
		obf, exists := pl.obfuscated[name]
		pl.mu.RUnlock()

		if exists {
			loaderCode += fmt.Sprintf(
				`	"%s": {Name: "%s", Data: "%s"},"`+"\n",
				name, name, obf.EncodedData[:min(80, len(obf.EncodedData))]+"...",
			)
		}
	}

	loaderCode += `}

func main() {
	for name, entry := range protoStore {
		raw, err := base64.StdEncoding.DecodeString(entry.Data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading %s: %%v", name, err)
			continue
		}
		fmt.Printf("[%s] loaded %%d bytes\\n", name, len(raw))
	}
}
`

	return loaderCode
}

func (pl *ProtoLoader) ExportObfuscated(name string, path string) error {
	pl.mu.RLock()
	obf, exists := pl.obfuscated[name]
	pl.mu.RUnlock()

	if !exists {
		return fmt.Errorf("proto not found: %s", name)
	}

	data := fmt.Sprintf(`// Obfuscated Proto Definition: %s
// This file was auto-generated by the Proto Obfuscation Loader.
// Do not edit manually.
package proto

const %s_Obfuscated = "%s"
const %s_SHA256 = "%s"
const %s_KeyFrag = "%s"
const %s_OriginalSize = %d
const %s_Compression = %v
`,
		name,
		name, obf.EncodedData,
		name, obf.SHA256,
		name, obf.XORKeyFrag,
		name, obf.OriginalSize,
		name, obf.Compression,
	)

	return os.WriteFile(path, []byte(data), 0644)
}

func (pl *ProtoLoader) FullProtoObfuscationSuite(protoName string, protoData []byte) map[string]interface{} {
	result := make(map[string]interface{})

	obf, err := pl.ObfuscateProto(protoName, protoData)
	if err != nil {
		result["error"] = err.Error()
		return result
	}

	result["name"] = obf.Name
	result["original_size"] = obf.OriginalSize
	result["compressed"] = obf.Compression
	result["encoded_size"] = len(obf.EncodedData)
	result["reduction_pct"] = fmt.Sprintf("%.1f%%", float64(len(obf.EncodedData))/float64(obf.OriginalSize)*100)
	result["sha256"] = obf.SHA256[:16] + "..."

	decoded, err := pl.DeobfuscateProto(obf)
	if err != nil {
		result["deobfuscate_error"] = err.Error()
		return result
	}

	if len(decoded) == obf.OriginalSize {
		result["roundtrip"] = "success"
	} else {
		result["roundtrip"] = fmt.Sprintf("size mismatch: %d != %d", len(decoded), obf.OriginalSize)
	}

	return result
}

func (pl *ProtoLoader) VaporizeBuffers() {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	for k := range pl.obfuscated {
		pl.obfuscated[k].EncodedData = ""
		pl.loaded[k] = false
	}
}

func (pl *ProtoLoader) GetObfuscatedNames() []string {
	pl.mu.RLock()
	defer pl.mu.RUnlock()

	var names []string
	for name := range pl.obfuscated {
		names = append(names, name)
	}
	return names
}

func hexToBytes(hexStr string) ([]byte, error) {
	result := make([]byte, len(hexStr)/2)
	for i := 0; i < len(result); i++ {
		fmt.Sscanf(hexStr[i*2:i*2+2], "%02x", &result[i])
	}
	return result, nil
}

var _ = io.EOF
var _ = bytes.NewBuffer
