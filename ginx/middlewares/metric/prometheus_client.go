package metric

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// PrometheusClient 用于查询 Prometheus 指标的客户端
type PrometheusClient struct {
	baseURL string
	client  *http.Client
}

// QueryResult 查询结果
type QueryResult struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// NewPrometheusClient 创建新的 Prometheus 客户端
func NewPrometheusClient(baseURL string) *PrometheusClient {
	return &PrometheusClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Query 执行即时查询
func (p *PrometheusClient) Query(query string) (*QueryResult, error) {
	u, err := url.Parse(p.baseURL + "/api/v1/query")
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("query", query)
	u.RawQuery = q.Encode()

	resp, err := p.client.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result QueryResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetAverageResponseTime 获取平均响应时间
func (p *PrometheusClient) GetAverageResponseTime() (*QueryResult, error) {
	query := `histogram_quantile(0.5, sum(rate(http_request_duration_seconds_bucket[5m])) by (le, path))`
	return p.Query(query)
}

// Get99thPercentile 获取 99 线响应时间
func (p *PrometheusClient) Get99thPercentile() (*QueryResult, error) {
	query := `histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket[5m])) by (le, path))`
	return p.Query(query)
}

// GetActiveRequests 获取当前活跃请求数
func (p *PrometheusClient) GetActiveRequests() (*QueryResult, error) {
	query := `http_requests_in_progress`
	return p.Query(query)
}

// GetRequestRate 获取请求速率（QPS）
func (p *PrometheusClient) GetRequestRate() (*QueryResult, error) {
	query := `sum(rate(http_request_duration_seconds_count[5m])) by (path)`
	return p.Query(query)
}

// GetErrorRate 获取错误率
func (p *PrometheusClient) GetErrorRate() (*QueryResult, error) {
	query := `sum(rate(http_request_duration_seconds_count{status=~"5.."}[5m])) by (path) / sum(rate(http_request_duration_seconds_count[5m])) by (path)`
	return p.Query(query)
}

// PrintMetrics 打印指标信息
func (p *PrometheusClient) PrintMetrics() error {
	fmt.Println("=== Prometheus 指标查询结果 ===")

	// 查询平均响应时间
	if result, err := p.GetAverageResponseTime(); err == nil {
		fmt.Println("\n平均响应时间:")
		for _, r := range result.Data.Result {
			path := r.Metric["path"]
			value := r.Value[1].(string)
			fmt.Printf("  %s: %s 秒\n", path, value)
		}
	}

	// 查询 99 线响应时间
	if result, err := p.Get99thPercentile(); err == nil {
		fmt.Println("\n99 线响应时间:")
		for _, r := range result.Data.Result {
			path := r.Metric["path"]
			value := r.Value[1].(string)
			fmt.Printf("  %s: %s 秒\n", path, value)
		}
	}

	// 查询活跃请求数
	if result, err := p.GetActiveRequests(); err == nil {
		fmt.Println("\n当前活跃请求数:")
		for _, r := range result.Data.Result {
			value := r.Value[1].(string)
			fmt.Printf("  %s\n", value)
		}
	}

	// 查询请求速率
	if result, err := p.GetRequestRate(); err == nil {
		fmt.Println("\n请求速率 (QPS):")
		for _, r := range result.Data.Result {
			path := r.Metric["path"]
			value := r.Value[1].(string)
			fmt.Printf("  %s: %s 请求/秒\n", path, value)
		}
	}

	return nil
}
