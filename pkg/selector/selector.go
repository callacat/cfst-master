// 请将此文件内容完全替换为以下代码

package selector

import (
	"math" // [新增] 导入 math 包
	"sort"

	"controller/pkg/config"
	"controller/pkg/models"
)

// [新增] roundFloat 函数用于将浮点数四舍五入到指定的小数位数
func roundFloat(val float64, precision uint) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}

func SelectTop(
	ag map[string][]models.DeviceResult,
	lines []config.Line,
	sc config.Scoring,
	th config.Thresholds,
) map[string]models.LineResult {

	selectedResults := make(map[string]models.LineResult)

	for _, ln := range lines {
		for _, ipVersion := range []string{"v4", "v6"} {
			compositeKey := ln.Operator + "-" + ipVersion
			list, ok := ag[compositeKey]
			if !ok {
				continue
			}

			var qualified []models.DeviceResult
			for _, r := range list {
				if r.LatencyMs > th.MaxLatencyMs ||
					r.DLMbps < th.MinDownloadMbps ||
					r.LossPct > th.MaxLossPct {
					continue
				}

				score := sc.LatencyWeight*float64(r.LatencyMs) +
					sc.SpeedWeight*r.DLMbps +
					sc.LossWeight*r.LossPct
				
				// [修改] 在此处将分数格式化为小数点后两位
				r.Score = roundFloat(score, 2)
				qualified = append(qualified, r)
			}

			m := make(map[string]models.DeviceResult)
			for _, r := range qualified {
				prev, ok := m[r.IP]
				if !ok || r.Score > prev.Score {
					m[r.IP] = r
				}
			}

			uniq := make([]models.DeviceResult, 0, len(m))
			for _, v := range m {
				uniq = append(uniq, v)
			}
			sort.Slice(uniq, func(i, j int) bool {
				return uniq[i].Score > uniq[j].Score
			})

			var active, candidates []models.SelectedItem
			dnsCap := ln.Cap // DNS 更新数量
			for i, r := range uniq {
				item := models.SelectedItem{
					IP:           r.IP,
					SourceDevice: r.Device,
					Score:        r.Score,
					LatencyMs:    r.LatencyMs,
					DLMbps:       r.DLMbps,
					Region:       r.Region,
				}
				// 这里的 cap 是用于 DNS 更新的
				if i < dnsCap {
					active = append(active, item)
				}
				// 依然保留所有合格的 IP 到 candidates，Gist 上传的筛选将在下一步骤处理
				candidates = append(candidates, item)
			}
			
			if len(active) > 0 || len(candidates) > 0 {
				selectedResults[compositeKey] = models.LineResult{
					Operator:   ln.Operator,
					Active:     active,
					Candidates: candidates,
				}
			}
		}
	}
	return selectedResults
}