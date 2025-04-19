package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	// QoS class for prioritization (high, medium, low)
	QoSPriority string `json:"qosPriority"`
	// Edge node identifier
	EdgeNodeID string `json:"edgeNodeId"`
	// Whether to enable SR-IOV support
	EnableSRIOV bool `json:"enableSRIOV"`
	// Whether to enable DPDK support
	EnableDPDK bool `json:"enableDPDK"`
	// Maximum latency treshold in milliseconds
	LatencyTreshold int `json:"latencyTreshold"`
	// Heartbeat interval for cloud connectivity in seconds
	CloudHeartbeatSec int `json:"cloudHeartbeatSec"`
	// Failover strategy (fast, balanced, reliable)
	FailoverStrategy string `json:"failoverStrategy"`
	// Kubeconfig file path (empty for in-cluster config)
	Kubeconfig string `json:"kubeconfig"`
}

func DefaultConfig() *Config {
	return &Config{
		QoSPriority:       "high",
		EdgeNodeID:        getDefaultEdgeNodeID(),
		EnableSRIOV:       false,
		EnableDPDK:        false,
		LatencyTreshold:   10,
		CloudHeartbeatSec: 30,
		FailoverStrategy:  "balanced",
		Kubeconfig:        "", // so it will use the pod's identity
	}
}

func LoadConfig(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	// if a file is provided, overwrite the default config
	if configPath != "" {
		file, err := os.Open(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open config file: %w", err)
		}
		defer file.Close()

		decoder := json.NewDecoder(file)
		if err := decoder.Decode(cfg); err != nil {
			return nil, fmt.Errorf("failed to decode config file: %w", err)
		}
	}

	// override with environment variables
	overrideFromEnv(cfg)

	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func overrideFromEnv(cfg *Config) {
	// QoS priority
	if val := os.Getenv("NSM_QOS_PRIORITY"); val != "" {
		cfg.QoSPriority = val
	}

	// Edge Node ID
	if val := os.Getenv("NSM_EDGE_NODE_ID"); val != "" {
		cfg.EdgeNodeID = val
	}

	// Enable SRIOV
	if val := os.Getenv("NSM_ENABLE_SRIOV"); val != "" {
		cfg.EnableSRIOV = strings.ToLower(val) == "true"
	}

	// Enable DPDK
	if val := os.Getenv("NSM_ENABLE_DPDK"); val != "" {
		cfg.EnableDPDK = strings.ToLower(val) == "true"
	}

	// Latency Treshold
	if val := os.Getenv("NSM_LATENCY_TRESHOLD"); val != "" {
		var treshold int
		if _, err := fmt.Sscanf(val, "%d", &treshold); err == nil {
			cfg.LatencyTreshold = treshold
		}
	}

	// Cloud Heartbeat
	if val := os.Getenv("NSM_CLOUD_HEARTBEAT_SEC"); val != "" {
		var heartbeat int
		if _, err := fmt.Sscanf(val, "%d", &heartbeat); err == nil {
			cfg.CloudHeartbeatSec = heartbeat
		}
	}

	// Failover Strategy
	if val := os.Getenv("NSM_FAILOVER_STRATEGY"); val != "" {
		cfg.FailoverStrategy = val
	}

	// Kubeconfig
	if val := os.Getenv("NSM_KUBECONFIG"); val != "" {
		cfg.Kubeconfig = val
	}
}

func validateConfig(cfg *Config) error {
	// Validate QoS
	validQoS := map[string]bool{"high": true, "medium": true, "low": true}
	if !validQoS[strings.ToLower(cfg.QoSPriority)] {
		return fmt.Errorf("invalid QoS priority: %s, must be one of: high, medium, low", cfg.QoSPriority)
	}

	// Validate Edge Node ID
	if cfg.EdgeNodeID == "" {
		return fmt.Errorf("edge node ID cannot be empty")
	}

	// Validate Latency Treshold
	if cfg.LatencyTreshold <= 0 {
		return fmt.Errorf("latency treshold must be greater than 0")
	}

	// Validate Cloud Heartbeat
	if cfg.CloudHeartbeatSec <= 0 {
		return fmt.Errorf("cloud heartbeat interval must be greater than 0")
	}

	// Validate failover strategy
	validFailover := map[string]bool{"fast": true, "balanced": true, "reliable": true}
	if !validFailover[strings.ToLower(cfg.FailoverStrategy)] {
		return fmt.Errorf("invalid failover strategy: %s, must be one of: fast, balanced, reliable", cfg.FailoverStrategy)
	}

	return nil
}

func getDefaultEdgeNodeID() string {
	hostname, err := os.Hostname()
	if err == nil && hostname != "" {
		return fmt.Sprintf("edge-%s", hostname)
	}

	// Fallback to a random identifier
	return fmt.Sprintf("edge-%d", os.Getpid())
}
