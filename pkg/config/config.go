package config

import (
	"os"
	"gopkg.in/yaml.v2"
)

type Huawei struct {
	Enabled   bool   `yaml:"enabled"`
	ProjectID string `yaml:"project_id"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	Region    string `yaml:"region"`
}

type Config struct {
	Gist struct {
		Token        string   `yaml:"token"`
		ProxyPrefix  string   `yaml:"proxy_prefix"`
		DeviceGists  []string `yaml:"device_gists"`
		ResultGistID string   `yaml:"result_gist_id"`
        MaxResultAgeHours int `yaml:"max_result_age_hours"`
	} `yaml:"gist"`

	DNS struct {
		Domain    string `yaml:"domain"`
		Subdomain string `yaml:"subdomain"`
		TTL       int    `yaml:"ttl"`
		Lines     []Line `yaml:"lines"`
	} `yaml:"dns"`

	Huawei     Huawei     `yaml:"huawei"`
	Scoring    Scoring    `yaml:"scoring"`
	Thresholds Thresholds `yaml:"thresholds"`
	Selection  Selection  `yaml:"selection"`
}

type Line struct {
	Operator        string `yaml:"operator"`
	ARecordsetID    string `yaml:"a_recordset_id"`
	AAAARecordsetID string `yaml:"aaaa_recordset_id"`
	Cap             int    `yaml:"cap"`
}

type Scoring struct {
	LatencyWeight float64 `yaml:"latency_weight"`
	SpeedWeight   float64 `yaml:"speed_weight"`
	LossWeight    float64 `yaml:"loss_weight"`
}

type Thresholds struct {
	MaxLatencyMs    int     `yaml:"max_latency_ms"`
	MinDownloadMbps float64 `yaml:"min_download_mbps"`
	MaxLossPct      float64 `yaml:"max_loss_pct"`
}

type Selection struct {
	CooldownMinutes    int     `yaml:"cooldown_minutes"`
	HysteresisEnterPct float64 `yaml:"hysteresis_enter_pct"`
	HysteresisExitPct  float64 `yaml:"hysteresis_exit_pct"`
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
	cfg.Huawei.AccessKey = os.ExpandEnv(cfg.Huawei.AccessKey)
	cfg.Huawei.SecretKey = os.ExpandEnv(cfg.Huawei.SecretKey)
	return &cfg, nil
}