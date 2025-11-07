package ai

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service Service
}

// NewHandler 创建AI处理器
func NewHandler() (*Handler, error) {
	service, err := NewService()
	if err != nil {
		return nil, err
	}

	return &Handler{
		service: service,
	}, nil
}

// ChatRequest 聊天请求结构
type ChatCompletionRequest struct {
	Prompt  string        `json:"prompt" binding:"required"`
	History []ChatMessage `json:"history,omitempty"` // 对话历史
}

// ChatResponse 聊天响应结构
type ChatCompletionResponse struct {
	Response string `json:"response"`
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
}

// ArticleSummaryRequest 文章摘要请求
type ArticleSummaryRequest struct {
	Content string `json:"content" binding:"required"`
}

// TranslateRequest 翻译请求
type TranslateRequest struct {
	Text       string `json:"text" binding:"required"`
	TargetLang string `json:"target_lang" binding:"required"`
}

// SentimentRequest 情感分析请求
type SentimentRequest struct {
	Text string `json:"text" binding:"required"`
}

// CommonResponse 通用响应
type CommonResponse struct {
	Result  string `json:"result"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// ChatCompletion 聊天完成API
func (h *Handler) ChatCompletion(c *gin.Context) {
	var req ChatCompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ChatCompletionResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	response, err := h.service.ChatCompletionWithHistory(c.Request.Context(), req.Prompt, req.History)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ChatCompletionResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ChatCompletionResponse{
		Response: response,
		Success:  true,
	})
}

// GenerateArticleSummary 生成文章摘要API
func (h *Handler) GenerateArticleSummary(c *gin.Context) {
	var req ArticleSummaryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, CommonResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	result, err := h.service.GenerateArticleSummary(c.Request.Context(), req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, CommonResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, CommonResponse{
		Result:  result,
		Success: true,
	})
}

// TranslateText 文本翻译API
func (h *Handler) TranslateText(c *gin.Context) {
	var req TranslateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, CommonResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	result, err := h.service.TranslateText(c.Request.Context(), req.Text, req.TargetLang)
	if err != nil {
		c.JSON(http.StatusInternalServerError, CommonResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, CommonResponse{
		Result:  result,
		Success: true,
	})
}

// AnalyzeSentiment 情感分析API
func (h *Handler) AnalyzeSentiment(c *gin.Context) {
	var req SentimentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, CommonResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	result, err := h.service.AnalyzeSentiment(c.Request.Context(), req.Text)
	if err != nil {
		c.JSON(http.StatusInternalServerError, CommonResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, CommonResponse{
		Result:  result,
		Success: true,
	})
}

// RegisterRoutes 注册AI相关路由
func (h *Handler) RegisterRoutes(router *gin.Engine) {
	ai := router.Group("/api/ai")
	{
		ai.POST("/chat", h.ChatCompletion)
		ai.POST("/summary", h.GenerateArticleSummary)
		ai.POST("/translate", h.TranslateText)
		ai.POST("/sentiment", h.AnalyzeSentiment)
	}
}
