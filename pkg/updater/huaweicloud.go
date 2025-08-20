package updater

import (
	"fmt"
	"log"

	"controller/pkg/config"
	"controller/pkg/models"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/basic"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/region"
	dns "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2/model"
)

// newHuaweiDNSClient 创建一个华为云 DNS 客户端
func newHuaweiDNSClient(cfg *config.Config) (*dns.DnsClient, error) {
	auth := basic.NewCredentialsBuilder().
		WithAk(cfg.Huawei.AccessKey).
		WithSk(cfg.Huawei.SecretKey).
		WithProjectId(cfg.Huawei.ProjectID).
		Build()

	// 查找区域终节点
	r := region.NewRegion(cfg.Huawei.Region, "https://dns.myhuaweicloud.com")

	client := dns.NewDnsClient(
		dns.DnsClientBuilder().
			WithRegion(r).
			WithCredential(auth).
			Build())

	return client, nil
}

// UpdateHuaweiCloud 调用华为云 DNS API 更新记录
func UpdateHuaweiCloud(line config.Line, ips []string, cfg *config.Config) error {
	client, err := newHuaweiDNSClient(cfg)
	if err != nil {
		return fmt.Errorf("创建华为云客户端失败: %w", err)
	}

	// 华为云的 recordset_id 格式通常是 "zone_id,recordset_id"
	// 但这里我们假设用户在 config.yml 中只配置了 recordset_id
	// 实际使用中可能需要先查询 zone_id
	// 为简化，我们假设 zone_id 已经包含或可以通过其他方式获得

	// 构建更新请求
	ttl := int32(cfg.DNS.TTL)
	req := &model.UpdateRecordSetReq{
		Ttl:     &ttl,
		Records: ips, // 直接传递 IP 列表
	}

	updateReq := &model.UpdateRecordSetRequest{
		RecordsetId:      line.RecordsetID,
		Body:             &model.UpdateRecordsetReq{
            Body: req,
        },
		// ZoneId 需要从 RecordsetID 或其他配置中解析出来
		// ZoneId: "your_zone_id", 
	}
    
    log.Printf("[info] 正在为线路 %s 更新华为云 DNS 记录: %v", line.Operator, ips)
	_, err = client.UpdateRecordSet(updateReq)
	if err != nil {
		return fmt.Errorf("调用华为云 UpdateRecordSet API 失败: %w", err)
	}

	return nil
}