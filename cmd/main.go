package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"controller/pkg/aggregator"
	"controller/pkg/config"
	"controller/pkg/gist"
	"controller/pkg/models"
	"controller/pkg/selector"
	"controller/pkg/updater"
)

const configFilePath = "config/config.yml"
const stateFilePath = "config/state.json"

// UpdateAll 函数精简为只处理华为云
func UpdateAll(selected map[string]models.LineResult, cfg *config.Config) error {
	if !cfg.Huawei.Enabled {
		log.Println("[info] Huawei Cloud updates are disabled in the config, skipping.")
		return nil
	}
	log.Println("[info] DNS provider is Huawei Cloud.")

	lineCfgMap := make(map[string]config.Line)
	for _, lc := range cfg.DNS.Lines {
		lineCfgMap[lc.Operator] = lc
	}

	for key, lineResult := range selected {
		if len(lineResult.Active) == 0 {
			log.Printf("[info] Line %s has no active IPs, skipping update", key)
			continue
		}

		parts := strings.Split(key, "-")
		operator, ipVersion := parts[0], parts[1]

		lineCfg, ok := lineCfgMap[operator]
		if !ok {
			log.Printf("[warn] No DNS line configuration found for operator: %s", operator)
			continue
		}

		var recordsetID string
		if ipVersion == "v4" {
			recordsetID = lineCfg.ARecordsetID
		} else if ipVersion == "v6" {
			recordsetID = lineCfg.AAAARecordsetID
		}

		if recordsetID == "" {
			log.Printf("[info] No recordset ID configured for line %s (%s), skipping", key, ipVersion)
			continue
		}

		var ips []string
		for _, item := range lineResult.Active {
			ips = append(ips, item.IP)
		}

		if err := updater.UpdateHuaweiCloud(recordsetID, ips, cfg); err != nil {
			return fmt.Errorf("failed to update DNS for line %s: %w", key, err)
		}
	}
	return nil
}

func main() {
	updateDNS := flag.Bool("update-dns", false, "Set this flag to actually update DNS records")
	flag.Parse()

	cfg, err := config.Load(configFilePath)
	if err != nil {
		log.Fatalf("[error] failed to load config: %v", err)
	}

	state := loadState()
	gc := gist.NewClient(cfg.Gist.Token, cfg.Gist.ProxyPrefix)

	var allResults []models.DeviceResult
	for _, gid := range cfg.Gist.DeviceGists {
		drs, err := gc.FetchDeviceResults(gid, cfg.Gist.MaxResultAgeHours)
		if err != nil {
			log.Printf("[warn] failed to fetch Gist %s: %v", gid, err)
			continue
		}
		allResults = append(allResults, drs...)
	}
	if len(allResults) == 0 {
		log.Fatal("[error] No device results found, exiting")
	}

	ag := aggregator.Aggregate(allResults)
	selected := selector.SelectTop(ag, cfg.DNS.Lines, cfg.Scoring, cfg.Thresholds)

	if *updateDNS {
		log.Println("[info] '--update-dns' flag is set, proceeding with DNS update check.")
		if shouldUpdateDNS(state, cfg) {
			log.Println("[info] Triggering DNS update...")
			if err := UpdateAll(selected, cfg); err != nil {
				log.Fatalf("[error] Failed to update DNS: %v", err)
			}
			log.Println("[info] DNS update process finished.")
			state.LastDNSWrite = time.Now()
			saveState(state)
		} else {
			log.Println("[info] DNS update not required (in cooldown or no better IPs).")
		}
	} else {
		log.Println("[info] '--update-dns' flag not set. Skipping DNS update.")
	}

	result := models.BuildResult(selected, state, cfg)
	outGistID, err := gc.CreateOrUpdateResultGist(cfg.Gist.ResultGistID, result)
	if err != nil {
		log.Fatalf("[error] Failed to push result Gist: %v", err)
	}
	log.Println("[info] Result has been written to Gist:", outGistID)
	log.Println("[info] Current run finished.")
}


// loadState, saveState, shouldUpdateDNS functions remain unchanged
func loadState() *models.State {
	data, err := os.ReadFile(stateFilePath)
	if err != nil {
		log.Println("[warn] state file not found, creating new state")
		return &models.State{}
	}
	var state models.State
	if err := json.Unmarshal(data, &state); err != nil {
		log.Fatalf("[error] failed to parse state file: %v", err)
	}
	return &state
}

func saveState(state *models.State) {
	data, _ := json.MarshalIndent(state, "", "  ")
	if err := os.WriteFile(stateFilePath, data, 0644); err != nil {
		log.Printf("[warn] failed to save state file: %v", err)
	}
}

func shouldUpdateDNS(state *models.State, cfg *config.Config) bool {
	if time.Since(state.LastDNSWrite) < time.Duration(cfg.Selection.CooldownMinutes)*time.Minute {
		return false
	}
	// TODO: Implement hysteresis logic here.
	return true
}