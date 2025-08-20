package main

import (
	"encoding/json"
	"fmt" // Add this line
	"log"
	"os"
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

func UpdateAll(selected map[string]models.LineResult, cfg *config.Config) error {
	provider := cfg.DNS.Provider
	log.Printf("[info] DNS 提供商: %s", provider)

	for _, lineCfg := range cfg.DNS.Lines {
		lineResult, ok := selected[lineCfg.Operator]
		if !ok || len(lineResult.Active) == 0 {
			log.Printf("[info] 线路 %s 无可用 IP，跳过更新", lineCfg.Operator)
			continue
		}

		var ips []string
		for _, item := range lineResult.Active {
			ips = append(ips, item.IP)
		}

		var err error
		switch provider {
		case "dnspod":
			err = updater.UpdateDNSpod(lineCfg, ips, cfg)
		case "huawei":
			err = updater.UpdateHuaweiCloud(lineCfg, ips, cfg) // [新增] 调用华为云更新
		default:
			return fmt.Errorf("不支持的 DNS 提供商: %s", provider)
		}

		if err != nil {
			return fmt.Errorf("为线路 %s 更新 DNS 失败: %w", lineCfg.Operator, err)
		}
	}
	return nil
}

func main() {
	// 1. 加载配置
	cfg, err := config.Load(configFilePath)
	if err != nil {
		log.Fatalf("[error] 加载配置失败: %v", err)
	}

	// [新增] 加载上次状态
	state := loadState()

	// 2. 初始化 Gist 客户端 (传入代理配置)
	gc := gist.NewClient(cfg.Gist.Token, cfg.Gist.ProxyPrefix)

	// 3. 拉取所有设备结果
	var allResults []models.DeviceResult
	for _, gid := range cfg.Gist.DeviceGists {
		drs, err := gc.FetchDeviceResults(gid)
		if err != nil {
			log.Printf("[warn] 获取 Gist %s 失败: %v", gid, err)
			continue
		}
		allResults = append(allResults, drs...)
	}
	if len(allResults) == 0 {
		log.Fatal("[error] 未获取到任何设备结果，退出")
	}

	// 4. 聚合打分
	ag := aggregator.Aggregate(allResults)

	// 5. 按线路筛选 Top IP (传入阈值)
	selected := selector.SelectTop(ag, cfg.DNS.Lines, cfg.Scoring, cfg.Thresholds)

	// [新增] 6. 检查是否需要更新 DNS (冷却期和滞回判断)
	if shouldUpdateDNS(state, cfg) {
		log.Println("[info] 触发 DNS 更新...")
		if err := UpdateAll(selected, cfg); err != nil { // [修改]
			log.Fatalf("[error] 更新 DNS 失败: %v", err)
		}
		log.Println("[info] DNS 更新完成")
		state.LastDNSWrite = time.Now()
		saveState(state)
	} else {
		log.Println("[info] 无需更新 DNS (冷却中或无更优 IP)")
	}

	// 7. 生成结果模型并写入/更新 Gist
	result := models.BuildResult(selected, state, cfg)
	outGistID, err := gc.CreateOrUpdateResultGist(cfg.Gist.ResultGistID, result)
	if err != nil {
		log.Fatalf("[error] 推送结果 Gist 失败: %v", err)
	}
	log.Println("[info] 结果已写入 Gist:", outGistID)

	log.Println("[info] 本次调度完成")
}

// [新增]
func loadState() *models.State {
	data, err := os.ReadFile(stateFilePath)
	if err != nil {
		log.Println("[warn] 状态文件不存在, 将创建新的状态")
		return &models.State{}
	}
	var state models.State
	if err := json.Unmarshal(data, &state); err != nil {
		log.Fatalf("[error] 解析状态文件失败: %v", err)
	}
	return &state
}

// [新增]
func saveState(state *models.State) {
	data, _ := json.MarshalIndent(state, "", "  ")
	if err := os.WriteFile(stateFilePath, data, 0644); err != nil {
		log.Printf("[warn] 保存状态文件失败: %v", err)
	}
}

// [新增]
func shouldUpdateDNS(state *models.State, cfg *config.Config) bool {
	// 检查冷却期
	if time.Since(state.LastDNSWrite) < time.Duration(cfg.Selection.CooldownMinutes)*time.Minute {
		return false
	}
	// TODO: 在这里实现更复杂的滞回逻辑,
	// 例如比较 `selected` 的 IP 与上次 Gist 中的 `active` IP 的平均分数。
	// 为简化, 此处仅实现了冷却期判断。
	return true
}