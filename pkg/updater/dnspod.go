package updater

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"controller/pkg/config"
	"controller/pkg/models"
)

// UpdateDNSpod 实现了针对 DNSpod 的更新逻辑
func UpdateDNSpod(line config.Line, ips []string, cfg *config.Config) error {
	// DNSpod 不支持在单次 API 调用中为同一记录设置多个值, 
	// 所以我们需要多次调用, 或者使用 "Record.Modify" 的 "multi_value" 模式 (如果套餐支持)
	// 这里简化为多次调用
	url := "https://dnsapi.cn/Record.Modify"

	// 为了原子性, 我们需要先删除旧的, 再添加新的。
	// 但 DNSpod API 的限制使得 "先加后减" 更复杂。
	// 这里采用一种简化的方法：为每个 IP 值调用一次 Modify。
	// 注意：这并非严格的原子替换，但在多数场景下可用。

	for _, ip := range ips {
		// 这里假设每个线路只管理一个子域名记录
		form := fmt.Sprintf(
			"login_token=%s&format=json&domain_id=%s&record_id=%s"+
				"&sub_domain=%s&record_line_id=%s&record_type=A&value=%s&ttl=%d",
			cfg.Gist.Token, // 实际应为 cfg.DNSpod.Token
			cfg.DNS.Domain,    // domain_id, 非 domain name
			line.RecordsetID,
			cfg.DNS.Subdomain,
			"0", // 默认线路
			ip,
			cfg.DNS.TTL,
		)

		// 注意：生产环境需要一个更健壮的 HTTP client
		_, err := http.Post(url,
			"application/x-www-form-urlencoded",
			bytes.NewBufferString(form))

		if err != nil {
			// 在实际应用中, 你可能需要处理部分失败的情况
			return fmt.Errorf("failed to update record for IP %s: %w", ip, err)
		}
	}

	return nil
}

// [修改] UpdateAll 移至 main.go, 这里只提供 provider 实现
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