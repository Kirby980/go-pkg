package connpool

import (
	"context"
	"database/sql"
	"log"
	"sync/atomic"

	"gorm.io/gorm"
)

const (
	PatternDstOnly  = "DST_ONLY"
	PatternSrcOnly  = "SRC_ONLY"
	PatternDstFirst = "DST_FIRST"
	PatternSrcFirst = "SRC_FIRST"
)

// DoubleWritePool 双写模式
type DoubleWritePool struct {
	src     gorm.ConnPool
	dst     gorm.ConnPool
	pattern atomic.Value
}

// NewDoubleWritePool 创建一个新的双写模式
func NewDoubleWritePool(src, dst gorm.ConnPool, pattern string) *DoubleWritePool {
	dw := &DoubleWritePool{
		src: src,
		dst: dst,
	}
	dw.pattern.Store(pattern)
	return dw
}

// UpdatePattern 更新双写模式
func (d *DoubleWritePool) UpdatePattern(pattern string) {
	d.pattern.Store(pattern)
}

// BeginTx gorm的事务接口
func (d *DoubleWritePool) BeginTx(ctx context.Context, opts *sql.TxOptions) (gorm.ConnPool, error) {
	pattern := d.pattern.Load().(string)
	switch pattern {
	case PatternSrcOnly:
		tx, err := d.src.(gorm.TxBeginner).BeginTx(ctx, opts)
		if err != nil {
			return nil, err
		}
		return &DoubleWritePoolTx{
			src:     tx,
			pattern: pattern,
			DoubleWritePool: DoubleWritePool{
				src:     d.src,
				pattern: d.pattern,
			},
		}, nil
	case PatternSrcFirst:
		srcTx, err := d.src.(gorm.TxBeginner).BeginTx(ctx, opts)
		if err != nil {
			return nil, err
		}
		dstTx, err := d.dst.(gorm.TxBeginner).BeginTx(ctx, opts)
		if err != nil {
			log.Println("双写模式下，目标表开启事务失败", err)
		}
		return &DoubleWritePoolTx{
			src:     srcTx,
			dst:     dstTx,
			pattern: pattern,
			DoubleWritePool: DoubleWritePool{
				src:     d.src,
				dst:     d.dst,
				pattern: d.pattern,
			},
		}, nil
	case PatternDstOnly:
		tx, err := d.dst.(gorm.TxBeginner).BeginTx(ctx, opts)
		if err != nil {
			return nil, err
		}
		return &DoubleWritePoolTx{
			dst:     tx,
			pattern: pattern,
			DoubleWritePool: DoubleWritePool{
				dst:     d.dst,
				pattern: d.pattern,
			},
		}, nil
	case PatternDstFirst:
		dstTx, err := d.dst.(gorm.TxBeginner).BeginTx(ctx, opts)
		if err != nil {
			return nil, err
		}
		srcTx, err := d.src.(gorm.TxBeginner).BeginTx(ctx, opts)
		if err != nil {
			log.Println("双写模式下，源表开启事务失败", err)
		}
		return &DoubleWritePoolTx{
			src:     srcTx,
			dst:     dstTx,
			pattern: pattern,
			DoubleWritePool: DoubleWritePool{
				src:     d.src,
				dst:     d.dst,
				pattern: d.pattern,
			},
		}, nil
	default:
		panic("没有双写模式事务操作")
	}
}

// PrepareContext 预编译接口
func (d *DoubleWritePool) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	panic("没有这种双写模式")
}

// ExecContext gorm的增删改的查询接口
func (d *DoubleWritePool) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	pattern, ok := d.pattern.Load().(string)
	if !ok {
		pattern = PatternSrcOnly // 默认使用源库
	}
	switch pattern {
	case PatternSrcOnly:
		return d.src.ExecContext(ctx, query, args...)
	case PatternSrcFirst:
		res, err := d.src.ExecContext(ctx, query, args...)
		if err != nil {
			return res, err
		}
		_, err = d.dst.ExecContext(ctx, query, args...)
		if err != nil {
			log.Println("双写模式下，目标库执行失败", err)
		}
		return res, nil
	case PatternDstOnly:
		return d.dst.ExecContext(ctx, query, args...)
	case PatternDstFirst:
		res, err := d.dst.ExecContext(ctx, query, args...)
		if err != nil {
			return res, err
		}
		_, err = d.src.ExecContext(ctx, query, args...)
		if err != nil {
			log.Println("双写模式下，源库执行失败", err)
		}
		return res, nil
	default:
		panic("没有这种双写模式")
	}
}

// QueryContext gorm的多行查询接口
func (d *DoubleWritePool) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	pattern, ok := d.pattern.Load().(string)
	if !ok {
		pattern = PatternSrcOnly // 默认使用源库
	}
	switch pattern {
	case PatternSrcOnly, PatternSrcFirst:
		return d.src.QueryContext(ctx, query, args...)
	case PatternDstOnly, PatternDstFirst:
		return d.dst.QueryContext(ctx, query, args...)
	default:
		panic("未知的双写模式")
	}
}

// QueryRowContext gorm的单行查询接口
func (d *DoubleWritePool) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	pattern, ok := d.pattern.Load().(string)
	if !ok {
		pattern = PatternSrcOnly // 默认使用源库
	}
	switch pattern {
	case PatternSrcOnly, PatternSrcFirst:
		return d.src.QueryRowContext(ctx, query, args...)
	case PatternDstOnly, PatternDstFirst:
		return d.dst.QueryRowContext(ctx, query, args...)
	default:
		panic("未知的双写模式")
	}
}

// DoubleWritePoolTx 双写模式的事务
type DoubleWritePoolTx struct {
	src     gorm.Tx
	dst     gorm.Tx
	pattern string
	DoubleWritePool
}

// Commit 提交事务
func (d *DoubleWritePoolTx) Commit() error {
	switch d.pattern {
	case PatternSrcOnly:
		return d.src.Commit()
	case PatternSrcFirst:
		err := d.src.Commit()
		if err != nil {
			return err
		}
		if d.dst != nil {
			err = d.dst.Commit()
			if err != nil {
				log.Println("双写模式下，目标表事务提交失败")
			}
		}
		return nil
	case PatternDstOnly:
		return d.dst.Commit()
	case PatternDstFirst:
		err := d.dst.Commit()
		if err != nil {
			return err
		}
		if d.src != nil {
			err = d.src.Commit()
			if err != nil {
				log.Println("双写模式下，源表事务提交失败")
			}
		}
		return nil
	default:
		panic("没有这种双写模式")
	}
}

// Rollback 回滚事务
func (d *DoubleWritePoolTx) Rollback() error {
	switch d.pattern {
	case PatternSrcOnly:
		return d.src.Rollback()
	case PatternSrcFirst:
		err := d.src.Rollback()
		if err != nil {
			return err
		}
		if d.dst != nil {
			err = d.dst.Rollback()
			if err != nil {
				log.Println("双写模式下，目标表事务回滚失败")
			}
		}
		return nil
	case PatternDstOnly:
		return d.dst.Rollback()
	case PatternDstFirst:
		err := d.dst.Rollback()
		if err != nil {
			return err
		}
		if d.src != nil {
			err = d.src.Rollback()
			if err != nil {
				log.Println("双写模式下，源表事务回滚失败")
			}
		}
		return nil
	default:
		panic("没有这种双写模式")
	}
}
