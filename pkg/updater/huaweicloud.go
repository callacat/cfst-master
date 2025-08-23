package updater

import (
	"fmt"
	"log"
	"time"

	"controller/pkg/config"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/basic"
	coreCfg "github.com/huaweicloud/huaweicloud-sdk-go-v3/core/config"
	dns "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2/model"
	// [新增] 导入 DNS 服务专属的 region 包
	dnsRegion "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2/region"
)

// newHuaweiDNSClient 创建一个华为云 DNS 客户端 (修正版)
func newHuaweiDNSClient(cfg *config.Config) (*dns.DnsClient, error) {
	// 1. 认证信息
	auth := basic.NewCredentialsBuilder().
		WithAk(cfg.Huawei.AccessKey).
		WithSk(cfg.Huawei.SecretKey).
		WithProjectId(cfg.Huawei.ProjectID).
		Build()

	// 2. [修正] 使用 SDK 的 region 功能，而不是手动拼接 Endpoint
	// 这样可以确保 SDK 自动使用官方、正确的服务地址
	r, err := dnsRegion.SafeValueOf(cfg.Huawei.Region)
	if err != nil {
		return nil, fmt.Errorf("invalid or unsupported region specified in config: %s, error: %w", cfg.Huawei.Region, err)
	}
	log.Printf("[debug] Using Huawei Cloud DNS region: %s", cfg.Huawei.Region)

	// 3. 添加 HTTP 配置，设置网络超时
	httpConfig := coreCfg.DefaultHttpConfig().
		WithTimeout(60 * time.Second) // 设置60秒超时

	// 4. 使用 Builder 构建客户端
	client := dns.NewDnsClient(
		dns.DnsClientBuilder().
			WithRegion(r).
			WithCredential(auth).
			WithHttpConfig(httpConfig).
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