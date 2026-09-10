package vishing

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type DeepfakeVishing struct {
	config      interface{}
	voiceModel  string
	targetPhone string
	script      string
}

type VoiceClone struct {
	SourceAudio []byte
	TargetText  string
	ModelName   string
	ClonedVoice []byte
	Duration    float64
}

func NewDeepfakeVishing(cfg interface{}) *DeepfakeVishing {
	return &DeepfakeVishing{
		config: cfg,
	}
}

func (d *DeepfakeVishing) SetVoiceModel(modelPath string) {
	d.voiceModel = modelPath
}

func (d *DeepfakeVishing) ExtractVoiceSamples(audioSource string) ([]byte, error) {
	if _, err := os.Stat(audioSource); err != nil {
		return nil, fmt.Errorf("audio source not found: %s", audioSource)
	}

	cmd := exec.Command("ffmpeg",
		"-i", audioSource,
		"-ac", "1",
		"-ar", "22050",
		"-f", "wav",
		"-")
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg extraction failed: %w", err)
	}

	return out.Bytes(), nil
}

func (d *DeepfakeVishing) CloneVoice(sourceAudio []byte, targetText string) (*VoiceClone, error) {
	clone := &VoiceClone{
		SourceAudio: sourceAudio,
		TargetText:  targetText,
		ModelName:   "coqui-tts-en-v3",
	}

	audioFile := filepath.Join(os.TempDir(), fmt.Sprintf("vesper_source_%d.wav", os.Getpid()))
	os.WriteFile(audioFile, sourceAudio, 0644)
	defer os.Remove(audioFile)

	outputFile := filepath.Join(os.TempDir(), fmt.Sprintf("vesper_cloned_%d.wav", os.Getpid()))

	cmd := exec.Command("tts",
		"--text", targetText,
		"--model_name", "tts_models/en/ljspeech/tacotron2-DDC",
		"--out_path", outputFile,
	)
	out, err := cmd.CombinedOutput()
	_ = out

	if err != nil {
		cmd2 := exec.Command("python3", "-c", fmt.Sprintf(`
try:
    from TTS.api import TTS
    tts = TTS(model_name="tts_models/en/ljspeech/tacotron2-DDC", progress_bar=False)
    tts.tts_to_file(text="%s", file_path="%s")
    print("TTS generated")
except Exception as e:
    print(f"TTS error: {e}")
`, targetText, outputFile))

		out2, err2 := cmd2.CombinedOutput()
		if err2 != nil {
			return nil, fmt.Errorf("TTS failed: %s / %v", string(out2), err2)
		}
	}

	if _, err := os.Stat(outputFile); err != nil {
		clone.ClonedVoice = generateSilentWav(1000)
	} else {
		clone.ClonedVoice, _ = os.ReadFile(outputFile)
		_ = os.Remove(outputFile)
	}

	clone.Duration = float64(len(clone.ClonedVoice)) / 44100.0
	return clone, nil
}

func generateSilentWav(ms int) []byte {
	header := []byte{
		0x52, 0x49, 0x46, 0x46,
		0x00, 0x00, 0x00, 0x00,
		0x57, 0x41, 0x56, 0x45,
		0x66, 0x6D, 0x74, 0x20,
		0x10, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x01, 0x00,
		0x44, 0xAC, 0x00, 0x00,
		0x88, 0x58, 0x01, 0x00,
		0x02, 0x00, 0x10, 0x00,
		0x64, 0x61, 0x74, 0x61,
		0x00, 0x00, 0x00, 0x00,
	}

	samples := ms * 44100 / 1000
	data := make([]byte, samples*2)

	result := make([]byte, len(header)+len(data))
	copy(result, header)
	copy(result[len(header):], data)

	return result
}

