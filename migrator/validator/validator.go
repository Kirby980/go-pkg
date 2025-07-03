package validator

import (
	"context"
	"reflect"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/Kirby980/go-pkg"
	"github.com/Kirby980/go-pkg/logger"
	"github.com/Kirby980/go-pkg/migrator"
	"github.com/Kirby980/go-pkg/migrator/events"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

type Validator[T migrator.Entity] struct {
	base   *gorm.DB
	target *gorm.DB
	l      logger.Logger
	p      events.Producer
	// SRC 表示 以源表为准，DST 表示 以目标表为准
	direction string
	// 增量校验时，每次查询数
	batchSize int
	// 批量查询时每次查询数
	limit         int
	highLoad      *atomic.Bool
	utime         int64
	sleepInterval time.Duration
	fromBase      func(ctx context.Context, offset int) (T, error)
}

func NewValidator[T migrator.Entity](
	base *gorm.DB,
	target *gorm.DB,
	direction string,
	l logger.Logger,
	p events.Producer) *Validator[T] {
	highLoad := &atomic.Bool{}
	highLoad.Store(false)

	res := &Validator[T]{base: base, target: target,
		l: l, p: p, direction: direction,
		highLoad: highLoad}
	res.fromBase = res.fullFromBase

	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			// 检测数据库负载状态
			isHighLoad := res.checkDatabaseLoad()
			highLoad.Store(isHighLoad)
			if isHighLoad {
				res.l.Info("检测到数据库高负载，暂停校验")
			}
		}
	}()

	return res
}

// checkDatabaseLoad 检测数据库负载状态
func (v *Validator[T]) checkDatabaseLoad() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 检测数据库连接数和活跃连接数
	var activeConnections int64
	var maxConnections int64
	var err error

	// 定义结构体来接收 SHOW STATUS 和 SHOW VARIABLES 的结果
	type StatusResult struct {
		VariableName string `gorm:"column:Variable_name"`
		Value        string `gorm:"column:Value"`
	}

	switch v.direction {
	case "SRC":
		// 查询当前活跃连接数
		var statusResult StatusResult
		err = v.base.WithContext(ctx).Raw("SHOW STATUS LIKE 'Threads_connected'").Scan(&statusResult).Error
		if err != nil {
			v.l.Error("查询活跃连接数失败", logger.Error(err))
			return false
		}
		activeConnections, err = strconv.ParseInt(statusResult.Value, 10, 64)
		if err != nil {
			v.l.Error("解析活跃连接数失败", logger.Error(err))
			return false
		}

		// 查询最大连接数
		err = v.base.WithContext(ctx).Raw("SHOW VARIABLES LIKE 'max_connections'").Scan(&statusResult).Error
		if err != nil {
			v.l.Error("查询最大连接数失败", logger.Error(err))
			return false
		}
		maxConnections, err = strconv.ParseInt(statusResult.Value, 10, 64)
		if err != nil {
			v.l.Error("解析最大连接数失败", logger.Error(err))
			return false
		}
	case "DST":
		// 查询当前活跃连接数
		var statusResult StatusResult
		err = v.target.WithContext(ctx).Raw("SHOW STATUS LIKE 'Threads_connected'").Scan(&statusResult).Error
		if err != nil {
			v.l.Error("查询活跃连接数失败", logger.Error(err))
			return false
		}
		activeConnections, err = strconv.ParseInt(statusResult.Value, 10, 64)
		if err != nil {
			v.l.Error("解析活跃连接数失败", logger.Error(err))
			return false
		}

		// 查询最大连接数
		err = v.target.WithContext(ctx).Raw("SHOW VARIABLES LIKE 'max_connections'").Scan(&statusResult).Error
		if err != nil {
			v.l.Error("查询最大连接数失败", logger.Error(err))
			return false
		}
		maxConnections, err = strconv.ParseInt(statusResult.Value, 10, 64)
		if err != nil {
			v.l.Error("解析最大连接数失败", logger.Error(err))
			return false
		}
	}
	// 如果活跃连接数超过最大连接数的80%，认为是高负载
	loadRatio := float64(activeConnections) / float64(maxConnections)
	isHighLoad := loadRatio > 0.8

	v.l.Debug("数据库负载检测",
		logger.Int64("active_connections", activeConnections),
		logger.Int64("max_connections", maxConnections),
		logger.Float64("load_ratio", loadRatio),
		logger.Bool("is_high_load", isHighLoad))

	return isHighLoad
}

