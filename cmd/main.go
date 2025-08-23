// 请将此文件的内容完整地覆盖到: cmd/main.go

package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"controller/pkg/aggregator"
	"controller/pkg/config"
	"controller/pkg/gist"
	"controller/pkg/models"
	"controller/pkg/selector"
	"controller/pkg/updater"
)

const configFilePath = "config/config.yml"
const resultGistIDFilePath = "config/result_gist_id.txt"

// [优化可读性] UpdateAll 函数现在包含更清晰的日志和变量名
func UpdateAll(selected map[string]models.LineResult, cfg *config.Config) error {
	if !cfg.Huawei.Enabled {
		log.Println("[info] 华为云更新功能已在配置中禁用, 跳过更新。")
		return nil
	}
	log.Printf("[info] DNS 提供商为华为云, 准备更新解析记录。")

	// 创建一个从 operator 代码到其配置的映射，方便快速查找
	lineCfgMap := make(map[string]config.Line)
	for _, lc := range cfg.DNS.Lines {
		lineCfgMap[lc.Operator] = lc
	}
	
	// [新增] 创建一个映射，用于在日志中显示更友好的运营商名称
	operatorFriendlyNames := map[string]string{
		"cm": "中国移动",
		"cu": "中国联通",
		"ct": "中国电信",
	}

	updateCount := 0
	// 遍历已筛选出的结果 (例如 "cu-v4", "cm-v6" 等)
	for key, lineResult := range selected {
		if len(lineResult.Active) == 0 {
			continue // 如果没有筛选出活动的IP，则跳过
		}

		parts := strings.Split(key, "-")
		operatorCode, ipVersion := parts[0], parts[1]
		
		// 从配置中查找此运营商的线路设置
		lineCfg, ok := lineCfgMap[operatorCode]
		if !ok {
			log.Printf("[warn] 在 config.yml 中未找到运营商 '%s' 的配置, 跳过。", operatorCode)
			continue
		}

		var recordsetID, recordType string
		if ipVersion == "v4" {
			recordsetID = lineCfg.ARecordsetID
			recordType = "A"
		} else { // ipVersion == "v6"
			recordsetID = lineCfg.AAAARecordsetID
			recordType = "AAAA"
		}

		if recordsetID == "" {
			log.Printf("[warn] 运营商 '%s' 的 %s 记录集 ID 为空, 跳过。", operatorCode, recordType)
			continue
		}

		// 准备要更新的 IP 列表
		var ipsToUpdate []string
		for _, item := range lineResult.Active {
			ipsToUpdate = append(ipsToUpdate, item.IP)
		}
		
		// 准备更新所需的其他参数
		zoneId := cfg.DNS.ZoneId
		fullRecordName := fmt.Sprintf("%s.%s.", cfg.DNS.Subdomain, cfg.DNS.Domain)
		friendlyName := operatorFriendlyNames[operatorCode] // 获取友好的中文名

		log.Printf("[info]     准备更新 [%s-%s] 线路 (运营商: %s)", friendlyName, ipVersion, operatorCode)
		log.Printf("[info]     => 记录名: %s, 记录集ID: %s", fullRecordName, recordsetID)
		
		// 调用更新函数
		err := updater.UpdateHuaweiCloud(zoneId, recordsetID, fullRecordName, recordType, ipsToUpdate, cfg)
		if err != nil {
			log.Printf("[error]    => 更新失败: %v", err)
			return err // 如果单次更新失败，则终止整个流程
		}
		
		log.Printf("[info]    => 成功更新 %d 个IP: %v", len(ipsToUpdate), ipsToUpdate)
		updateCount++
	}

	if updateCount == 0 {
		log.Println("[info] 本次运行没有需要更新的 DNS 记录。")
	}
	return nil
}

// main 函数保持不变
func main() {
	log.Println("========================================================================")
	log.Println(" M U L T I - N E T   C O N T R O L L E R   S T A R T I N G")
	log.Println("========================================================================")

	cfg, err := config.Load(configFilePath)
	if err != nil {
		log.Fatalf("[error] Failed to load config: %v", err)
	}

	if cfg.DNS.ZoneId == "" || cfg.DNS.ZoneId == "YOUR_ZONE_ID_HERE" {
		log.Fatalf("[FATAL] 'dns.zone_id' is not set in config.yml. Please add your Zone ID to proceed.")
	}

	if cfg.Gist.ResultGistID == "" {
		log.Println("[info] result_gist_id is not set in config.yml, trying to load from local file...")
		idBytes, err := os.ReadFile(resultGistIDFilePath)
		if err == nil {
			savedID := strings.TrimSpace(string(idBytes))
			if savedID != "" {
				log.Printf("[info] Found saved result_gist_id: %s", savedID)
				cfg.Gist.ResultGistID = savedID
			}
		} else {
			log.Printf("[info] No saved result_gist_id file found. A new Gist will be created if needed.")
		}
	}

	gc := gist.NewClient(cfg.Gist.Token, cfg.Gist.ProxyPrefix)

	log.Println("\n[PHASE 1] FETCHING DEVICE RESULTS...")
	var allResults []models.DeviceResult
	for _, gid := range cfg.Gist.DeviceGists {
		drs, err := gc.FetchDeviceResults(gid, cfg.Gist.MaxResultAgeHours)
		if err != nil {
			log.Printf("[warn] Could not process Gist %s due to an error.", gid)
			continue
		}
		allResults = append(allResults, drs...)
	}
	if len(allResults) == 0 {
		log.Fatal("[FATAL] No valid device results found. Cannot continue.")
	}
	log.Printf("[PHASE 1 COMPLETE] Fetched a total of %d valid results.", len(allResults))

	log.Println("\n[PHASE 2] AGGREGATING & SELECTING TOP IPs...")
	ag := aggregator.Aggregate(allResults)
	log.Printf("[info] Aggregated results into %d groups (e.g., 'cu-v4').", len(ag))
	selected := selector.SelectTop(ag, cfg.DNS.Lines, cfg.Scoring, cfg.Thresholds)
	log.Println("[PHASE 2 COMPLETE] Finished selecting top IPs.")

	log.Println("\n[PHASE 3] PROCESSING DNS UPDATES...")
	if err := UpdateAll(selected, cfg); err != nil {
		log.Fatalf("[FATAL] A critical error occurred during DNS update: %v", err)
	}
	log.Println("[PHASE 3 COMPLETE]")

	log.Println("\n[PHASE 4] UPLOADING RESULT GIST...")
	result := models.BuildResult(selected, cfg)
	originalGistID := cfg.Gist.ResultGistID
	outGistID, err := gc.CreateOrUpdateResultGist(cfg.Gist.ResultGistID, result)
	if err != nil {
		log.Fatalf("[FATAL] Failed to push result Gist: %v", err)
	}

	if originalGistID == "" && outGistID != "" {
		log.Println("------------------------------------------------------------------------")
		log.Printf("[ACTION REQUIRED] New Result Gist created with ID: %s", outGistID)
		log.Printf("                  It is recommended to add this ID to your 'config.yml'.")
		log.Printf("                  (ID has been saved to %s for auto-loading)", resultGistIDFilePath)
		log.Println("------------------------------------------------------------------------")
		if err := os.WriteFile(resultGistIDFilePath, []byte(outGistID), 0644); err != nil {
			log.Printf("[warn] Failed to save result_gist_id to local file: %v", err)
		}
	}
	log.Printf("[PHASE 4 COMPLETE] Result written to Gist: %s", outGistID)

	log.Println("\n========================================================================")
	log.Println(" R U N   F I N I S H E D")
	log.Println("========================================================================")
}