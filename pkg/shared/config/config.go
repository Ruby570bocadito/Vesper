// Package config provides unified configuration for all Vesper Framework components.
// Configuration is loaded from YAML files, environment variables, and CLI flags.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the root configuration for the entire framework.
type Config struct {
	Agent     AgentConfig     `yaml:"agent"`
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	Crypto    CryptoConfig    `yaml:"crypto"`
	AI        AIConfig        `yaml:"ai"`
	Logging   LoggingConfig   `yaml:"logging"`
	Lab       LabConfig       `yaml:"lab"`
	Blue      BlueConfig      `yaml:"blue"`
	Evasion   EvasionConfig   `yaml:"evasion"`
	Safety    SafetyConfig    `yaml:"safety"`
	Dashboard DashboardConfig `yaml:"dashboard"`
}

type AgentConfig struct {
	ID               string `yaml:"id"`
	Name             string `yaml:"name"`
	C2Server         string `yaml:"c2_server"`
	C2Port           int    `yaml:"c2_port"`
	BridgePort       int    `yaml:"bridge_port"`
	HeartbeatSeconds int    `yaml:"heartbeat_seconds"`
	StealthMode      bool   `yaml:"stealth_mode"`
}

type ServerConfig struct {
	Host        string        `yaml:"host"`
	Port        int           `yaml:"port"`
	MaxSessions int           `yaml:"max_sessions"`
	SessionTTL  time.Duration `yaml:"session_ttl"`
	EnableWS    bool          `yaml:"enable_ws"`
	WSPort      int           `yaml:"ws_port"`
	EnableTLS   bool          `yaml:"enable_tls"`
	AutoCert    bool          `yaml:"auto_cert"`
	CertFile    string        `yaml:"cert_file"`
	KeyFile     string        `yaml:"key_file"`
}

type DatabaseConfig struct {
	Driver      string `yaml:"driver"`
	DSN         string `yaml:"dsn"`
	MaxConns    int    `yaml:"max_conns"`
	AutoMigrate bool   `yaml:"auto_migrate"`
}

type CryptoConfig struct {
	KeyExchange   string        `yaml:"key_exchange"`
	AEADCipher    string        `yaml:"aead_cipher"`
	SessionKeyTTL time.Duration `yaml:"session_key_ttl"`
}

