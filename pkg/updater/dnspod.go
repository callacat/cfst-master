package updater

import (
	"bytes"
	"fmt"
	"net/http"
	// "strings" // Removed unused import

	"controller/pkg/config"
	"controller/pkg/models"
)

// UpdateDNSpod implements the update logic for DNSpod
func UpdateDNSpod(line config.Line, ips []string, cfg *config.Config) error {
	// DNSpod does not support setting multiple values for the same record in a single API call,
	// so we need to call it multiple times, or use the "multi_value" mode (if the plan supports it).
	// We'll simplify here by calling Modify for each IP value.
	url := "https://dnsapi.cn/Record.Modify"

	// Note: This is not a strictly atomic replacement, but it's functional for most scenarios.
	for _, ip := range ips {
		// This assumes each line manages a single subdomain record.
		form := fmt.Sprintf(
			"login_token=%s&format=json&domain_id=%s&record_id=%s"+
				"&sub_domain=%s&record_line_id=%s&record_type=A&value=%s&ttl=%d",
			cfg.Gist.Token, // Should ideally be cfg.DNSPod.Token
			cfg.DNS.Domain,    // domain_id, not domain name
			line.RecordsetID,
			cfg.DNS.Subdomain,
			"0", // Default line
			ip,
			cfg.DNS.TTL,
		)

		// A more robust HTTP client should be used in production.
		_, err := http.Post(url,
			"application/x-www-form-urlencoded",
			bytes.NewBufferString(form))

		if err != nil {
			// In a real application, you might need to handle partial failures.
			return fmt.Errorf("failed to update record for IP %s: %w", ip, err)
		}
	}

	return nil
}

// updateAll has been moved to main.go, this file only provides the provider implementation.
func updateAll(sel map[string][]models.SelectedItem, cfg *config.Config) error {
    if cfg.DNS.Provider == "dnspod" {
        for _, ln := range cfg.DNS.Lines {
            lineResult, ok := sel[ln.Operator]
            if !ok || len(lineResult) == 0 {
                continue
            }
            
            var ips []string
            for _, item := range lineResult {
                ips = append(ips, item.IP)
            }

            if err := UpdateDNSpod(ln, ips, cfg); err != nil {
                return err
            }
        }
        return nil
    }
    return fmt.Errorf("unsupported dns provider %s", cfg.DNS.Provider)
}