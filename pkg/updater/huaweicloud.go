package updater

import (
	"fmt"
	"log"
	"time" // [新增] 导入 time 包

	"controller/pkg/config"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/basic"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/region"
	
	// [新增] 导入 core/config 包
	coreCfg "github.com/huaweicloud/huaweicloud-sdk-go-v3/core/config"
	
	dns "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2"
	"github.comcom/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2/model"
)


// newHuaweiDNSClient 创建一个华为云 DNS 客户端 (优化版)
func newHuaweiDNSClient(cfg *config.Config) (*dns.DnsClient, error) {
	// 1. 认证信息 (保持不变)
	auth := basic.NewCredentialsBuilder().
		WithAk(cfg.Huawei.AccessKey).
		WithSk(cfg.Huawei.SecretKey).
		WithProjectId(cfg.Huawei.ProjectID).
		Build()

	// 2. [优化] 动态生成 Endpoint，避免硬编码
	// 假设 region 总是 "cn-north-4" 这类格式
	endpoint := fmt.Sprintf("https://dns.%s.myhuaweicloud.com", cfg.Huawei.Region)
	log.Printf("[debug] Using Huawei Cloud DNS endpoint: %s", endpoint)
	r := region.NewRegion(cfg.Huawei.Region, endpoint)

	// 3. [优化] 添加 HTTP 配置，设置网络超时
	httpConfig := coreCfg.DefaultHttpConfig().
		WithTimeout(60 * time.Second) // 设置60秒超时

	// 4. 使用 Builder 构建客户端
	client := dns.NewDnsClient(
		dns.DnsClientBuilder().
			WithRegion(r).
			WithCredential(auth).
			WithHttpConfig(httpConfig). // 应用 HTTP 配置
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
