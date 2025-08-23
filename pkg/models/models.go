// 请将此文件的内容完整地覆盖到: pkg/models/models.go

package models

import (
	"controller/pkg/config"
	"fmt"
	"strconv"
	"time"
)

// DeviceResult 设备推送的单条测试记录
type DeviceResult struct {
	Device    string  `json:"device"`
	Operator  string  `json:"operator"` // 将由文件名填充
	IP        string  `json:"ip"`
	LatencyMs int     `json:"latency_ms"`
	LossPct   float64 `json:"loss_pct"`
	DLMbps    float64 `json:"dl_mbps"`
	Region    string  `json:"region"`
	Score     float64 `json:"score,omitempty"`
	IPVersion string  `json:"-"` // 将由文件名填充
}

// SelectedItem 包含 IP 和其来源的详细信息
type SelectedItem struct {
	IP           string  `json:"ip"`
	SourceDevice string  `json:"source_device"`
	Score        float64 `json:"score"`
	LatencyMs    int     `json:"latency_ms"`
	DLMbps       float64 `json:"dl_mbps"`
	Region       string  `json:"region"`
}

// LineResult 包含 active 和 candidates 列表
type LineResult struct {
	Operator   string         `json:"operator"`
	Active     []SelectedItem `json:"active"`
	Candidates []SelectedItem `json:"candidates"`
}

// FinalResult 写入 Gist 的结构
type FinalResult struct {
	Timestamp time.Time         `json:"timestamp"`
	Domain    string            `json:"domain"`
	Subdomain string            `json:"subdomain"`
	Lines     []LineResult      `json:"lines"`
	Explain   map[string]string `json:"explain"`
}

// BuildResult 将选中结果包装成 FinalResult
func BuildResult(sel map[string]LineResult, cfg *config.Config) FinalResult {
	fr := FinalResult{
		Timestamp: time.Now(),
		Domain:    cfg.DNS.Domain,
		Subdomain: cfg.DNS.Subdomain,
		Explain: map[string]string{
			"max_latency_ms":    strconv.Itoa(cfg.Thresholds.MaxLatencyMs),
			"min_download_mbps": fmt.Sprintf("%.2f", cfg.Thresholds.MinDownloadMbps),
			"max_loss_pct":      fmt.Sprintf("%.2f", cfg.Thresholds.MaxLossPct),
		},
	}

	for _, ln := range sel {
		fr.Lines = append(fr.Lines, ln)
	}
	return fr
}
