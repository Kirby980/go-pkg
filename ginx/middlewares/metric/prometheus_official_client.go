package metric

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// PrometheusOfficialClient 使用官方 Prometheus Go API 客户端
type PrometheusOfficialClient struct {
	api v1.API
}

// NewPrometheusOfficialClient 创建新的官方 Prometheus 客户端
func NewPrometheusOfficialClient(prometheusURL string) (*PrometheusOfficialClient, error) {
	client, err := api.NewClient(api.Config{
		Address: prometheusURL,
	})
	if err != nil {
		return nil, fmt.Errorf("创建 Prometheus 客户端失败: %w", err)
	}

	return &PrometheusOfficialClient{
		api: v1.NewAPI(client),
	}, nil
}

// GetAverageResponseTime 获取平均响应时间
func (p *PrometheusOfficialClient) GetAverageResponseTime() (model.Value, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `histogram_quantile(0.5, sum(rate(http_request_duration_seconds_bucket[5m])) by (le, path))`
	result, _, err := p.api.Query(ctx, query, time.Now())
	return result, err
}

// Get99thPercentile 获取 99 线响应时间
func (p *PrometheusOfficialClient) Get99thPercentile() (model.Value, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket[5m])) by (le, path))`
	result, _, err := p.api.Query(ctx, query, time.Now())
	return result, err
}

// Get95thPercentile 获取 95 线响应时间
func (p *PrometheusOfficialClient) Get95thPercentile() (model.Value, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le, path))`
	result, _, err := p.api.Query(ctx, query, time.Now())
	return result, err
}

// GetActiveRequests 获取当前活跃请求数
func (p *PrometheusOfficialClient) GetActiveRequests() (model.Value, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `http_requests_in_progress`
	result, _, err := p.api.Query(ctx, query, time.Now())
	return result, err
}

// GetRequestRate 获取请求速率（QPS）
func (p *PrometheusOfficialClient) GetRequestRate() (model.Value, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `sum(rate(http_request_duration_seconds_count[5m])) by (path)`
	result, _, err := p.api.Query(ctx, query, time.Now())
	return result, err
}

// GetErrorRate 获取错误率
func (p *PrometheusOfficialClient) GetErrorRate() (model.Value, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `sum(rate(http_request_duration_seconds_count{status=~"5.."}[5m])) by (path) / sum(rate(http_request_duration_seconds_count[5m])) by (path)`
	result, _, err := p.api.Query(ctx, query, time.Now())
	return result, err
}

// GetMetricsRange 获取时间范围内的指标数据
func (p *PrometheusOfficialClient) GetMetricsRange(query string, start, end time.Time, step time.Duration) (model.Value, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, _, err := p.api.QueryRange(ctx, query, v1.Range{
		Start: start,
		End:   end,
		Step:  step,
	})
	return result, err
}

// PrintMetrics 打印指标信息
func (p *PrometheusOfficialClient) PrintMetrics() error {
	fmt.Println("=== Prometheus 指标查询结果（官方客户端）===")

	// 查询平均响应时间
	if result, err := p.GetAverageResponseTime(); err == nil {
		fmt.Println("\n平均响应时间:")
		if vector, ok := result.(model.Vector); ok {
			for _, sample := range vector {
				path := string(sample.Metric["path"])
				fmt.Printf("  %s: %.3f 秒\n", path, float64(sample.Value))
			}
		}
	} else {
		fmt.Printf("查询平均响应时间失败: %v\n", err)
	}

	// 查询 99 线响应时间
	if result, err := p.Get99thPercentile(); err == nil {
		fmt.Println("\n99 线响应时间:")
		if vector, ok := result.(model.Vector); ok {
			for _, sample := range vector {
				path := string(sample.Metric["path"])
				fmt.Printf("  %s: %.3f 秒\n", path, float64(sample.Value))
			}
		}
	} else {
		fmt.Printf("查询 99 线响应时间失败: %v\n", err)
	}

	// 查询 95 线响应时间
	if result, err := p.Get95thPercentile(); err == nil {
		fmt.Println("\n95 线响应时间:")
		if vector, ok := result.(model.Vector); ok {
			for _, sample := range vector {
				path := string(sample.Metric["path"])
				fmt.Printf("  %s: %.3f 秒\n", path, float64(sample.Value))
			}
		}
	} else {
		fmt.Printf("查询 95 线响应时间失败: %v\n", err)
	}

	// 查询活跃请求数
	if result, err := p.GetActiveRequests(); err == nil {
		fmt.Println("\n当前活跃请求数:")
		if vector, ok := result.(model.Vector); ok {
			for _, sample := range vector {
				fmt.Printf("  %.0f\n", float64(sample.Value))
			}
		}
	} else {
		fmt.Printf("查询活跃请求数失败: %v\n", err)
	}

	// 查询请求速率
	if result, err := p.GetRequestRate(); err == nil {
		fmt.Println("\n请求速率 (QPS):")
		if vector, ok := result.(model.Vector); ok {
			for _, sample := range vector {
				path := string(sample.Metric["path"])
				fmt.Printf("  %s: %.2f 请求/秒\n", path, float64(sample.Value))
			}
		}
	} else {
		fmt.Printf("查询请求速率失败: %v\n", err)
	}

	// 查询错误率
	if result, err := p.GetErrorRate(); err == nil {
		fmt.Println("\n错误率:")
		if vector, ok := result.(model.Vector); ok {
			for _, sample := range vector {
				path := string(sample.Metric["path"])
				fmt.Printf("  %s: %.2f%%\n", path, float64(sample.Value)*100)
			}
		}
	} else {
		fmt.Printf("查询错误率失败: %v\n", err)
	}

	return nil
}

// GetMetricValue 获取单个指标值
func (p *PrometheusOfficialClient) GetMetricValue(query string) (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, _, err := p.api.Query(ctx, query, time.Now())
	if err != nil {
		return 0, err
	}

	if vector, ok := result.(model.Vector); ok && len(vector) > 0 {
		return float64(vector[0].Value), nil
	}

	return 0, fmt.Errorf("没有找到指标数据")
}

// GetMetricWithLabels 获取带标签的指标值
func (p *PrometheusOfficialClient) GetMetricWithLabels(query string) (map[string]float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, _, err := p.api.Query(ctx, query, time.Now())
	if err != nil {
		return nil, err
	}

	metrics := make(map[string]float64)
	if vector, ok := result.(model.Vector); ok {
		for _, sample := range vector {
			// 构建标签字符串作为 key
			key := ""
			for name, value := range sample.Metric {
				if key != "" {
					key += ","
				}
				key += string(name) + "=" + string(value)
			}
			metrics[key] = float64(sample.Value)
		}
	}

	return metrics, nil
}
