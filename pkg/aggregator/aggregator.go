package aggregator

import "controller/pkg/models"

// 按 operator 聚合所有设备结果
func Aggregate(drs []models.DeviceResult) map[string][]models.DeviceResult {
    ag := make(map[string][]models.DeviceResult)
    for _, d := range drs {
        ag[d.Operator] = append(ag[d.Operator], d)
    }
    return ag
}
