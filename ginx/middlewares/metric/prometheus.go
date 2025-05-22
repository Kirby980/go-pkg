package metric

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

type MiddlewareBuilder struct {
	summaryOpt prometheus.SummaryOpts
	gaugeOpt   prometheus.GaugeOpts
	labels     []string
}

func NewMiddlewareBuilder(summaryOpt prometheus.SummaryOpts, gaugeOpt prometheus.GaugeOpts, lables ...string) *MiddlewareBuilder {
	return &MiddlewareBuilder{
		summaryOpt: summaryOpt,
		gaugeOpt:   gaugeOpt,
		labels:     lables,
	}
}

func (m *MiddlewareBuilder) Build() gin.HandlerFunc {
	// pattern 是指你命中的路由
	// 是指你的 HTTP 的 status
	// path /detail/1
	summary := prometheus.NewSummaryVec(m.summaryOpt, m.labels)
	prometheus.MustRegister(summary)
	gauge := prometheus.NewGauge(m.gaugeOpt)
	prometheus.MustRegister(gauge)
	return func(ctx *gin.Context) {
		start := time.Now()
		gauge.Inc()
		defer func() {
			duration := time.Since(start)
			gauge.Dec()
			// 404????
			pattern := ctx.FullPath()
			if pattern == "" {
				pattern = "unknown"
			}
			summary.WithLabelValues(
				ctx.Request.Method,
				pattern,
				strconv.Itoa(ctx.Writer.Status()),
			).Observe(float64(duration.Milliseconds()))
		}()
		ctx.Next()
	}
}
