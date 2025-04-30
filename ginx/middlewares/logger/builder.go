package logger

import (
	"bytes"
	"context"
	"io"
	"time"

	"github.com/gin-gonic/gin"
)

type MiddlewareBuilder struct {
	allowReqBody  bool
	allowRespBody bool
	bodySize      int
	loggerFunc    func(ctx context.Context, al *AccessLog)
}

func NewBuilder(fn func(ctx context.Context, al *AccessLog)) *MiddlewareBuilder {
	return &MiddlewareBuilder{
		loggerFunc: fn,
	}
}

func (m *MiddlewareBuilder) SetBodySize(size int) *MiddlewareBuilder {
	if size <= 0 {
		size = 1024
	}
	m.bodySize = size
	return m
}
func (m *MiddlewareBuilder) AllowReqBody() *MiddlewareBuilder {
	m.allowReqBody = true
	return m
}
func (m *MiddlewareBuilder) AllowRespBody() *MiddlewareBuilder {
	m.allowRespBody = true
	return m

}
func (m *MiddlewareBuilder) Build() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		start := time.Now()
		url := ctx.Request.URL.String()
		if len(url) > m.bodySize {
			url = url[:m.bodySize]
		}
		al := &AccessLog{
			Method: ctx.Request.Method,
			Url:    url,
		}
		if ctx.Request.Body != nil && m.allowReqBody {
			//body 读完就没了是iostream
			body, _ := ctx.GetRawData()
			ctx.Request.Body = io.NopCloser(bytes.NewBuffer(body))
			if len(body) > m.bodySize {
				body = body[:m.bodySize]
			}
			al.ReqBody = string(body)
		}

		if m.allowRespBody {
			ctx.Writer = &responseWriter{
				al:             al,
				ResponseWriter: ctx.Writer,
			}
		}
		defer func() {
			al.Duration = time.Since(start).String()
			m.loggerFunc(ctx, al)
		}()
		ctx.Next()

	}
}

type responseWriter struct {
	al *AccessLog
	gin.ResponseWriter
}

func (rw *responseWriter) WriteHandler(statusCode int) {
	rw.al.Status = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *responseWriter) Write(data []byte) (int, error) {
	rw.al.RespBody = string(data)
	return rw.ResponseWriter.Write(data)
}
func (rw *responseWriter) WriteString(data string) (int, error) {
	rw.al.RespBody = string(data)
	return rw.ResponseWriter.WriteString(data)
}

type AccessLog struct {
	Method   string
	Url      string
	Duration string
	ReqBody  string
	RespBody string
	Status   int
}