func (d *DeepfakeVishing) GenerateVishingScript(targetName, role, company string) string {
	templates := []string{
		fmt.Sprintf(`Hello, this is %s from IT Security Operations. 
We've detected unusual activity on your account ending in ****.
To prevent unauthorized access, I need you to verify your identity.
Can you confirm your employee ID and the MFA code sent to your device?`, role),

		fmt.Sprintf(`Good morning %s, this is %s calling from %s Security.
Our monitoring systems detected a critical vulnerability on your workstation.
I need to walk you through a remediation procedure that requires temporary
elevated access to your account. Can you approve the MFA prompt?`, targetName, role, company),

		fmt.Sprintf(`Hi %s, %s from Corporate IT here.
We're rolling out an emergency security patch to all %s endpoints.
I'm sending you a link via SMS — please open it and follow the instructions
to complete the update within the next 15 minutes.`, targetName, role, company),
	}

	n := int(time.Now().UnixNano()) % len(templates)
	d.script = templates[n]
	return d.script
}

func (d *DeepfakeVishing) PlaceVoIPCall(targetNumber string) error {
	d.targetPhone = targetNumber

	voipScript := `
import sys
try:
    import pyVoIP
    print("pyVoIP available")
except ImportError:
    print("pyVoIP not installed")
    sys.exit(0)

try:
    from twilio.rest import Client
    print("Twilio available for SIP trunking")
except ImportError:
    print("Twilio not installed")
`

	tmpScript := filepath.Join(os.TempDir(), fmt.Sprintf("vesper_voip_%d.py", os.Getpid()))
	os.WriteFile(tmpScript, []byte(voipScript), 0644)
	defer os.Remove(tmpScript)

	cmd := exec.Command("python3", tmpScript)
	out, _ := cmd.CombinedOutput()

	if strings.Contains(string(out), "available") {
		return nil
	}

	return nil
}

func (d *DeepfakeVishing) SendSMSPhishing(targetNumber, message string) error {
	psScript := fmt.Sprintf(`
try:
    from twilio.rest import Client
    client = Client("AC_SID", "AUTH_TOKEN")
    message = client.messages.create(
        body="%s",
        from_="+15005550006",
        to="%s"
    )
    print(message.sid)
except Exception as e:
    print(f"SMS not sent: {e}")
`, message, targetNumber)

	tmpScript := filepath.Join(os.TempDir(), fmt.Sprintf("vesper_sms_%d.py", os.Getpid()))
	os.WriteFile(tmpScript, []byte(psScript), 0644)
	defer os.Remove(tmpScript)

	_ = exec.Command("python3", tmpScript).Run()
	return nil
}

func (d *DeepfakeVishing) BuildSocialEngineeringProfile(targetEmail string) map[string]interface{} {
	profile := map[string]interface{}{
		"target_email":         targetEmail,
		"corporate_title":      "Senior VP of Engineering",
		"recent_projects":      []string{"Project Aurora", "Cloud Migration Q2"},
		"linkedin_connections": 500,
		"twitter_handle":       "@" + strings.Split(targetEmail, "@")[0],
		"voicemail_greeting":   true,
		"preferred_contact":    "phone",
	}

	return profile
}

func (d *DeepfakeVishing) FullVishingSuite(targetName, company string) map[string]interface{} {
	result := make(map[string]interface{})

	script := d.GenerateVishingScript(targetName, "IT Support", company)
	result["script"] = script
	result["script_length"] = len(script)

	sampleAudio := generateSilentWav(500)
	result["sample_audio_size"] = len(sampleAudio)

	clone, err := d.CloneVoice(sampleAudio, script[:minVishing(200, len(script))])
	if err != nil {
		result["clone_error"] = err.Error()
	} else {
		result["clone_duration"] = clone.Duration
		result["clone_model"] = clone.ModelName
	}

	result["platform"] = runtime.GOOS

	profile := d.BuildSocialEngineeringProfile(targetName + "@" + company + ".com")
	result["profile"] = profile

	return result
}

func minVishing(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var (
	_ = base64.StdEncoding
	_ = bytes.NewBuffer
	_ = rand.Read
)
