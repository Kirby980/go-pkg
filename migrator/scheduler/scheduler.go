package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Kirby980/go-pkg/ginx"
	"github.com/Kirby980/go-pkg/gormx/connpool"
	"github.com/Kirby980/go-pkg/logger"
	"github.com/Kirby980/go-pkg/migrator"
	"github.com/Kirby980/go-pkg/migrator/events"
	"github.com/Kirby980/go-pkg/migrator/validator"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Scheduler 用来统一管理整个迁移过程
// 它不是必须的，你可以理解为这是为了方便用户操作（和你理解）而引入的。
type Scheduler[T migrator.Entity] struct {
	lock       sync.Mutex
	src        *gorm.DB
	dst        *gorm.DB
	pool       *connpool.DoubleWritePool
	l          logger.Logger
	pattern    string
	cancelFull func()
	cancelIncr func()
	producer   events.Producer

	// 添加状态管理
	isFullRunning bool
	isIncrRunning bool

	// 如果你要允许多个全量校验同时运行
	//fulls map[string]func()
}

func NewScheduler[T migrator.Entity](
	l logger.Logger,
	src *gorm.DB,
	dst *gorm.DB,
	// 这个是业务用的 DoubleWritePool
	pool *connpool.DoubleWritePool,
	producer events.Producer) *Scheduler[T] {
	return &Scheduler[T]{
		l:       l,
		src:     src,
		dst:     dst,
		pattern: connpool.PatternSrcOnly,
		cancelFull: func() {
			// 初始的时候，啥也不用做
		},
		cancelIncr: func() {
			// 初始的时候，啥也不用做
		},
		pool:     pool,
		producer: producer,
	}
}

// 这一个也不是必须的，就是你可以考虑利用配置中心，监听配置中心的变化
// 把全量校验，增量校验做成分布式任务，利用分布式任务调度平台来调度
// 批量全量校验和全量校验只能同时运行一个
func (s *Scheduler[T]) RegisterRoutes(server *gin.RouterGroup) {
	// 将这个暴露为 HTTP 接口
	// 你可以配上对应的 UI
	server.POST("/src_only", ginx.Wrap(s.SrcOnly))
	server.POST("/src_first", ginx.Wrap(s.SrcFirst))
	server.POST("/dst_first", ginx.Wrap(s.DstFirst))
	server.POST("/dst_only", ginx.Wrap(s.DstOnly))
	server.POST("/full/start", ginx.Wrap(s.StartFullValidation))
	server.POST("/full/stop", ginx.Wrap(s.StopFullValidation))
	server.POST("/incr/stop", ginx.Wrap(s.StopIncrementValidation))
	server.POST("/incr/start", ginx.WrapBody(s.StartIncrementValidation))
	server.POST("/full/batch/start", ginx.WrapBody(s.StartFullValidationBatch))
	server.POST("/full/batch/stop", ginx.Wrap(s.StopFullValidationBatch))
	server.GET("/status", ginx.Wrap(s.GetStatus))
}

// GetStatus 获取当前状态
func (s *Scheduler[T]) GetStatus(c *gin.Context) (ginx.Result, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	return ginx.Result{
		Data: map[string]interface{}{
			"pattern":      s.pattern,
			"full_running": s.isFullRunning,
			"incr_running": s.isIncrRunning,
		},
		Msg: "OK",
	}, nil
}

// ---- 下面是四个阶段 ---- //

// SrcOnly 只读写源表
func (s *Scheduler[T]) SrcOnly(c *gin.Context) (ginx.Result, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.pattern = connpool.PatternSrcOnly
	s.pool.UpdatePattern(connpool.PatternSrcOnly)
	return ginx.Result{
		Msg: "OK",
	}, nil
}

// SrcFirst 先读写源表
// 先读写源表，再读写目标表
func (s *Scheduler[T]) SrcFirst(c *gin.Context) (ginx.Result, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.pattern = connpool.PatternSrcFirst
	s.pool.UpdatePattern(connpool.PatternSrcFirst)
	return ginx.Result{
		Msg: "OK",
	}, nil
}

// DstFirst 先读写目标表
// 先读写目标表，再读写源表
func (s *Scheduler[T]) DstFirst(c *gin.Context) (ginx.Result, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.pattern = connpool.PatternDstFirst
	s.pool.UpdatePattern(connpool.PatternDstFirst)
	return ginx.Result{
		Msg: "OK",
	}, nil
}

// DstOnly 只读写目标表
func (s *Scheduler[T]) DstOnly(c *gin.Context) (ginx.Result, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.pattern = connpool.PatternDstOnly
	s.pool.UpdatePattern(connpool.PatternDstOnly)
	return ginx.Result{
		Msg: "OK",
	}, nil
}

// StopIncrementValidation 停止增量校验
func (s *Scheduler[T]) StopIncrementValidation(c *gin.Context) (ginx.Result, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.isIncrRunning {
		s.cancelIncr()
		s.isIncrRunning = false
		return ginx.Result{
			Msg: "停止增量校验成功",
		}, nil
	}
	return ginx.Result{
		Msg: "增量校验未运行",
	}, nil
}

