package ginx

import (
	"net/http"
	"strconv"

	"github.com/Kirby980/study/webook/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

var L = logger.NewZapLogger(zap.NewExample())

var vector *prometheus.CounterVec

func InitConter(opt prometheus.CounterOpts, lables ...string) {
	vector = prometheus.NewCounterVec(opt, lables)
	prometheus.MustRegister(vector)
}

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

// code 4为用户错误，5为系统错误
type Result struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}
