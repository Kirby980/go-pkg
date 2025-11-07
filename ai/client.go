package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL           string
	apiKey            string
	model             string
	httpClient        *http.Client
	fallbackModels    []string // 备选模型列表
	currentModelIndex int      // 当前使用的模型索引
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

type ChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int         `json:"index"`
		Message ChatMessage `json:"message"`
		Finish  string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	ErrorString string `json:"error,omitempty"` // 支持字符串格式的错误
	ErrorObj    *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"-"` // 不再直接从JSON解析
}

// GetErrorMessage 获取错误信息（兼容两种格式）
func (r *ChatResponse) GetErrorMessage() string {
	if r.ErrorString != "" {
		return r.ErrorString
	}
	if r.ErrorObj != nil {
		return r.ErrorObj.Message
	}
	return ""
}

// HasError 检查是否有错误
func (r *ChatResponse) HasError() bool {
	return r.ErrorString != "" || r.ErrorObj != nil
}

// NewClient 创建新的AI客户端
func NewClient(baseURL, apiKey, model string) *Client {
	var fallbackModels []string
	if strings.Contains(baseURL, "siliconflow") {
		fallbackModels = SiliconFlowModels
	}

	return &Client{
		baseURL:           baseURL,
		apiKey:            apiKey,
		model:             model,
		fallbackModels:    fallbackModels,
		currentModelIndex: -1, // 初始使用配置的模型
		httpClient: &http.Client{
			Timeout: 180 * time.Second, // 增加到3分钟，确保有足够的响应时间
			Transport: &http.Transport{
				MaxIdleConns:        100,
				IdleConnTimeout:     90 * time.Second,
				DisableCompression:  false,
				TLSHandshakeTimeout: 10 * time.Second,
			},
		},
	}
}

// Chat 发送聊天请求到AI服务（支持多轮对话）
func (c *Client) Chat(ctx context.Context, messages []ChatMessage) (*ChatResponse, error) {
	// 转换消息格式
	apiMessages := make([]ChatMessage, 0, len(messages))
	for _, msg := range messages {
		apiMessages = append(apiMessages, ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// 如果没有备选模型，使用原有逻辑
	if len(c.fallbackModels) == 0 {
		req := ChatRequest{
			Model:       c.model,
			Messages:    apiMessages,
			Temperature: 0.7,
			MaxTokens:   1000,
		}
		return c.sendChatRequest(ctx, req)
	}

	// 有备选模型，尝试使用当前模型和备选模型
	var lastErr error
	startIndex := c.currentModelIndex

	// 如果是第一次调用，先尝试配置的模型
	if startIndex == -1 {
		req := ChatRequest{
			Model:       c.model,
			Messages:    apiMessages,
			Temperature: 0.7,
			MaxTokens:   1000,
		}

		resp, err := c.sendChatRequest(ctx, req)
		if err == nil && !resp.HasError() {
			return resp, nil
		}

		lastErr = err
		if resp != nil && resp.HasError() {
			lastErr = fmt.Errorf("API error: %s", resp.GetErrorMessage())
		}

		log.Printf("Primary model %s failed: %v, trying fallback models...", c.model, lastErr)
		startIndex = 0
	}

	// 尝试备选模型
	for i := startIndex; i < len(c.fallbackModels); i++ {
		model := c.fallbackModels[i]
		req := ChatRequest{
			Model:       model,
			Messages:    apiMessages,
			Temperature: 0.7,
			MaxTokens:   1000,
		}

		log.Printf("Trying fallback model: %s", model)
		resp, err := c.sendChatRequest(ctx, req)

		if err == nil && !resp.HasError() {
			// 成功了，记住这个模型索引以便下次直接使用
			c.currentModelIndex = i
			log.Printf("Successfully using model: %s", model)
			return resp, nil
		}

		if err != nil {
			lastErr = err
		} else if resp.HasError() {
			lastErr = fmt.Errorf("API error with model %s: %s", model, resp.GetErrorMessage())
		}

		log.Printf("Model %s failed: %v", model, lastErr)
	}

	return nil, fmt.Errorf("all models failed, last error: %v", lastErr)
}

// ChatWithText 简化的文本聊天接口
func (c *Client) ChatWithText(ctx context.Context, text string) (string, error) {
	messages := []ChatMessage{
		{
			Role:    "user",
			Content: text,
		},
	}

	resp, err := c.Chat(ctx, messages)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from AI service")
	}

	return resp.Choices[0].Message.Content, nil
}

// sendChatRequest 发送HTTP请求到AI服务，带有重试机制
func (c *Client) sendChatRequest(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	var lastErr error
	maxRetries := 1                    // 减少到1次重试（总共2次尝试），因为我们有多个模型可以尝试且超时时间已增加
	backoffDuration := 2 * time.Second // 重试前等待2秒

	for retry := 0; retry <= maxRetries; retry++ {
		if retry > 0 {
			waitTime := backoffDuration * time.Duration(1<<(retry-1))
			time.Sleep(waitTime)
		}

		requestURL := c.baseURL + "/chat/completions"
		httpReq, err := http.NewRequestWithContext(ctx, "POST", requestURL, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
		httpReq.Header.Set("User-Agent", "webook-ai-client/1.0")
		httpReq.Header.Set("Connection", "close")

		startTime := time.Now()
		log.Printf("Sending AI request to %s with model %s (attempt %d/%d)", requestURL, req.Model, retry+1, maxRetries+1)

		resp, err := c.httpClient.Do(httpReq)
		elapsed := time.Since(startTime)

		if err != nil {
			lastErr = fmt.Errorf("send request to %s (attempt %d/%d): %w", requestURL, retry+1, maxRetries+1, err)
			log.Printf("Request failed after %.2f seconds: %v", elapsed.Seconds(), lastErr)
			continue
		}

		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		var chatResp ChatResponse
		if err := json.Unmarshal(body, &chatResp); err != nil {
			lastErr = fmt.Errorf("decode response from %s (attempt %d/%d): %w, body: %s", requestURL, retry+1, maxRetries+1, err, string(body))
			log.Printf("Response decode error: %v", lastErr)
			continue
		}

		// 即使状态码不是200，也返回响应让上层处理
		if resp.StatusCode != http.StatusOK {
			log.Printf("API request to %s returned status %d for model %s: %s", requestURL, resp.StatusCode, req.Model, string(body))
			// 对于客户端错误（4xx），不重试
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				return &chatResp, nil
			}
			continue
		}

		log.Printf("Successfully received response from %s for model %s in %.2f seconds", requestURL, req.Model, elapsed.Seconds())
		return &chatResp, nil
	}

	return nil, lastErr
}
