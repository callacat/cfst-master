// 请将此文件内容完全替换为以下代码

package selector

import (
	"sort"

	"controller/pkg/config"
	"controller/pkg/models"
)

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
				r.Score = score
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
			cap := ln.Cap
			for i, r := range uniq {
				item := models.SelectedItem{
					IP:           r.IP,
					SourceDevice: r.Device,
					Score:        r.Score,
					LatencyMs:    r.LatencyMs,
					DLMbps:       r.DLMbps,
					Region:       r.Region,
				}
				if i < cap {
					active = append(active, item)
				} else {
					candidates = append(candidates, item)
				}
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
