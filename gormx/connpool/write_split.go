package connpool

import (
	"context"
	"database/sql"
	"sync/atomic"

	"gorm.io/gorm"
)

// WriteSplit 读写分离或主从模式
type WriteSplit struct {
	master  gorm.ConnPool
	slave   []gorm.ConnPool
	current uint64
}

// NewWriteSplit 创建一个新的 WriteSplit 实例
func NewWriteSplit(master gorm.ConnPool, slaves ...gorm.ConnPool) *WriteSplit {
	return &WriteSplit{
		master: master,
		slave:  slaves,
	}
}

func (w *WriteSplit) BeginTx(ctx context.Context, opts *sql.TxOptions) (gorm.ConnPool, error) {
	return w.master.(gorm.TxBeginner).BeginTx(ctx, opts)
}

func (w *WriteSplit) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return w.master.PrepareContext(ctx, query)
}

func (w *WriteSplit) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return w.master.ExecContext(ctx, query, args...)
}

func (w *WriteSplit) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	// 使用原子操作获取当前索引
	idx := atomic.AddUint64(&w.current, 1) % uint64(len(w.slave))
	// 获取对应的从库连接
	slave := w.slave[idx]
	// 执行查询
	rows, err := slave.QueryContext(ctx, query, args...)
	if err != nil {
		// 如果查询失败，尝试其他从库
		for i := 1; i < len(w.slave); i++ {
			nextIdx := (idx + uint64(i)) % uint64(len(w.slave))
			nextSlave := w.slave[nextIdx]
			rows, err = nextSlave.QueryContext(ctx, query, args...)
			if err == nil {
				return rows, nil
			}
		}
		// 所有从库都失败，返回错误
		return nil, err
	}
	return rows, nil
}

func (w *WriteSplit) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	// 使用原子操作获取当前索引
	idx := atomic.AddUint64(&w.current, 1) % uint64(len(w.slave))
	// 获取对应的从库连接
	slave := w.slave[idx]
	// 执行查询
	row := slave.QueryRowContext(ctx, query, args...)
	if row != nil {
		return row
	}
	// 如果查询失败，尝试其他从库
	for i := 1; i < len(w.slave); i++ {
		nextIdx := (idx + uint64(i)) % uint64(len(w.slave))
		nextSlave := w.slave[nextIdx]
		row = nextSlave.QueryRowContext(ctx, query, args...)
		if row != nil {
			return row
		}
	}
	return nil
}
