package typed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/yxxchange/pipefree/helper/log"
	"github.com/yxxchange/pipefree/infra/dal/model"
)

// Update 通过服务端 API 更新资源
func (c *nodeExecClient) Update(ctx context.Context, nodeExec *model.NodeExec) (*model.NodeExec, error) {
	url := fmt.Sprintf("%s/pipe_exec/update", c.baseURL)
	
	log.Infof("🔄 Client Update: id=%d, name=%s, phase=%s", 
		nodeExec.Id, nodeExec.Name, nodeExec.Phase.Phase)
	log.Infof("📡 Calling API: %s", url)
	
	body, err := json.Marshal(nodeExec)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal node exec: %w", err)
	}
	
	log.Debugf("📤 Request body: %s", string(body))
	
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Errorf("❌ HTTP request failed: %v", err)
		return nil, fmt.Errorf("failed to call server API: %w", err)
	}
	defer resp.Body.Close()
	
	log.Infof("📥 HTTP response status: %d", resp.StatusCode)
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}
	
	var response struct {
		Code   int             `json:"code"`
		ErrMsg string          `json:"err_msg"`
		Info   *model.NodeExec `json:"info"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	log.Infof("📋 Server response: code=%d, err_msg=%s", response.Code, response.ErrMsg)
	
	if response.Code != 0 {
		return nil, fmt.Errorf("server error: %s", response.ErrMsg)
	}
	
	log.Infof("✅ Client Update successful")
	return response.Info, nil
}

// Delete 通过服务端 API 删除资源
func (c *nodeExecClient) Delete(ctx context.Context, namespace, kind string, id int64) error {
	url := fmt.Sprintf("%s/pipe_exec/delete?namespace=%s&kind=%s&id=%d", c.baseURL, namespace, kind, id)
	
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call server API: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}
	
	var response struct {
		Code   int    `json:"code"`
		ErrMsg string `json:"err_msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	
	if response.Code != 0 {
		return fmt.Errorf("server error: %s", response.ErrMsg)
	}
	
	return nil
}