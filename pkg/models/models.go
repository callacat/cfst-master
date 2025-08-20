package models

import (
	"controller/pkg/config"
	"time"
)

// DeviceResult 设备推送的单条测试记录
type DeviceResult struct {
	Device    string  `json:"device"`
	Operator  string  `json:"operator"` // ct/cu/cm
	IP        string  `json:"ip"`
	LatencyMs int     `json:"latency_ms"`
	DLMbps    float64 `json:"dl_mbps"`
	JitterMs  int     `json:"jitter_ms"`
	LossPct   float64 `json:"loss_pct"`
	Score     float64 `json:"score"`
}

// [修改] SelectedItem 包含 IP 和其来源的详细信息
type SelectedItem struct {
	IP           string  `json:"ip"`
	SourceDevice string  `json:"source_device"`
	Score        float64 `json:"score"`
	LatencyMs    int     `json:"latency_ms"`
	DLMbps       float64 `json:"dl_mbps"`
}

// [修改] LineResult 包含 active 和 candidates 列表
type LineResult struct {
	Operator   string         `json:"operator"`
	Active     []SelectedItem `json:"active"`
	Candidates []SelectedItem `json:"candidates"`
}

// [修改] FinalResult 写入 Gist 的结构, 增加 Explain 字段
type FinalResult struct {
	Timestamp      time.Time         `json:"timestamp"`
	Domain         string            `json:"domain"`
	Subdomain      string            `json:"subdomain"`
	Lines          []LineResult      `json:"lines"`
	Explain        map[string]string `json:"explain"`
	LastDNSWrite   time.Time         `json:"last_dns_write"`
	NextDNSWriteAt time.Time         `json:"next_dns_write_at"`
}

// [新增] State 用于在本地持久化上次的 DNS 更新状态
type State struct {
	LastDNSWrite time.Time `json:"last_dns_write"`
}

// BuildResult 将选中结果包装成 FinalResult
func BuildResult(sel map[string]LineResult, state *State, cfg *config.Config) FinalResult {
	fr := FinalResult{
		Timestamp:    time.Now(),
		Domain:       cfg.DNS.Domain,
		Subdomain:    cfg.DNS.Subdomain,
		LastDNSWrite: state.LastDNSWrite,
		NextDNSWriteAt: state.LastDNSWrite.Add(
			time.Duration(cfg.Selection.CooldownMinutes) * time.Minute),
		Explain: map[string]string{
			"cooldown_minutes":      string(rune(cfg.Selection.CooldownMinutes)),
			"hysteresis_enter_pct":  string(rune(cfg.Selection.HysteresisEnterPct)),
			"max_latency_ms":        string(rune(cfg.Thresholds.MaxLatencyMs)),
			"min_download_mbps":     string(rune(cfg.Thresholds.MinDownloadMbps)),
			"max_loss_pct":          string(rune(cfg.Thresholds.MaxLossPct)),
		},
	}
	for _, ln := range cfg.DNS.Lines {
		if res, ok := sel[ln.Operator]; ok {
			fr.Lines = append(fr.Lines, res)
		}
	}
	return fr
}