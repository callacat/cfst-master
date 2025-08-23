// 请将此文件的内容完整地覆盖到: cmd/main.go

package main

import (
	"log"
	"os"
	"strings"

	"controller/pkg/aggregator"
	"controller/pkg/config"
	"controller/pkg.gist"
	"controller/pkg/models"
	"controller/pkg/selector"
	"controller/pkg/updater"
)

const configFilePath = "config/config.yml"
const resultGistIDFilePath = "config/result_gist_id.txt"

func UpdateAll(selected map[string]models.LineResult, cfg *config.Config) error {
	if !cfg.Huawei.Enabled {
		log.Println("[info] Huawei Cloud updates are disabled in the config, skipping.")
		return nil
	}
	log.Printf("[info] DNS provider is Huawei Cloud. Preparing to update records.")

	lineCfgMap := make(map[string]config.Line)
	for _, lc := range cfg.DNS.Lines {
		lineCfgMap[lc.Operator] = lc
	}

	updateCount := 0
	for key, lineResult := range selected {
		if len(lineResult.Active) == 0 {
			continue
		}

		parts := strings.Split(key, "-")
		operator, ipVersion := parts[0], parts[1]
		lineCfg, ok := lineCfgMap[operator]
		if !ok {
			continue
		}

		var recordsetID string
		if ipVersion == "v4" {
			recordsetID = lineCfg.ARecordsetID
		} else {
			recordsetID = lineCfg.AAAARecordsetID
		}

		if recordsetID == "" {
			continue
		}

		var ips []string
		for _, item := range lineResult.Active {
			ips = append(ips, item.IP)
		}

		log.Printf("[info]     Updating recordset for line '%s' (ID: %s) with IPs: %v", key, recordsetID, ips)
		if err := updater.UpdateHuaweiCloud(recordsetID, ips, cfg); err != nil {
			log.Printf("[error]    => FAILED to update DNS for line %s: %v", key, err)
			return err
		}
		log.Printf("[info]    => SUCCESS for line %s.", key)
		updateCount++
	}

	if updateCount == 0 {
		log.Println("[info] No DNS records needed updating in this run.")
	}
	return nil
}

func main() {
	log.Println("========================================================================")
	log.Println(" M U L T I - N E T   C O N T R O L L E R   S T A R T I N G")
	log.Println("========================================================================")

	cfg, err := config.Load(configFilePath)
	if err != nil {
		log.Fatalf("[error] Failed to load config: %v", err)
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
	if cfg.Huawei.Enabled {
		log.Println("[info] Triggering DNS update process...")
		if err := UpdateAll(selected, cfg); err != nil {
			log.Fatalf("[FATAL] A critical error occurred during DNS update: %v", err)
		}
		log.Println("[info] DNS update process finished.")
	} else {
		log.Println("[info] Huawei Cloud updates are disabled in config. DNS update skipped.")
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
		// [修正] 将 log.Printf 分成多行以避免语法错误
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