// SleepInterval 设置增量校验间隔时间
func (v *Validator[T]) SleepInterval(i time.Duration) *Validator[T] {
	v.sleepInterval = i
	return v

}

// Utime 设置增量校验的 utime
func (v *Validator[T]) Utime(utime int64) *Validator[T] {
	v.utime = utime
	return v
}

// Incr 设置增量校验
func (v *Validator[T]) Incr() *Validator[T] {
	v.fromBase = v.intrFromBase
	return v
}

// Limit 设置全量校验批量校验的 limit
func (v *Validator[T]) Limit(limit int) *Validator[T] {
	v.limit = limit
	return v
}

// Validate 校验
// batch 是否批量校验
func (v *Validator[T]) Validate(ctx context.Context, batch bool) error {
	var eg errgroup.Group
	if batch {
		eg.Go(func() error {
			v.validateBatchBaseToTarget(ctx)
			return nil
		})
		eg.Go(func() error {
			v.validateBatchTargetToBase(ctx)
			return nil
		})
	} else {
		eg.Go(func() error {
			v.validateBaseToTarget(ctx)
			return nil
		})
		eg.Go(func() error {
			v.validateTargetToBase(ctx)
			return nil
		})
	}
	return eg.Wait()
}

// <utime, id> 然后执行 SELECT * FROM xx WHERE utime > ? ORDER BY id
// 索引排序，还是内存排序？

// Validate 调用者可以通过 ctx 来控制校验程序退出
// 全量校验，是不是一条条比对？
// 所以要从数据库里面一条条查询出来
// utime 上面至少要有一个索引，并且 utime 必须是第一列
// <utime, col1, col2>, <utime> 这种可以
// <col1, utime> 这种就不可以
func (v *Validator[T]) validateBaseToTarget(ctx context.Context) {
	offset := 0
	for {
		//
		if v.highLoad.Load() {
			// 挂起
			v.l.Info("数据库高负载,暂停校验,等待1分钟后重试")
			time.Sleep(time.Minute)
			continue
		}

		// 找到了 base 中的数据
		// 例如 .Order("id DESC")，每次插入数据，就会导致你的 offset 不准了
		// 如果我的表没有 id 这个列怎么办？
		// 找一个类似的列，比如说 ctime (创建时间）
		// 作业。你改成批量，性能要好很多
		src, err := v.fromBase(ctx, offset)
		switch err {
		case context.Canceled, context.DeadlineExceeded:
			// 超时或者被人取消了
			return
		case nil:
			// 你真的查到了数据
			// 要去 target 里面找对应的数据
			var dst T
			err = v.target.Where("id = ?", src.ID()).First(&dst).Error
			// 我在这里，怎么办？
			switch err {
			case context.Canceled, context.DeadlineExceeded:
				// 超时或者被人取消了
				return
			case nil:
				// 如果有自定义的比较逻辑，就用自定义的比较逻辑
				// 如果没有，就用反射
				var srcAny any = src
				if c1, ok := srcAny.(interface {
					// 有没有自定义的比较逻辑
					CompareTo(c2 migrator.Entity) bool
				}); ok {
					// 有，我就用它的
					if !c1.CompareTo(dst) {
						v.notify(ctx, src.ID(),
							events.InconsistentEventTypeNEQ)
					}
				} else {
					// 没有，我就用反射
					if !reflect.DeepEqual(src, dst) {
						v.notify(ctx, src.ID(),
							events.InconsistentEventTypeNEQ)
					}
				}
			case gorm.ErrRecordNotFound:
				// 这意味着，target 里面少了数据
				v.notify(ctx, src.ID(),
					events.InconsistentEventTypeTargetMissing)
			default:
				// 这里，要不要汇报，数据不一致？
				// 你有两种做法：
				// 1. 我认为，大概率数据是一致的，我记录一下日志，下一条
				v.l.Error("查询 target 数据失败", logger.Error(err))
				// 2. 我认为，出于保险起见，我应该报数据不一致，试着去修一下
				// 如果真的不一致了，没事，修它
				// 如果假的不一致（也就是数据一致），也没事，就是多余修了一次
				// 不好用哪个 InconsistentType
			}

		case gorm.ErrRecordNotFound:
			// 比完了。没数据了，全量校验结束了
			// 同时支持全量校验和增量校验，你这里就不能直接返回
			// 在这里，你要考虑：有些情况下，用户希望退出，有些情况下。用户希望继续
			// 当用户希望继续的时候，你要 sleep 一下
			if v.sleepInterval <= 0 {
				return
			}
			time.Sleep(v.sleepInterval)
			continue
		default:
			// 数据库错误
			v.l.Error("校验数据，查询 base 出错",
				logger.Error(err))
			// 课堂演示方便，你可以删掉
			time.Sleep(time.Second)
			// offset 最好是挪一下
			// 这里要不要挪
		}
		offset++
	}
}

