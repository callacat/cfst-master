// 请将此文件的内容完整地覆盖到: cmd/main.go

package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"controller/pkg/aggregator"
	"controller/pkg/config"
	"controller/pkg/gist"
	"controller/pkg/models"
	"controller/pkg/selector"
	"controller/pkg/updater"

	"github.com/robfig/cron/v3"
)

const configFilePath = "config/config.yml"
const resultGistIDFilePath = "config/result_gist_id.txt"

// AppContext 包含应用程序的共享状态
type AppContext struct {
	Config *config.Config
	GistID string
}

func UpdateAll(selected map[string]models.LineResult, cfg *config.Config) (int, error) {
	if !cfg.Huawei.Enabled {
		log.Println("[info] 华为云更新功能已在配置中禁用, 跳过更新。")
		return 0, nil
	}

	lineCfgMap := make(map[string]config.Line)
	for _, lc := range cfg.DNS.Lines {
		lineCfgMap[lc.Operator] = lc
	}
	
	operatorFriendlyNames := map[string]string{"cm": "中国移动", "cu": "中国联通", "ct": "中国电信"}

	updateCount := 0
	for key, lineResult := range selected {
		if len(lineResult.Active) == 0 {
			continue
		}

		parts := strings.Split(key, "-")
		operatorCode, ipVersion := parts[0], parts[1]
		
		lineCfg, ok := lineCfgMap[operatorCode]
		if !ok {
			log.Printf("[warn] 在 config.yml 中未找到运营商 '%s' 的配置, 跳过。", operatorCode)
			continue
		}

		var recordsetID, recordType string
		if ipVersion == "v4" {
			recordsetID, recordType = lineCfg.ARecordsetID, "A"
		} else {
			recordsetID, recordType = lineCfg.AAAARecordsetID, "AAAA"
		}

		if recordsetID == "" {
			log.Printf("[warn] 运营商 '%s' 的 %s 记录集 ID 为空, 跳过。", operatorCode, recordType)
			continue
		}

		var ipsToUpdate []string
		for _, item := range lineResult.Active {
			ipsToUpdate = append(ipsToUpdate, item.IP)
		}
		
		zoneId := cfg.DNS.ZoneId
		fullRecordName := fmt.Sprintf("%s.%s.", cfg.DNS.Subdomain, cfg.DNS.Domain)
		friendlyName := operatorFriendlyNames[operatorCode]

		log.Printf("[info]     准备更新 [%s-%s] 线路 (运营商: %s) @ %s", friendlyName, ipVersion, operatorCode, time.Now().Format("15:04:05"))
		log.Printf("[info]     => 记录名: %s, 记录集ID: %s", fullRecordName, recordsetID)
		
		err := updater.UpdateHuaweiCloud(zoneId, recordsetID, fullRecordName, recordType, ipsToUpdate, cfg)
		if err != nil {
			log.Printf("[error]    => 更新失败: %v", err)
			return updateCount, err
		}
		
		log.Printf("[info]    => 成功更新 %d 个IP: %v", len(ipsToUpdate), ipsToUpdate)
		updateCount++
	}

	if updateCount == 0 {
		log.Println("[info] 本次运行没有需要更新的 DNS 记录。")
	}
	return updateCount, nil
}

// [重构] runTask 封装了单次执行的核心逻辑
func runTask(appCtx *AppContext) {
	cfg := appCtx.Config
	log.Println("========================================================================")
	log.Printf(" [ %s ] R U N N I N G   T A S K", time.Now().Format(time.RFC1123))
	log.Println("========================================================================")
	
	gc := gist.NewClient(cfg.Gist.Token, cfg.Gist.ProxyPrefix)

	log.Println("\n[PHASE 1] FETCHING DEVICE RESULTS...")
	var allResults []models.DeviceResult
	for _, gid := range cfg.Gist.DeviceGists {
		drs, err := gc.FetchDeviceResults(gid, cfg.Gist.GistUpdateCheckMinutes)
		if err != nil {
			log.Printf("[warn] Could not process Gist %s due to an error: %v", gid, err)
			continue
		}
		allResults = append(allResults, drs...)
	}

	if len(allResults) == 0 {
		log.Println("[info] 在设定的时间范围内没有找到任何更新的 Gist 或有效结果。任务结束。")
		log.Println("============================ T A S K   F I N I S H E D ============================\n")
		return
	}
	log.Printf("[PHASE 1 COMPLETE] Fetched a total of %d valid results from recently updated Gists.", len(allResults))

	log.Println("\n[PHASE 2] AGGREGATING & SELECTING TOP IPs...")
	ag := aggregator.Aggregate(allResults)
	log.Printf("[info] Aggregated results into %d groups (e.g., 'cu-v4').", len(ag))
	selected := selector.SelectTop(ag, cfg.DNS.Lines, cfg.Scoring, cfg.Thresholds)
	log.Println("[PHASE 2 COMPLETE] Finished selecting top IPs.")

	log.Println("\n[PHASE 3] PROCESSING DNS UPDATES...")
	updatesMade, err := UpdateAll(selected, cfg)
	if err != nil {
		log.Printf("[FATAL] A critical error occurred during DNS update: %v", err)
		return
	}
	log.Println("[PHASE 3 COMPLETE]")

	if updatesMade > 0 {
		log.Println("\n[PHASE 4] UPLOADING RESULT GIST...")
		filesToUpload := models.BuildResultGistFiles(selected)
		originalGistID := appCtx.GistID
		outGistID, err := gc.CreateOrUpdateResultGist(originalGistID, filesToUpload)
		if err != nil {
			log.Fatalf("[FATAL] Failed to push result Gist: %v", err)
		}

		if originalGistID == "" && outGistID != "" {
			appCtx.GistID = outGistID
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
	} else {
		log.Println("\n[PHASE 4] SKIPPED: No DNS updates were made, so result Gist was not updated.")
	}
	log.Println("============================ T A S K   F I N I S H E D ============================\n")
}

func main() {
	log.Println("========================================================================")
	log.Println(" M U L T I - N E T   C O N T R O L L E R   S T A R T I N G")
	log.Println("========================================================================")

	cfg, err := config.Load(configFilePath)
	if err != nil {
		log.Fatalf("[error] Failed to load config: %v", err)
	}
	
	if cfg.Cron.Spec == "" {
		log.Fatalf("[FATAL] 'cron.spec' is not set in config.yml. Please add a valid cron expression to proceed.")
	}

	// 准备应用上下文
	appCtx := &AppContext{Config: cfg}
	
	// 加载 Gist ID
	gistID := cfg.Gist.ResultGistID
	if gistID == "" {
		idBytes, err := os.ReadFile(resultGistIDFilePath)
		if err == nil {
			savedID := strings.TrimSpace(string(idBytes))
			if savedID != "" {
				log.Printf("[info] Found saved result_gist_id: %s", savedID)
				gistID = savedID
			}
		}
	}
	appCtx.GistID = gistID

	// 设置 Cron 调度器
	c := cron.New()
	_, err = c.AddFunc(cfg.Cron.Spec, func() { runTask(appCtx) })
	if err != nil {
		log.Fatalf("[FATAL] Invalid cron spec '%s': %v", cfg.Cron.Spec, err)
	}
	
	// 立即执行一次任务，然后启动定时器
	go runTask(appCtx)
	c.Start()
	
	log.Printf("Cron scheduler started with spec: '%s'. Waiting for jobs...", cfg.Cron.Spec)
	
	// 优雅地关闭
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("Shutting down scheduler...")
	<-c.Stop().Done()
	log.Println("Shutdown complete.")
}