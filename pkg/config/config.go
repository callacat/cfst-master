package config

import (
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Gist struct {
		Token        string   `yaml:"token"`
		ProxyPrefix  string   `yaml:"proxy_prefix"` // [新增]
		DeviceGists  []string `yaml:"device_gists"`
		ResultGistID string   `yaml:"result_gist_id"`
	} `yaml:"gist"`

	DNS struct {
		Provider  string `yaml:"provider"`
		Domain    string `yaml:"domain"`
		Subdomain string `yaml:"subdomain"`
		TTL       int    `yaml:"ttl"`
		Lines     []Line `yaml:"lines"`
	} `yaml:"dns"`

	Scoring    Scoring    `yaml:"scoring"`
	Thresholds Thresholds `yaml:"thresholds"` // [新增]
	Selection  Selection  `yaml:"selection"`  // [新增]

	Huawei struct {
		ProjectID string `yaml:"project_id"`
		AccessKey string `yaml:"access_key"` // [新增]
		SecretKey string `yaml:"secret_key"` // [新增]
		Region    string `yaml:"region"`    // [新增]
	} `yaml:"huawei"`
}

type Scoring struct {
	LatencyWeight float64 `yaml:"latency_weight"`
	SpeedWeight   float64 `yaml:"speed_weight"`
	JitterWeight  float64 `yaml:"jitter_weight"`
	LossWeight    float64 `yaml:"loss_weight"`
}

// [新增]
type Thresholds struct {
	MaxLatencyMs    int     `yaml:"max_latency_ms"`
	MinDownloadMbps float64 `yaml:"min_download_mbps"`
	MaxLossPct      float64 `yaml:"max_loss_pct"`
}

// [新增]
type Selection struct {
	CooldownMinutes    int     `yaml:"cooldown_minutes"`
	HysteresisEnterPct float64 `yaml:"hysteresis_enter_pct"`
	HysteresisExitPct  float64 `yaml:"hysteresis_exit_pct"`
}

type Line struct {
	Operator    string `yaml:"operator"`
	RecordsetID string `yaml:"recordset_id"`
	Cap         int    `yaml:"cap"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	cfg.Gist.Token = os.ExpandEnv(cfg.Gist.Token)
    cfg.Huawei.ProjectID = os.ExpandEnv(cfg.Huawei.ProjectID)
    cfg.Huawei.AccessKey = os.ExpandEnv(cfg.Huawei.AccessKey) // [新增]
    cfg.Huawei.SecretKey = os.ExpandEnv(cfg.Huawei.SecretKey) // [新增]
    return &cfg, nil
}