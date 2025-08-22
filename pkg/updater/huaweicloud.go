package updater

import (
	"fmt"
	"log"
	"controller/pkg/config"
	// ... other imports
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2/model"
)

// newHuaweiDNSClient creates a Huawei Cloud DNS client
func newHuaweiDNSClient(cfg *config.Config) (*dns.DnsClient, error) {
	auth := basic.NewCredentialsBuilder().
		WithAk(cfg.Huawei.AccessKey).
		WithSk(cfg.Huawei.SecretKey).
		WithProjectId(cfg.Huawei.ProjectID).
		Build()

	r := region.NewRegion(cfg.Huawei.Region, "https://dns.myhuaweicloud.com")

	client := dns.NewDnsClient(
		dns.DnsClientBuilder().
			WithRegion(r).
			WithCredential(auth).
			Build())

	return client, nil
}

// UpdateHuaweiCloud calls the Huawei Cloud DNS API to update records
func UpdateHuaweiCloud(recordsetID string, ips []string, cfg *config.Config) error {
	client, err := newHuaweiDNSClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create Huawei Cloud client: %w", err)
	}

	ttl := int32(cfg.DNS.TTL)
	
	updateReq := &model.UpdateRecordSetRequest{
		RecordsetId: recordsetID, // [修改] 使用传入的 recordsetID
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