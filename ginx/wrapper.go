package ginx

import (
	"net/http"
	"strconv"

	"github.com/Kirby980/go-pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

var L = logger.NewZapLogger(zap.NewExample())

var vector *prometheus.CounterVec

func InitCounter(opt prometheus.CounterOpts, lables ...string) {
	vector = prometheus.NewCounterVec(opt, lables)
	prometheus.MustRegister(vector)
}

// WrapBody 包装一个函数，用于处理请求体
func WrapBody[Req any](fn func(ctx *gin.Context, req Req) (Result, error)) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req Req
		if err := ctx.Bind(&req); err != nil {
			return
		}
		res, err := fn(ctx, req)
		if err != nil {
			L.Error("处理业务逻辑出错",
				logger.String("path", ctx.Request.URL.Path),
				logger.String("route", ctx.FullPath()),
				logger.Error(err))
		}
		vector.WithLabelValues(strconv.Itoa(res.Code)).Inc()
		ctx.JSON(http.StatusOK, res)
	}
}

// WarpBodyAndToken 包装一个函数，用于处理请求体和token
func WarpBodyAndToken[Req any, C jwt.Claims](fn func(ctx *gin.Context, req Req, uc C) (Result, error), claims string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		val, ok := ctx.Get(claims)
		if !ok {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c, ok := val.(C)
		if !ok {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		var req Req
		if err := ctx.Bind(&req); err != nil {
			L.Error("参数错误",
				logger.String("path", ctx.Request.URL.Path),
				logger.String("route", ctx.FullPath()),
				logger.Error(err))
			return
		}
		res, err := fn(ctx, req, c)
		if err != nil {
			L.Error("处理业务逻辑出错",
				logger.String("path", ctx.Request.URL.Path),
				logger.String("route", ctx.FullPath()),
				logger.Error(err))
		}
		vector.WithLabelValues(strconv.Itoa(res.Code)).Inc()
		ctx.JSON(http.StatusOK, res)
	}
}

// WrapToken 包装一个函数，用于处理token
func WrapToken[C jwt.Claims](fn func(ctx *gin.Context, uc C) (Result, error), claims string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		val, ok := ctx.Get(claims)
		if !ok {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c, ok := val.(C)
		if !ok {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		res, err := fn(ctx, c)
		if err != nil {
			L.Error("处理业务逻辑出错",
				logger.String("path", ctx.Request.URL.Path),
				logger.String("route", ctx.FullPath()),
				logger.Error(err))
		}

		vector.WithLabelValues(strconv.Itoa(res.Code)).Inc()
		ctx.JSON(http.StatusOK, res)
	}
}

// Wrap 包装一个函数，用于处理请求
func Wrap(fn func(ctx *gin.Context) (Result, error)) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		res, err := fn(ctx)
		if err != nil {
			// 开始处理 error，其实就是记录一下日志
			L.Error("处理业务逻辑出错",
				logger.String("path", ctx.Request.URL.Path),
				// 命中的路由
				logger.String("route", ctx.FullPath()),
				logger.Error(err))
		}
		vector.WithLabelValues(strconv.Itoa(res.Code)).Inc()
		ctx.JSON(http.StatusOK, res)
	}
}

// code 4为用户错误，5为系统错误
type Result struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}