// validateBatchBaseToTarget 批量全量校验
func (v *Validator[T]) validateBatchBaseToTarget(ctx context.Context) {
	offset := 0
	for {
		if v.highLoad.Load() {
			v.l.Info("数据库高负载,暂停校验,等待1分钟后重试")
			time.Sleep(time.Minute)
			continue
		}

		var srcs []T
		dbCtx, cancel := context.WithTimeout(ctx, time.Second)

		// 全量校验不应该加 utime 条件，应该校验所有数据
		var err error
		if v.utime > 0 {
			// 增量校验模式
			err = v.base.WithContext(dbCtx).
				Order("id").
				Where("utime >= ?", v.utime).
				Offset(offset).
				Limit(v.limit).
				Find(&srcs).Error
		} else {
			// 全量校验模式
			err = v.base.WithContext(dbCtx).
				Order("id").
				Offset(offset).
				Limit(v.limit).
				Find(&srcs).Error
		}
		cancel()

		switch err {
		case context.Canceled, context.DeadlineExceeded:
			return
		case nil:
			if len(srcs) == 0 {
				if v.sleepInterval <= 0 {
					return
				}
				time.Sleep(v.sleepInterval)
				continue
			}
			err1 := v.dstDiff(srcs)
			if err1 != nil {
				v.l.Error("校验数据，查询 target 出错",
					logger.Error(err1))
			}
		default:
			v.l.Error("src => dst 查询源表失败",
				logger.Error(err))
		}
		if len(srcs) < v.limit {
			return
		}
		offset += len(srcs)
	}
}

// dstDiff 比对 srcs 和 dsts
func (v *Validator[T]) dstDiff(srcs []T) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	ids := pkg.ToOtherStruct(srcs, func(idx int, t T) int64 {
		return t.ID()
	})
	var dsts []T
	err := v.target.WithContext(ctx).Where("id in ?", ids).Find(&dsts).Error
	if err != nil {
		return err
	}
	dstMap := v.toMap(dsts)
	for _, src := range srcs {
		dst, ok := dstMap[src.ID()]
		if !ok {
			v.notify(ctx, src.ID(), events.InconsistentEventTypeTargetMissing)
			continue
		}
		// 修复参数错误：应该是 src, dst 而不是 v, dst
		if !reflect.DeepEqual(src, dst) {
			v.notify(ctx, src.ID(), events.InconsistentEventTypeNEQ)
		}
	}
	return nil
}

// toMap 将数据转换为 map
func (v *Validator[T]) toMap(data []T) map[int64]T {
	res := make(map[int64]T, len(data))
	for _, v := range data {
		res[v.ID()] = v
	}
	return res
}

// fullFromBase 全量校验校验函数，每次查询一条数据
func (v *Validator[T]) fullFromBase(ctx context.Context, offset int) (T, error) {
	dbCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	var src T
	// 找到了 base 中的数据
	// 例如 .Order("id DESC")，每次插入数据，就会导致你的 offset 不准了
	// 如果我的表没有 id 这个列怎么办？
	// 找一个类似的列，比如说 ctime (创建时间）
	// 作业。你改成批量，性能要好很多
	err := v.base.WithContext(dbCtx).
		// 最好不要取等号
		Offset(offset).
		Order("id").First(&src).Error
	return src, err
}

