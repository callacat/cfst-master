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

// newHuaweiDNSClient creates a configured Huawei Cloud DNS service client.
func newHuaweiDNSClient(cfg *config.Config) (*dns.DnsClient, error) {
	// 1. Build authentication credentials
	auth := basic.NewCredentialsBuilder().
		WithAk(cfg.Huawei.AccessKey).
		WithSk(cfg.Huawei.SecretKey).
		WithProjectId(cfg.Huawei.ProjectID).
		Build()

	// 2. Safely build the service region information using the SDK's region functionality.
	r, err := dnsRegion.SafeValueOf(cfg.Huawei.Region)
	if err != nil {
		return nil, fmt.Errorf("invalid or unsupported region '%s' specified: %w", cfg.Huawei.Region, err)
	}
	log.Printf("[debug] Using Huawei Cloud DNS region: %s", cfg.Huawei.Region)

	// 3. Configure the HTTP client, for example, by setting a timeout.
	httpConfig := coreCfg.DefaultHttpConfig().
		WithTimeout(60 * time.Second)

	// 4. Use the builder pattern to create the final client instance.
	client := dns.NewDnsClient(
		dns.DnsClientBuilder().
			WithRegion(r).
			WithCredential(auth).
			WithHttpConfig(httpConfig).
			Build())

	return client, nil
}

// UpdateHuaweiCloud updates a single DNS record set based on the provided parameters.
// This function calls Huawei Cloud's "UpdateRecordSet" API (singular), which is the correct
// method for modifying an existing record set by its unique ID.
func UpdateHuaweiCloud(recordsetID string, ips []string, cfg *config.Config) error {
	// Step 1: Initialize the DNS client
	client, err := newHuaweiDNSClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create Huawei Cloud client: %w", err)
	}

	// Step 2: Prepare the API request body.
	// For this API, we only need to provide the parameters being changed: TTL and the IP records.
	ttl := int32(cfg.DNS.TTL)
	updateBody := &model.UpdateRecordSetReq{
		Ttl:     &ttl,
		Records: &ips, // 'records' is a slice of strings, where each element is an IP address.
	}

	// Step 3: Build the complete request object for the singular update operation.
	updateReq := &model.UpdateRecordSetRequest{
		RecordsetId: recordsetID,
		Body:        updateBody,
	}

	// Step 4: Execute the API call
	log.Printf("[info] Updating Huawei Cloud DNS records for recordset %s with IPs: %v", recordsetID, ips)
	_, err = client.UpdateRecordSet(updateReq)
	if err != nil {
		return fmt.Errorf("failed to call Huawei Cloud UpdateRecordSet API for recordset %s: %w", recordsetID, err)
	}

	log.Printf("[info] Successfully updated recordset %s.", recordsetID)
	return nil
}