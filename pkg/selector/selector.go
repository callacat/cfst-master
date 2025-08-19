package selector

import (
    "sort"
    "controller/pkg/models"
    "controller/pkg/config"
)

// SelectTop 对每线路的 DeviceResult 打分并选 Top cap 个 IP
func SelectTop(
    ag map[string][]models.DeviceResult,
    lines []config.Line,
    sc config.Scoring,
) map[string][]string {
    sel := make(map[string][]string)
    for _, ln := range lines {
        list := ag[ln.Operator]
	// 计算分数时：
	for i, r := range list {
		score := sc.LatencyWeight*(1/float64(r.LatencyMs)) +
				sc.SpeedWeight*r.DLMbps -
				sc.JitterWeight*float64(r.JitterMs) -
				sc.LossWeight*r.LossPct
		list[i].Score = score
	}

        // 去重同 IP & 排序
        m := map[string]models.DeviceResult{}
        for _, r := range list {
            prev, ok := m[r.IP]
            if !ok || r.Score > prev.Score {
                m[r.IP] = r
            }
        }
        uniq := make([]models.DeviceResult, 0, len(m))
        for _, v := range m { uniq = append(uniq, v) }
        sort.Slice(uniq, func(i, j int) bool {
            return uniq[i].Score > uniq[j].Score
        })
        cap := ln.Cap
        if len(uniq) < cap {
            cap = len(uniq)
        }
        for _, r := range uniq[:cap] {
            sel[ln.Operator] = append(sel[ln.Operator], r.IP)
        }
    }
    return sel
}
