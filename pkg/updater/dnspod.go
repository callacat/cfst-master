package updater

import (
    "bytes"
    "fmt"
    "net/http"
    "controller/pkg/config"
)

// UpdateAll 为所有线路批量更新
func UpdateAll(sel map[string][]string, cfg *config.Config) error {
    if cfg.DNS.Provider == "dnspod" {
        for _, ln := range cfg.DNS.Lines {
            ips := sel[ln.Operator]
            if len(ips) == 0 {
                continue
            }
            // 只更新同一条 recordset
            if err := updateLine(ln, ips, cfg); err != nil {
                return err
            }
        }
        return nil
    }
    // TODO: 添加华为云分支
    return fmt.Errorf("unsupported dns provider %s", cfg.DNS.Provider)
}

func updateLine(ln config.Line, ips []string, cfg *config.Config) error {
    url := "https://dnsapi.cn/Record.Modify"
    for _, ip := range ips {
        form := fmt.Sprintf(
            "login_token=%s&format=json&domain=%s&record_id=%s"+
                "&sub_domain=%s&record_line=默认&record_type=A&value=%s",
            cfg.Gist.Token, // 改为 cfg.DNSPod.Token
            cfg.DNS.Domain,
            ln.RecordsetID,
            cfg.DNS.Subdomain,
            ip,
        )
        _, err := http.Post(url,
            "application/x-www-form-urlencoded",
            bytes.NewBufferString(form))
        if err != nil {
            return err
        }
    }
    return nil
}
