package selector

import (
	"sort"

	"controller/pkg/config"
	"controller/pkg/models"
)

// [修改] 返回值模型更丰富
func SelectTop(
	ag map[string][]models.DeviceResult,
	lines []config.Line,
	sc config.Scoring,
	th config.Thresholds,
) map[string]models.LineResult {

	selectedResults := make(map[string]models.LineResult)

	for _, ln := range lines {
		list, ok := ag[ln.Operator]
		if !ok {
			continue
		}

		var qualified []models.DeviceResult
		for _, r := range list {
			// [新增] 质量阈值过滤
			if r.LatencyMs > th.MaxLatencyMs ||
				r.DLMbps < th.MinDownloadMbps ||
				r.LossPct > th.MaxLossPct {
				continue
			}
			
			// 计算分数
			score := sc.LatencyWeight*float64(r.LatencyMs) +
				sc.SpeedWeight*r.DLMbps +
				sc.JitterWeight*float64(r.JitterMs) +
				sc.LossWeight*r.LossPct
			r.Score = score
			qualified = append(qualified, r)
		}

		// 去重同 IP & 排序
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

		// 分离 Active 和 Candidates
		var active, candidates []models.SelectedItem
		cap := ln.Cap
		for i, r := range uniq {
			item := models.SelectedItem{
				IP:           r.IP,
				SourceDevice: r.Device,
				Score:        r.Score,
				LatencyMs:    r.LatencyMs,
				DLMbps:       r.DLMbps,
			}
			if i < cap {
				active = append(active, item)
			} else {
				candidates = append(candidates, item)
			}
		}

		selectedResults[ln.Operator] = models.LineResult{
			Operator:   ln.Operator,
			Active:     active,
			Candidates: candidates,
		}
	}
	return selectedResults
}