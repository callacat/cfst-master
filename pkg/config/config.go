package config

import (
    "os"
    "gopkg.in/yaml.v2"
)

type Config struct {
    Gist struct {
        Token        string   `yaml:"token"`
        DeviceGists  []string `yaml:"device_gists"`
        ResultGistID string   `yaml:"result_gist_id"`
    } `yaml:"gist"`

    DNS struct {
        Provider    string `yaml:"provider"`
        Domain      string `yaml:"domain"`
        Subdomain   string `yaml:"subdomain"`
        TTL         int    `yaml:"ttl"`
        Lines       []Line `yaml:"lines"`
    } `yaml:"dns"`

    Scoring struct {
        LatencyWeight float64 `yaml:"latency_weight"`
        SpeedWeight   float64 `yaml:"speed_weight"`
        JitterWeight  float64 `yaml:"jitter_weight"`
        LossWeight    float64 `yaml:"loss_weight"`
    } `yaml:"scoring"`

    Huawei struct {
        ProjectID string `yaml:"project_id"`
    } `yaml:"huawei"`
}

type Line struct {
    Operator    string `yaml:"operator"`
    RecordsetID string `yaml:"recordset_id"`
    Cap         int    `yaml:"cap"`
}

// Load 从文件和环境变量加载配置
func Load(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    var cfg Config
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return nil, err
    }
    // 替换 env var 占位
    cfg.Gist.Token = os.ExpandEnv(cfg.Gist.Token)
    cfg.Huawei.ProjectID = os.ExpandEnv(cfg.Huawei.ProjectID)
    return &cfg, nil
}
