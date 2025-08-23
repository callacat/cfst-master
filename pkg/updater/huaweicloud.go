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
	dnsRegion "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2/region"
)

// newHuaweiDNSClient remains the same, it correctly creates the client.
func newHuaweiDNSClient(cfg *config.Config) (*dns.DnsClient, error) {
	auth := basic.NewCredentialsBuilder().
		WithAk(cfg.Huawei.AccessKey).
		WithSk(cfg.Huawei.SecretKey).
		WithProjectId(cfg.Huawei.ProjectID).
		Build()

	r, err := dnsRegion.SafeValueOf(cfg.Huawei.Region)
	if err != nil {
		return nil, fmt.Errorf("invalid or unsupported region '%s' specified: %w", cfg.Huawei.Region, err)
	}

	httpConfig := coreCfg.DefaultHttpConfig().
		WithTimeout(60 * time.Second)

	client := dns.NewDnsClient(
		dns.DnsClientBuilder().
			WithRegion(r).
			WithCredential(auth).
			WithHttpConfig(httpConfig).
			Build())

	return client, nil
}


// [REFACTORED] UpdateHuaweiCloud now uses the UpdateRecordSets (plural) method.
func UpdateHuaweiCloud(zoneId, recordsetID, recordName, recordType string, ips []string, cfg *config.Config) error {
	client, err := newHuaweiDNSClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create Huawei Cloud client: %w", err)
	}

	// Build the request object based on the successful debug code.
	request := &model.UpdateRecordSetsRequest{}
	request.ZoneId = zoneId
	request.RecordsetId = recordsetID
	request.Body = &model.UpdateRecordSetsReq{
		Name:    recordName,
		Type:    recordType,
		Ttl:     &cfg.DNS.TTL,
		Records: &ips,
	}

	log.Printf("[info] Updating DNS using 'UpdateRecordSets' for %s (ID: %s) with IPs: %v", recordName, recordsetID, ips)
	_, err = client.UpdateRecordSets(request)
	if err != nil {
		return fmt.Errorf("failed to call Huawei Cloud UpdateRecordSets API: %w", err)
	}

	return nil
}