// intrFromBase 增量校验
func (v *Validator[T]) intrFromBase(ctx context.Context, offset int) (T, error) {
	dbCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	var src T
	// 找到了 base 中的数据
	// 例如 .Order("id DESC")，每次插入数据，就会导致你的 offset 不准了
	// 如果我的表没有 id 这个列怎么办？
	// 找一个类似的列，比如说 ctime (创建时间）
	// 作业。你改成批量，性能要好很多
	err := v.base.WithContext(dbCtx).
		// 最好不要取等号
		Where("utime > ?", v.utime).
		Offset(offset).
		Order("utime ASC, id ASC").First(&src).Error

	// 等我琢磨一下
	// 按段取
	// WHERE utime >= ? LIMIT 10 ORDER BY UTIME
	// v.utime = srcList[len(srcList)].Utime()

	return src, err
}

// 通用写法，摆脱对 T 的依赖
//func (v *Validator[T]) intrFromBaseV1(ctx context.Context, offset int) (T, error) {
//	rows, err := v.base.WithContext(dbCtx).
//		// 最好不要取等号
//		Where("utime > ?", v.utime).
//		Offset(offset).
//		Order("utime ASC, id ASC").Rows()
//	cols, err := rows.Columns()
//	// 所有列的值
//	vals := make([]any, len(cols))
//	rows.Scan(vals...)
//	return vals
//}

// 理论上来说，可以利用 count 来加速这个过程，
// 我举个例子，假如说你初始化目标表的数据是 昨天的 23:59:59 导出来的
// 那么你可以 COUNT(*) WHERE ctime < 今天的零点，count 如果相等，就说明没删除
// 这一步大多数情况下效果很好，尤其是那些软删除的。
// 如果 count 不一致，那么接下来，你理论上来说，还可以分段 count
// 比如说，我先 count 第一个月的数据，一旦有数据删除了，你还得一条条查出来
// A utime=昨天
// A 在 base 里面，今天删了，A 在 target 里面，utime 还是昨天
// 这个地方，可以考虑不用 utime
// A 在删除之前，已经被修改过了，那么 A 在 target 里面的 utime 就是今天了
func (v *Validator[T]) validateTargetToBase(ctx context.Context) {
	// 先找 target，再找 base，找出 base 中已经被删除的
	// 理论上来说，就是 target 里面一条条找
	offset := 0
	if v.highLoad.Load() {
		v.l.Info("数据库高负载,暂停校验,等待1分钟后重试")
		time.Sleep(time.Minute)
		return
	}
	for {
		dbCtx, cancel := context.WithTimeout(ctx, time.Second)

		var dstTs T
		var err error

		// 根据校验模式决定是否使用 utime 条件
		if v.utime > 0 {
			// 增量校验模式：只校验指定时间后的数据
			err = v.target.WithContext(dbCtx).
				Where("utime > ?", v.utime).
				Select("id").
				Offset(offset).
				Order("utime").First(&dstTs).Error
		} else {
			// 全量校验模式：校验所有数据
			err = v.target.WithContext(dbCtx).
				Select("id").
				Offset(offset).
				Order("id").First(&dstTs).Error
		}
		cancel()

		switch err {
		case context.Canceled, context.DeadlineExceeded:
			// 超时或者被人取消了
			return
		case gorm.ErrRecordNotFound:
			// 没数据了。直接返回
			if v.sleepInterval <= 0 {
				return
			}
			time.Sleep(v.sleepInterval)
			continue
		case nil:
			// 找到了 target 中的数据，去 base 里面找对应的数据
			var srcTs T
			err = v.base.Where("id = ?", dstTs.ID()).First(&srcTs).Error
			switch err {
			case context.Canceled, context.DeadlineExceeded:
				// 超时或者被人取消了
				return
			case gorm.ErrRecordNotFound:
				// 这意味着，base 里面少了数据
				v.notify(ctx, dstTs.ID(), events.InconsistentEventTypeBaseMissing)
			case nil:
				// 如果有自定义的比较逻辑，就用自定义的比较逻辑
				// 如果没有，就用反射
				var dstAny any = dstTs
				if c1, ok := dstAny.(interface {
					// 有没有自定义的比较逻辑
					CompareTo(c2 migrator.Entity) bool
				}); ok {
					// 有，我就用它的
					if !c1.CompareTo(srcTs) {
						v.notify(ctx, dstTs.ID(),
							events.InconsistentEventTypeNEQ)
					}
				} else {
					// 没有，我就用反射
					if !reflect.DeepEqual(dstTs, srcTs) {
						v.notify(ctx, dstTs.ID(),
							events.InconsistentEventTypeNEQ)
					}
				}
			default:
				// 记录日志
				v.l.Error("查询 base 数据失败", logger.Error(err))
			}
		default:
			// 记录日志，continue 掉
			v.l.Error("查询target 失败", logger.Error(err))
		}
		offset++
	}
}

