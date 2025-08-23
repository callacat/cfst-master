package updater

import (
	"fmt"
	"log"

	"controller/pkg/config"
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

	// [修改] 移除硬编码的 Endpoint URL，让 SDK 自动解析
	// 旧代码: r := region.NewRegion(cfg.Huawei.Region, "https://dns.myhuaweicloud.com")
	r := region.NewRegion(cfg.Huawei.Region, "https://dns.myhuaweicloud.com")

	client := dns.NewDnsClient(
		dns.DnsClientBuilder().
			WithRegion(r).
			WithCredential(auth).
			Build())

	return client, nil
}

// UpdateHuaweiCloud 调用华为云 DNS API 更新记录
func UpdateHuaweiCloud(recordsetID string, ips []string, cfg *config.Config) error {
	client, err := newHuaweiDNSClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create Huawei Cloud client: %w", err)
	}

	ttl := int32(cfg.DNS.TTL)

	updateReq := &model.UpdateRecordSetRequest{
		RecordsetId: recordsetID,
		Body: &model.UpdateRecordSetReq{
			Ttl:     &ttl,
			Records: &ips,
		},
	}

	log.Printf("[info] Updating Huawei Cloud DNS records for recordset %s: %v", recordsetID, ips)
	_, err = client.UpdateRecordSet(updateReq)
	if err != nil {
		return fmt.Errorf("failed to call Huawei Cloud UpdateRecordSet API: %w", err)
	}

	return nil
}
