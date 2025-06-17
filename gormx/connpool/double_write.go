package connpool

import (
	"context"
	"database/sql"
	"log"
	"sync/atomic"

	"gorm.io/gorm"
)

const (
	patternDstOnly  = "DST_ONLY"
	patternSrcOnly  = "SRC_ONLY"
	patternDstFirst = "DST_FIRST"
	patternSrcFirst = "SRC_FIRST"
)

// DoubleWrite 双写模式
type DoubleWrite struct {
	src     gorm.ConnPool
	dst     gorm.ConnPool
	pattern atomic.Value
}

// NewDoubleWrite 创建一个新的双写模式
func NewDoubleWrite(src, dst gorm.ConnPool, pattern string) *DoubleWrite {
	dw := &DoubleWrite{
		src: src,
		dst: dst,
	}
	dw.pattern.Store(pattern)
	return dw
}

// UpdatePattern 更新双写模式
func (d *DoubleWrite) UpdatePattern(pattern string) {
	d.pattern.Store(pattern)
}

// BeginTx gorm的事务接口
func (d *DoubleWrite) BeginTx(ctx context.Context, opts *sql.TxOptions) (gorm.ConnPool, error) {
	pattern := d.pattern.Load().(string)
	switch pattern {
	case patternSrcOnly:
		tx, err := d.src.(gorm.TxBeginner).BeginTx(ctx, opts)
		if err != nil {
			return nil, err
		}
		return &DoubleWriteTx{
			src:     tx,
			pattern: pattern,
			DoubleWrite: DoubleWrite{
				src:     d.src,
				pattern: d.pattern,
			},
		}, nil
	case patternSrcFirst:
		srcTx, err := d.src.(gorm.TxBeginner).BeginTx(ctx, opts)
		if err != nil {
			return nil, err
		}
		dstTx, err := d.dst.(gorm.TxBeginner).BeginTx(ctx, opts)
		if err != nil {
			log.Println("双写模式下，目标表开启事务失败", err)
		}
		return &DoubleWriteTx{
			src:     srcTx,
			dst:     dstTx,
			pattern: pattern,
			DoubleWrite: DoubleWrite{
				src:     d.src,
				dst:     d.dst,
				pattern: d.pattern,
			},
		}, nil
	case patternDstOnly:
		tx, err := d.dst.(gorm.TxBeginner).BeginTx(ctx, opts)
		if err != nil {
			return nil, err
		}
		return &DoubleWriteTx{
			dst:     tx,
			pattern: pattern,
			DoubleWrite: DoubleWrite{
				dst:     d.dst,
				pattern: d.pattern,
			},
		}, nil
	case patternDstFirst:
		dstTx, err := d.dst.(gorm.TxBeginner).BeginTx(ctx, opts)
		if err != nil {
			return nil, err
		}
		srcTx, err := d.src.(gorm.TxBeginner).BeginTx(ctx, opts)
		if err != nil {
			log.Println("双写模式下，源表开启事务失败", err)
		}
		return &DoubleWriteTx{
			src:     srcTx,
			dst:     dstTx,
			pattern: pattern,
			DoubleWrite: DoubleWrite{
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
func (d *DoubleWrite) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	panic("没有这种双写模式")
}

// ExecContext gorm的增删改的查询接口
func (d *DoubleWrite) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	pattern, ok := d.pattern.Load().(string)
	if !ok {
		pattern = patternSrcOnly // 默认使用源库
	}
	switch pattern {
	case patternSrcOnly:
		return d.src.ExecContext(ctx, query, args...)
	case patternSrcFirst:
		res, err := d.src.ExecContext(ctx, query, args...)
		if err != nil {
			return res, err
		}
		_, err = d.dst.ExecContext(ctx, query, args...)
		if err != nil {
			log.Println("双写模式下，目标库执行失败", err)
		}
		return res, nil
	case patternDstOnly:
		return d.dst.ExecContext(ctx, query, args...)
	case patternDstFirst:
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
func (d *DoubleWrite) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	pattern, ok := d.pattern.Load().(string)
	if !ok {
		pattern = patternSrcOnly // 默认使用源库
	}
	switch pattern {
	case patternSrcOnly, patternSrcFirst:
		return d.src.QueryContext(ctx, query, args...)
	case patternDstOnly, patternDstFirst:
		return d.dst.QueryContext(ctx, query, args...)
	default:
		panic("未知的双写模式")
	}
}

// QueryRowContext gorm的单行查询接口
func (d *DoubleWrite) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	pattern, ok := d.pattern.Load().(string)
	if !ok {
		pattern = patternSrcOnly // 默认使用源库
	}
	switch pattern {
	case patternSrcOnly, patternSrcFirst:
		return d.src.QueryRowContext(ctx, query, args...)
	case patternDstOnly, patternDstFirst:
		return d.dst.QueryRowContext(ctx, query, args...)
	default:
		panic("未知的双写模式")
	}
}

// DoubleWriteTx 双写模式的事务
type DoubleWriteTx struct {
	src     gorm.Tx
	dst     gorm.Tx
	pattern string
	DoubleWrite
}

// Commit 提交事务
func (d *DoubleWriteTx) Commit() error {
	switch d.pattern {
	case patternSrcOnly:
		return d.src.Commit()
	case patternSrcFirst:
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
	case patternDstOnly:
		return d.dst.Commit()
	case patternDstFirst:
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
func (d *DoubleWriteTx) Rollback() error {
	switch d.pattern {
	case patternSrcOnly:
		return d.src.Rollback()
	case patternSrcFirst:
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
	case patternDstOnly:
		return d.dst.Rollback()
	case patternDstFirst:
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
