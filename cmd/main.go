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

// AppContext 包含应用程序的共享状态，特别是需要在任务间持久化的 Gist ID
type AppContext struct {
	ResultGistID string
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

// [修改] runTask 现在接收配置和 Gist ID 作为参数，不再依赖外部上下文
func runTask(cfg *config.Config, resultGistID string) string {
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
		return resultGistID
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
		return resultGistID
	}
	log.Println("[PHASE 3 COMPLETE]")

	var newGistID = resultGistID
	if updatesMade > 0 {
		log.Println("\n[PHASE 4] UPLOADING RESULT GIST...")
		filesToUpload := models.BuildResultGistFiles(selected)
		outGistID, err := gc.CreateOrUpdateResultGist(resultGistID, filesToUpload)
		if err != nil {
			log.Fatalf("[FATAL] Failed to push result Gist: %v", err)
		}

		if resultGistID == "" && outGistID != "" {
			newGistID = outGistID
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
	return newGistID
}

func main() {
	log.Println("========================================================================")
	log.Println(" M U L T I - N E T   C O N T R O L L E R   S T A R T I N G")
	log.Println("========================================================================")

	// [修改] 首次加载配置仅用于获取 cron 表达式和启动前检查
	initialCfg, err := config.Load(configFilePath)
	if err != nil {
		log.Fatalf("[error] Failed to load initial config: %v", err)
	}
	
	if initialCfg.Cron.Spec == "" {
		log.Fatalf("[FATAL] 'cron.spec' is not set in config.yml. Please add a valid cron expression to proceed.")
	}

	// [修改] AppContext 仅用于存储需要在任务执行间保持状态的 resultGistID
	appCtx := &AppContext{}

	// 创建 Cron 调度器
	c := cron.New()
	
	// [核心修改] 将所有逻辑（包括配置重载）放入 cron 执行的函数中
	_, err = c.AddFunc(initialCfg.Cron.Spec, func() {
		// --- 1. 热重载配置 ---
		log.Println("[info] Reloading configuration from", configFilePath)
		cfg, err := config.Load(configFilePath)
		if err != nil {
			log.Printf("[error] Failed to reload config, skipping run: %v", err)
			return
		}

		// --- 2. 重新加载 Gist ID 状态 ---
		// 优先使用配置文件中的 Gist ID
		gistID := cfg.Gist.ResultGistID
		if gistID == "" {
			// 如果配置文件为空，则尝试从上下文（上次运行的结果）获取
			gistID = appCtx.ResultGistID
			if gistID == "" {
				// 如果上下文也为空（首次运行），则从本地文件加载
				idBytes, err := os.ReadFile(resultGistIDFilePath)
				if err == nil {
					savedID := strings.TrimSpace(string(idBytes))
					if savedID != "" {
						log.Printf("[info] Found saved result_gist_id from file: %s", savedID)
						gistID = savedID
					}
				}
			}
		}
		
		// --- 3. 执行核心任务 ---
		newGistID := runTask(cfg, gistID)

		// --- 4. 更新状态 ---
		// 将新创建的 Gist ID 保存到上下文中，供下次任务使用
		if newGistID != "" && appCtx.ResultGistID != newGistID {
			log.Printf("[info] Updating Gist ID in context to: %s", newGistID)
			appCtx.ResultGistID = newGistID
		}
	})

	if err != nil {
		log.Fatalf("[FATAL] Invalid cron spec '%s': %v", initialCfg.Cron.Spec, err)
	}
	
	// 启动定时器，并立即触发一次任务
	c.Start()
	log.Printf("Cron scheduler started with spec: '%s'. Triggering initial run...", initialCfg.Cron.Spec)
	
	// 立即执行一次任务
	if len(c.Entries()) > 0 {
		c.Entries()[0].Job.Run()
	}
	
	// 优雅地关闭
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("Shutting down scheduler...")
	<-c.Stop().Done()
	log.Println("Shutdown complete.")
}