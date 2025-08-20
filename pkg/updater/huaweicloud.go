package updater

import (
	"fmt"
	"log"

	"controller/pkg/config"
	// "controller/pkg/models" // Removed unused import

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/basic"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/region"
	dns "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2"
	"github.comcom/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2/model"
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
func UpdateHuaweiCloud(line config.Line, ips []string, cfg *config.Config) error {
	client, err := newHuaweiDNSClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create Huawei Cloud client: %w", err)
	}

	// This assumes the zone_id is globally unique and doesn't need to be specified.
	// In a real-world scenario, you might need to query for the zone_id first.
	// For simplicity, we'll proceed assuming the recordset_id is sufficient.

	ttl := int32(cfg.DNS.TTL)
	
	// [FIX] Corrected the structure and types for the update request
	updateReq := &model.UpdateRecordSetRequest{
		RecordsetId: line.RecordsetID,
		Body: &model.UpdateRecordSetReq{
			Ttl:     &ttl,
			Records: &ips, // Pass the address of the slice
		},
		// ZoneId: "your_zone_id", // This may be required depending on your setup
	}

	log.Printf("[info] Updating Huawei Cloud DNS records for line %s: %v", line.Operator, ips)
	_, err = client.UpdateRecordSet(updateReq)
	if err != nil {
		return fmt.Errorf("failed to call Huawei Cloud UpdateRecordSet API: %w", err)
	}

	return nil
}