type AIConfig struct {
	Enabled       bool    `yaml:"enabled"`
	Model         string  `yaml:"model"`
	OllamaHost    string  `yaml:"ollama_host"`
	OllamaPort    int     `yaml:"ollama_port"`
	Temperature   float64 `yaml:"temperature"`
	MaxTokens     int     `yaml:"max_tokens"`
	AutoApproval  bool    `yaml:"auto_approval"`
	MinConfidence float64 `yaml:"min_confidence"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
	File   string `yaml:"file"`
}

type LabConfig struct {
	Enable     bool   `yaml:"enable"`
	Network    string `yaml:"network"`
	Subnet     string `yaml:"subnet"`
	AttackerIP string `yaml:"attacker_ip"`
}

type BlueConfig struct {
	Enabled    bool   `yaml:"enabled"`
	ReportPath string `yaml:"report_path"`
}

type EvasionConfig struct {
	AMSI        bool `yaml:"amsi"`
	ETW         bool `yaml:"etw"`
	Polymorphic bool `yaml:"polymorphic"`
	SleepJitter bool `yaml:"sleep_jitter"`
	JitterMin   int  `yaml:"jitter_min_ms"`
	JitterMax   int  `yaml:"jitter_max_ms"`
}

type SafetyConfig struct {
	KillSwitchEnabled bool   `yaml:"kill_switch_enabled"`
	KillSwitchCode    string `yaml:"kill_switch_code"`
	AutoDestructHours int    `yaml:"auto_destruct_hours"`
	GeofenceEnabled   bool   `yaml:"geofence_enabled"`
	MaxInfections     int    `yaml:"max_infections"`
	NoPersistence     bool   `yaml:"no_persistence"`
}

type DashboardConfig struct {
	Enabled        bool   `yaml:"enabled"`
	Port           int    `yaml:"port"`
	DevMode        bool   `yaml:"dev_mode"`
	AuthToken      string `yaml:"auth_token"`
	JWTSecret      string `yaml:"jwt_secret"`
	JWTExpiryHours int    `yaml:"jwt_expiry_hours"`
}

// Load reads configuration from a YAML file path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", path, err)
	}

	// Expand environment variables in config content
	data = []byte(os.ExpandEnv(string(data)))

	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	// Override with environment variables
	cfg.applyEnvOverrides()

	return cfg, nil
}

// Default returns a Config with sensible defaults.
func Default() *Config {
	return &Config{
		Agent: AgentConfig{
			C2Port:           8443,
			BridgePort:       9100,
			HeartbeatSeconds: 30,
			StealthMode:      false,
		},
		Server: ServerConfig{
			Host:        "127.0.0.1",
			Port:        8443,
			MaxSessions: 5000,
			SessionTTL:  5 * time.Minute,
			EnableWS:    true,
			WSPort:      8446,
			EnableTLS:   true,
			AutoCert:    true,
		},
		Database: DatabaseConfig{
			Driver:      "sqlite",
			DSN:         "vesper.db",
			MaxConns:    10,
			AutoMigrate: true,
		},
		Crypto: CryptoConfig{
			KeyExchange:   "X25519",
			AEADCipher:    "XChaCha20-Poly1305",
			SessionKeyTTL: 30 * time.Minute,
		},
		AI: AIConfig{
			Enabled:       true,
			Model:         "llama3.2",
			OllamaHost:    "localhost",
			OllamaPort:    11434,
			Temperature:   0.7,
			MaxTokens:     4096,
			AutoApproval:  true,
			MinConfidence: 0.65,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
			Output: "stdout",
		},
		Lab: LabConfig{
			Enable:     false,
			Network:    "vesper-lab",
			Subnet:     "172.20.0.0/24",
			AttackerIP: "172.20.0.10",
		},
		Blue: BlueConfig{
			Enabled:    true,
			ReportPath: "reports/blue_metrics.json",
		},
		Evasion: EvasionConfig{
			AMSI:        true,
			ETW:         true,
			Polymorphic: false,
			SleepJitter: true,
			JitterMin:   500,
			JitterMax:   5000,
		},
		Safety: SafetyConfig{
			KillSwitchEnabled: true,
			KillSwitchCode:    "EMERGENCY_STOP",
			AutoDestructHours: 2,
			GeofenceEnabled:   true,
			MaxInfections:     1000,
			NoPersistence:     true,
		},
		Dashboard: DashboardConfig{
			Enabled:        true,
			Port:           3000,
			DevMode:        false,
			JWTExpiryHours: 24,
		},
	}
}

// applyEnvOverrides overrides config values with environment variables.
func (c *Config) applyEnvOverrides() {
	if v := os.Getenv("VESPER_C2_HOST"); v != "" {
		c.Server.Host = v
	}
	if v := os.Getenv("VESPER_C2_PORT"); v != "" {
		fmt.Sscanf(v, "%d", &c.Server.Port)
	}
	if v := os.Getenv("VESPER_AGENT_ID"); v != "" {
		c.Agent.ID = v
	}
	if v := os.Getenv("VESPER_C2_SERVER"); v != "" {
		c.Agent.C2Server = v
	}
	if v := os.Getenv("VESPER_DB_DSN"); v != "" {
		c.Database.DSN = v
	}
	if v := os.Getenv("VESPER_AI_MODEL"); v != "" {
		c.AI.Model = v
	}
	if v := os.Getenv("VESPER_OLLAMA_HOST"); v != "" {
		c.AI.OllamaHost = v
	}
	if v := os.Getenv("VESPER_STEALTH"); strings.ToLower(v) == "true" {
		c.Agent.StealthMode = true
	}
	if v := os.Getenv("VESPER_LOG_LEVEL"); v != "" {
		c.Logging.Level = v
	}
	if v := os.Getenv("VESPER_KILL_SWITCH"); v != "" {
		c.Safety.KillSwitchCode = v
	}
	if v := os.Getenv("VESPER_DASHBOARD_AUTH_TOKEN"); v != "" {
		c.Dashboard.AuthToken = v
	}
	if v := os.Getenv("VESPER_DASHBOARD_JWT_SECRET"); v != "" {
		c.Dashboard.JWTSecret = v
	}
}

// Validate checks configuration for required fields and consistency.
func (c *Config) Validate() error {
	if c.Agent.C2Server == "" && c.Server.Port == 0 {
		return fmt.Errorf("either agent.c2_server or server.port must be configured")
	}
	if c.AI.Enabled && c.AI.OllamaHost == "" {
		return fmt.Errorf("ai.ollama_host is required when AI is enabled")
	}
	if c.Safety.GeofenceEnabled && c.Lab.Network == "" {
		// Warn but don't fail — lab network is optional
	}
	return nil
}

// Merge overlays a partial config on top of this one.
func (c *Config) Merge(overlay *Config) {
	if overlay == nil {
		return
	}
	data, _ := yaml.Marshal(c)
	overlayData, _ := yaml.Marshal(overlay)
	merged := append(data, overlayData...)
	yaml.Unmarshal(merged, c)
}
