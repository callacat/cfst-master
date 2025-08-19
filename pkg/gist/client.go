package gist

import (
    "bytes"
    "encoding/json"
    "io"
    "net/http"

    "controller/pkg/models"
)
// ...

// Client wraps GitHub Gist API
type Client struct {
    token string
}

func NewClient(token string) *Client {
    return &Client{token: token}
}

// FetchDeviceResults 从指定 gistID 读取默认 文件 (results.json) 并反序列化
func (c *Client) FetchDeviceResults(gistID string) ([]models.DeviceResult, error) {
    url := "https://api.github.com/gists/" + gistID
    req, _ := http.NewRequest("GET", url, nil)
    req.Header.Set("Authorization", "token "+c.token)
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    var gist struct {
        Files map[string]struct{ RawURL string `json:"raw_url"` } `json:"files"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&gist); err != nil {
        return nil, err
    }
    // 假设设备上传到文件名 "results.json"
    raw, ok := gist.Files["results.json"]
    if !ok {
        // 尝试第一个文件
        for _, f := range gist.Files {
            raw = f
            break
        }
    }
    data, err := http.Get(raw.RawURL)
    if err != nil {
        return nil, err
    }
    defer data.Body.Close()
    body, _ := io.ReadAll(data.Body)
    var drs []models.DeviceResult
    if err := json.Unmarshal(body, &drs); err != nil {
        return nil, err
    }
    return drs, nil
}

// CreateOrUpdateResultGist 根据 resultGistID 决定 Create(P=POST) 或 Update(PATCH)
func (c *Client) CreateOrUpdateResultGist(gistID string, fr models.FinalResult) (string, error) {
    // 序列化内容
    content, _ := json.MarshalIndent(fr, "", "  ")
    bodyMap := map[string]interface{}{
        "description": "Multi-Net 优选 IP 结果",
        "public":      false,
        "files": map[string]map[string]string{
            "selected.json": {"content": string(content)},
        },
    }
    bodyBytes, _ := json.Marshal(bodyMap)

    var method, url string
    if gistID == "" {
        method = "POST"
        url = "https://api.github.com/gists"
    } else {
        method = "PATCH"
        url = "https://api.github.com/gists/" + gistID
    }
    req, _ := http.NewRequest(method, url, bytes.NewReader(bodyBytes))
    req.Header.Set("Authorization", "token "+c.token)
    req.Header.Set("Content-Type", "application/json")
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    var respObj struct{ ID string `json:"id"` }
    if err := json.NewDecoder(resp.Body).Decode(&respObj); err != nil {
        return "", err
    }
    return respObj.ID, nil
}
