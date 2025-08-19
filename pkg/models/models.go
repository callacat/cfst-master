package models

import "time"

// DeviceResult 设备推送的单条测试记录
type DeviceResult struct {
    Device     string  `json:"device"`
    Operator   string  `json:"operator"`    // ct/cu/cm
    IP         string  `json:"ip"`
    LatencyMs  int     `json:"latency_ms"`
    DLMbps     float64 `json:"dl_mbps"`
    JitterMs   int     `json:"jitter_ms"`
    LossPct    float64 `json:"loss_pct"`
    Score      float64 `json:"score"`       // 新增字段，用于存储综合打分
}

// LineResult 最终选出的每线路结果
type LineResult struct {
    Operator string   `json:"operator"`
    IPs      []string `json:"ips"`
}

// FinalResult 写入 Gist 的结构
type FinalResult struct {
    Timestamp   time.Time    `json:"timestamp"`
    Domain      string       `json:"domain"`
    Subdomain   string       `json:"subdomain"`
    Lines       []LineResult `json:"lines"`
}

// BuildResult 将选中结果包装成 FinalResult
func BuildResult(sel map[string][]string, cfg interface {
    DNS struct {
        Domain    string
        Subdomain string
        Lines     []struct{ Operator string }
    }
}) FinalResult {
    fr := FinalResult{
        Timestamp: time.Now(),
        Domain:    cfg.DNS.Domain,
        Subdomain: cfg.DNS.Subdomain,
    }
    for _, ln := range cfg.DNS.Lines {
        fr.Lines = append(fr.Lines, LineResult{
            Operator: ln.Operator,
            IPs:      sel[ln.Operator],
        })
    }
    return fr
}
