package ai

import (
	"context"
	"fmt"
)

type Service interface {
	// ChatCompletion 基础聊天完成接口
	ChatCompletion(ctx context.Context, prompt string) (string, error)

	// ChatCompletionWithHistory 带历史上下文的聊天接口
	ChatCompletionWithHistory(ctx context.Context, prompt string, history []ChatMessage) (string, error)

	// GenerateArticleSummary 生成文章摘要
	GenerateArticleSummary(ctx context.Context, content string) (string, error)

	// TranslateText 文本翻译
	TranslateText(ctx context.Context, text string, targetLang string) (string, error)

	// AnalyzeSentiment 情感分析
	AnalyzeSentiment(ctx context.Context, text string) (string, error)
}

type service struct {
	client *Client
}

// NewService 创建AI服务
func NewService() (Service, error) {
	client, err := InitClient()
	if err != nil {
		return nil, fmt.Errorf("init AI client: %w", err)
	}

	return &service{
		client: client,
	}, nil
}

func (s *service) ChatCompletion(ctx context.Context, prompt string) (string, error) {
	return s.client.ChatWithText(ctx, prompt)
}

func (s *service) ChatCompletionWithHistory(ctx context.Context, prompt string, history []ChatMessage) (string, error) {
	// 将ChatMessage转换为client需要的格式
	clientMessages := make([]ChatMessage, 0, len(history))

	// 添加历史消息
	for _, msg := range history {
		// 只添加之前的对话，不包括当前的prompt
		clientMessages = append(clientMessages, ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// 使用客户端的Chat方法处理带历史的请求
	resp, err := s.client.Chat(ctx, clientMessages)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from AI service")
	}

	return resp.Choices[0].Message.Content, nil
}

func (s *service) GenerateArticleSummary(ctx context.Context, content string) (string, error) {
	prompt := fmt.Sprintf("请为以下文章生成一个简洁的摘要（100-200字）：\n\n%s", content)
	return s.client.ChatWithText(ctx, prompt)
}

func (s *service) TranslateText(ctx context.Context, text string, targetLang string) (string, error) {
	prompt := fmt.Sprintf("请将以下文本翻译成%s：\n\n%s", targetLang, text)
	return s.client.ChatWithText(ctx, prompt)
}

func (s *service) AnalyzeSentiment(ctx context.Context, text string) (string, error) {
	prompt := fmt.Sprintf("请分析以下文本的情感倾向（积极/消极/中性），并简要说明理由：\n\n%s", text)
	return s.client.ChatWithText(ctx, prompt)
}