// notifyBaseMissing 通知 base 丢失
func (v *Validator[T]) notifyBaseMissing(ctx context.Context, ids []int64) {
	for _, id := range ids {
		v.notify(ctx, id, events.InconsistentEventTypeBaseMissing)
	}
}

// notify 通知数据不一致
func (v *Validator[T]) notify(ctx context.Context, id int64, typ string) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	event := events.InconsistentEvent{
		ID:        id,
		Direction: v.direction,
		Type:      typ,
	}
	v.l.Info("发送数据不一致事件", logger.Int64("id", id), logger.String("type", typ), logger.String("direction", v.direction))
	err := v.p.ProduceInconsistentEvent(ctx, event)
	cancel()
	if err != nil {
		// 这又是一个问题
		// 怎么办？
		// 你可以重试，但是重试也会失败，记日志，告警，手动去修
		// 我直接忽略，下一轮修复和校验又会找出来
		v.l.Error("发送数据不一致的消息失败", logger.Error(err))
	} else {
		v.l.Info("数据不一致事件发送成功", logger.Int64("id", id), logger.String("type", typ))
	}
}

// validateBatchTargetToBase 批量反向校验
func (v *Validator[T]) validateBatchTargetToBase(ctx context.Context) {
	offset := 0
	for {
		if v.highLoad.Load() {
			v.l.Info("数据库高负载,暂停校验,等待1分钟后重试")
			time.Sleep(time.Minute)
			continue
		}

		var dstTs []T
		dbCtx, cancel := context.WithTimeout(ctx, time.Second)

		// 根据校验模式决定是否使用 utime 条件
		var err error
		if v.utime > 0 {
			// 增量校验模式：只校验指定时间后的数据
			err = v.target.WithContext(dbCtx).
				Where("utime > ?", v.utime).
				Select("id").
				Offset(offset).Limit(v.limit).
				Order("utime").Find(&dstTs).Error
		} else {
			// 全量校验模式：校验所有数据
			err = v.target.WithContext(dbCtx).
				Select("id").
				Offset(offset).Limit(v.limit).
				Order("id").Find(&dstTs).Error
		}
		cancel()

		switch err {
		case context.Canceled, context.DeadlineExceeded:
			return
		case nil:
			if len(dstTs) == 0 {
				if v.sleepInterval <= 0 {
					return
				}
				time.Sleep(v.sleepInterval)
				continue
			}
			err1 := v.baseDiff(dstTs)
			if err1 != nil {
				v.l.Error("校验数据，查询 base 出错",
					logger.Error(err1))
			}
		default:
			v.l.Error("dst => src 查询目标表失败",
				logger.Error(err))
		}
		if len(dstTs) < v.limit {
			return
		}
		offset += len(dstTs)
	}
}

// baseDiff 比对 dsts 和 srcs
func (v *Validator[T]) baseDiff(dsts []T) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	ids := pkg.ToOtherStruct(dsts, func(idx int, t T) int64 {
		return t.ID()
	})
	var srcs []T
	err := v.base.WithContext(ctx).Where("id in ?", ids).Find(&srcs).Error
	if err != nil {
		return err
	}
	srcMap := v.toMap(srcs)
	for _, dst := range dsts {
		src, ok := srcMap[dst.ID()]
		if !ok {
			v.notify(ctx, dst.ID(), events.InconsistentEventTypeBaseMissing)
			continue
		}
		// 比较数据一致性
		if !reflect.DeepEqual(src, dst) {
			v.notify(ctx, dst.ID(), events.InconsistentEventTypeNEQ)
		}
	}
	return nil
}
