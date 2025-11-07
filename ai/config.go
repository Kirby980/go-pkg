package ai

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Model         string                    `toml:"model"`
	ModelProvider string                    `toml:"model_provider"`
	AuthMethod    string                    `toml:"preferred_auth_method"`
	Providers     map[string]ProviderConfig `toml:"model_providers"`
}

type ProviderConfig struct {
	Name    string `toml:"name"`
	BaseURL string `toml:"base_url"`
	WireAPI string `toml:"wire_api"`
}

type AuthConfig struct {
	OpenAIAPIKey string `json:"OPENAI_API_KEY"`
}

// SiliconFlow推荐模型列表，按优先级排序
var SiliconFlowModels = []string{
	"Qwen/Qwen3-8B", // 阿里千问3代8B
	"deepseek-ai/DeepSeek-R1-Distill-Qwen-7B", // DeepSeek推理优化
	"deepseek-ai/DeepSeek-R1-0528-Qwen3-8B",   // DeepSeek R1最新版
	"Qwen/Qwen2-7B-Instruct",                  // 千问2代7B
	"tencent/Hunyuan-MT-7B",                   // 腾讯混元多任务
	"THUDM/GLM-4.1V-9B-Thinking",              // 智谱GLM思维链
	"deepseek-ai/DeepSeek-OCR",                // DeepSeek OCR
}

// LoadConfig 从默认位置加载配置
func LoadConfig() (*Config, *AuthConfig, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, fmt.Errorf("get home dir: %w", err)
	}

	// 加载主配置
	configPath := filepath.Join(homeDir, ".codex", "config.toml")
	var config Config
	if _, err := toml.DecodeFile(configPath, &config); err != nil {
		return nil, nil, fmt.Errorf("decode config file: %w", err)
	}

	// 加载认证配置
	authPath := filepath.Join(homeDir, ".codex", "auth.json")
	authData, err := os.ReadFile(authPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read auth file: %w", err)
	}

	var authConfig AuthConfig
	if err := json.Unmarshal(authData, &authConfig); err != nil {
		return nil, nil, fmt.Errorf("decode auth file: %w", err)
	}

	return &config, &authConfig, nil
}

// InitClient 根据配置初始化AI客户端
func InitClient() (*Client, error) {
	config, authConfig, err := LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	provider, exists := config.Providers[config.ModelProvider]
	if !exists {
		return nil, fmt.Errorf("provider %s not found in config", config.ModelProvider)
	}

	if authConfig.OpenAIAPIKey == "" || authConfig.OpenAIAPIKey == "这里换成你申请的 KEY" || authConfig.OpenAIAPIKey == "请在这里替换为你的DeepSeek API Key" {
		return nil, fmt.Errorf("please set your API key in ~/.codex/auth.json")
	}

	client := NewClient(provider.BaseURL, authConfig.OpenAIAPIKey, config.Model)

	// 根据provider设置对应的备选模型列表
	if config.ModelProvider == "siliconflow" {
		client.fallbackModels = SiliconFlowModels
		client.currentModelIndex = -1
	}

	return client, nil
}
