// 请将此文件的内容完整地覆盖到: pkg/models/models.go

package models

import (
	"fmt"
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

// SelectedItem 包含优选出的IP详细信息
// [修改] 移除了 SourceDevice
type SelectedItem struct {
	IP        string  `json:"ip"`
	Score     float64 `json:"score"`
	LatencyMs int     `json:"latency_ms"`
	DLMbps    float64 `json:"dl_mbps"`
	Region    string  `json:"region"`
}

// LineResult 在程序内部流转，包含一个线路（如 cu-v4）的所有合格及待更新IP
type LineResult struct {
	Operator   string
	IPVersion  string // [新增] e.g., "v4", "v6"
	Active     []SelectedItem
	Candidates []SelectedItem
}

// --- [新增] 专用于 Gist JSON 文件输出的结构体 ---

// GistFileContent 是最终写入 Gist 中每个JSON文件的顶层结构
type GistFileContent struct {
	UpdatedAt string         `json:"updated_at"`
	Results   []SelectedItem `json:"results"`
}

// BuildResultGistFiles 将优选结果构造成准备上传到 Gist 的多个文件
// [重构] 此函数现在生成一个文件名到文件内容的映射
func BuildResultGistFiles(sel map[string]LineResult) map[string]string {
	filesToUpload := make(map[string]string)

	for key, ln := range sel {
		if len(ln.Candidates) == 0 {
			continue // 如果没有任何合格的 IP，则不生成该文件
		}

		// 文件名格式: ct-v4.json, cu-v6.json 等
		fileName := fmt.Sprintf("%s-%s.json", ln.Operator, ln.IPVersion)

		content := GistFileContent{
			UpdatedAt: time.Now().Format(time.RFC3339),
			Results:   ln.Candidates, // 上传所有合格的IP
		}

		// 转换为格式化的 JSON 字符串
		jsonBytes, err := json.MarshalIndent(content, "", "  ")
		if err != nil {
			// 在实际应用中，这里应该记录日志
			continue
		}

		filesToUpload[fileName] = string(jsonBytes)
	}

	return filesToUpload
}