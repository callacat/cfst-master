// in callacat/cfst-master/cfst-master-2d2a1ac99ee8da5a0ba09ea066319c4c1df907a6/pkg/aggregator/aggregator.go
package aggregator

import (
	"controller/pkg/models"
	"fmt" // [新增]
)


// [修改] 按 operator 和 IPVersion 的组合来聚合
func Aggregate(drs []models.DeviceResult) map[string][]models.DeviceResult {
    ag := make(map[string][]models.DeviceResult)
    for _, d := range drs {
        // 创建复合键, e.g., "cu-v4" or "cm-v6"
        compositeKey := fmt.Sprintf("%s-%s", d.Operator, d.IPVersion)
        ag[compositeKey] = append(ag[compositeKey], d)
    }
    return ag
}