// StartIncrementValidation 启动增量校验
func (s *Scheduler[T]) StartIncrementValidation(c *gin.Context,
	req StartIncrRequest) (ginx.Result, error) {
	// 开启增量校验
	s.lock.Lock()
	defer s.lock.Unlock()

	// 检查是否已经在运行
	if s.isIncrRunning {
		return ginx.Result{
			Code: 400,
			Msg:  "增量校验已在运行中",
		}, nil
	}

	// 取消上一次的
	if s.cancelIncr != nil {
		s.cancelIncr()
	}

	v, err := s.newValidator()
	if err != nil {
		return ginx.Result{
			Code: 5,
			Msg:  "系统异常",
		}, nil
	}
	v.Incr().Utime(req.Utime).
		SleepInterval(time.Duration(req.Interval) * time.Millisecond)

	go func() {
		var ctx context.Context
		ctx, s.cancelIncr = context.WithCancel(context.Background())

		s.isIncrRunning = true
		err := v.Validate(ctx, false)
		if err != nil {
			s.l.Warn("退出增量校验", logger.Error(err))
		}
		s.isIncrRunning = false
	}()

	return ginx.Result{
		Msg: "启动增量校验成功",
	}, nil
}

// StopFullValidation 停止全量校验
func (s *Scheduler[T]) StopFullValidation(c *gin.Context) (ginx.Result, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.isFullRunning {
		s.cancelFull()
		s.isFullRunning = false
		return ginx.Result{
			Msg: "停止全量校验成功",
		}, nil
	}
	return ginx.Result{
		Msg: "全量校验未运行",
	}, nil
}

// StartFullValidation 全量校验
func (s *Scheduler[T]) StartFullValidation(c *gin.Context) (ginx.Result, error) {
	// 可以考虑去重的问题
	s.lock.Lock()
	defer s.lock.Unlock()

	// 检查是否已经在运行
	if s.isFullRunning {
		return ginx.Result{
			Code: 400,
			Msg:  "全量校验已在运行中",
		}, nil
	}

	// 取消上一次的
	if s.cancelFull != nil {
		s.cancelFull()
	}

	v, err := s.newValidator()
	if err != nil {
		return ginx.Result{
			Code: 5,
			Msg:  "创建校验器失败",
		}, err
	}
	var ctx context.Context
	ctx, s.cancelFull = context.WithCancel(context.Background())
	s.isFullRunning = true

	go func() {
		// 先取消上一次的
		err := v.Validate(ctx, false)
		if err != nil {
			s.l.Warn("退出全量校验", logger.Error(err))
		}
		s.isFullRunning = false

	}()

	return ginx.Result{
		Msg: "启动全量校验成功",
	}, nil
}

// StartFullValidationBatch 启动批量全量校验
func (s *Scheduler[T]) StartFullValidationBatch(c *gin.Context, req BatchStartFullRequest) (ginx.Result, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	// 检查是否已经在运行
	if s.isFullRunning {
		return ginx.Result{
			Code: 400,
			Msg:  "全量校验已在运行中",
		}, nil
	}

	// 取消上一次的
	if s.cancelFull != nil {
		s.cancelFull()
	}

	v, err := s.newValidator()
	if err != nil {
		return ginx.Result{
			Code: 5,
			Msg:  "系统异常",
		}, err
	}
	v.Limit(req.Limit)

	// 创建新的 context 和 cancel 函数
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFull = cancel
	s.isFullRunning = true

	go func() {
		err := v.Validate(ctx, true)
		if err != nil {
			s.l.Warn("退出批量全量校验", logger.Error(err))
		}
		s.isFullRunning = false
	}()

	return ginx.Result{
		Msg: "启动批量全量校验成功",
	}, nil
}

// StopFullValidationBatch 停止批量全量校验
func (s *Scheduler[T]) StopFullValidationBatch(c *gin.Context) (ginx.Result, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.isFullRunning {
		s.cancelFull()
		s.isFullRunning = false
		return ginx.Result{
			Msg: "停止批量全量校验成功",
		}, nil
	}

	return ginx.Result{
		Msg: "批量全量校验未运行",
	}, nil
}

// newValidator 创建一个 validator
func (s *Scheduler[T]) newValidator() (*validator.Validator[T], error) {
	switch s.pattern {
	case connpool.PatternSrcOnly, connpool.PatternSrcFirst:
		return validator.NewValidator[T](s.src, s.dst, "SRC", s.l, s.producer), nil
	case connpool.PatternDstFirst, connpool.PatternDstOnly:
		return validator.NewValidator[T](s.dst, s.src, "DST", s.l, s.producer), nil
	default:
		return nil, fmt.Errorf("未知的 pattern %s", s.pattern)
	}
}

// StartIncrRequest 启动增量校验的请求
type StartIncrRequest struct {
	Utime int64 `json:"utime"`
	// 毫秒数
	// json 不能正确处理 time.Duration 类型
	Interval int64 `json:"interval"`
}

type BatchStartFullRequest struct {
	Limit int `json:"limit"`
}
