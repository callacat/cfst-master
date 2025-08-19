package main

import (
    "log"
    "time"

    "controller/pkg/aggregator"
    "controller/pkg/config"
    "controller/pkg/gist"
    "controller/pkg/models"
    "controller/pkg/selector"
    "controller/pkg/updater"
)

func main() {
    // 1. 加载配置
    cfg, err := config.Load("config.yml")
    if err != nil {
        log.Fatal("加载配置失败:", err)
    }

    // 2. 初始化 Gist 客户端
    gc := gist.NewClient(cfg.Gist.Token)

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
        log.Fatal("未获取到任何设备结果，退出")
    }

    // 4. 聚合打分
    ag := aggregator.Aggregate(allResults)

    // 5. 按线路筛选 Top IP
    selected := selector.SelectTop(ag, cfg.DNS.Lines, cfg.Scoring)

    // 6. 更新 DNS
    if err := updater.UpdateAll(selected, cfg); err != nil {
        log.Fatal("更新 DNS 失败:", err)
    }
    log.Println("DNS 更新完成")

    // 7. 生成结果模型并写入/更新 Gist
    result := models.BuildResult(selected, cfg)
    outGistID, err := gc.CreateOrUpdateResultGist(cfg.Gist.ResultGistID, result)
    if err != nil {
        log.Fatal("推送结果 Gist 失败:", err)
    }
    log.Println("结果已写入 Gist:", outGistID)

    // 8. 退出或等待下次调度
    log.Println("完成，等待下次调度")
    time.Sleep(time.Minute * 10)
}
