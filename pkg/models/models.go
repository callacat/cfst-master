// 请将此文件的内容完整地覆盖到: pkg/models/models.go

package models

import (
	"controller/pkg/config"
	"sort"
	"time"
)

// DeviceResult 设备推送的单条测试记录 (保持不变)
type DeviceResult struct {
	Device    string  `json:"device"`
	Operator  string  `json:"operator"`
	IP        string  `json:"ip"`
	LatencyMs int     `json:"latency_ms"`
	LossPct   float64 `json:"loss_pct"`
	DLMbps    float64 `json:"dl_mbps"`
	Region    string  `json:"region"`
	Score     float64 `json:"score,omitempty"`
	IPVersion string  `json:"-"`
}

// SelectedItem 包含 IP 和其来源的详细信息 (保持不变)
type SelectedItem struct {
	IP           string  `json:"ip"`
	SourceDevice string  `json:"source_device"`
	Score        float64 `json:"score"`
	LatencyMs    int     `json:"latency_ms"`
	DLMbps       float64 `json:"dl_mbps"`
	Region       string  `json:"region"`
}

// LineResult 包含 active 和 candidates 列表 (保持不变)
// 这个结构体在程序内部流转，包含所有需要的数据
type LineResult struct {
	Operator   string
	Active     []SelectedItem
	Candidates []SelectedItem
}

// --- [新增] 专用于 Gist JSON 输出的结构体 ---

// GistLineResult 是最终写入 Gist 的线路结果，只包含 Active 字段
type GistLineResult struct {
	Operator string         `json:"operator"`
	Active   []SelectedItem `json:"active"`
}

// FinalResult 是最终写入 Gist 的顶层结构
// [修改] 移除了 Explain 字段
type FinalResult struct {
	Timestamp time.Time        `json:"timestamp"`
	Domain    string           `json:"domain"`
	Subdomain string           `json:"subdomain"`
	Lines     []GistLineResult `json:"lines"` // [修改] 使用新的 GistLineResult
}

// BuildResult 将选中结果包装成最终上传到 Gist 的 FinalResult
// [重构] 此函数现在实现了内容裁剪和格式化
func BuildResult(sel map[string]LineResult, cfg *config.Config) FinalResult {
	fr := FinalResult{
		Timestamp: time.Now(),
		Domain:    cfg.DNS.Domain,
		Subdomain: cfg.DNS.Subdomain,
		// Lines 将在下面填充
	}

	gistCap := cfg.DNS.GistUploadCap // 获取 Gist 上传数量上限

	var finalLines []GistLineResult
	for _, ln := range sel {
		// 从 Candidates (包含所有合格IP) 中根据 GistUploadCap 截取最终要上传的列表
		// 注意: Candidates 已经按分数从高到低排序
		var activeForGist []SelectedItem
		if len(ln.Candidates) > 0 {
			// 如果 GistCap 小于总数，则截取
			if gistCap < len(ln.Candidates) {
				activeForGist = ln.Candidates[:gistCap]
			} else {
				// 否则使用全部
				activeForGist = ln.Candidates
			}
		}

		// 只有当有IP时才添加到最终结果中
		if len(activeForGist) > 0 {
			finalLines = append(finalLines, GistLineResult{
				Operator: ln.Operator,
				Active:   activeForGist,
			})
		}
	}
	
	// 对最终结果按运营商名称排序，确保输出顺序稳定
	sort.Slice(finalLines, func(i, j int) bool {
		return finalLines[i].Operator < finalLines[j].Operator
	})

	fr.Lines = finalLines
	return fr
}