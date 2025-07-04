package fixer

import (
	"context"
	"errors"

	"github.com/Kirby980/go-pkg/migrator"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OverrideFixer[T migrator.Entity] struct {
	// 因为本身其实这个不涉及什么领域对象，
	// 这里操作的不是 migrator 本身的领域对象
	base    *gorm.DB
	target  *gorm.DB
	columns []string
}

// NewOverrideFixer 用于获取 T 中的所有字段
func NewOverrideFixer[T migrator.Entity](base *gorm.DB,
	target *gorm.DB) (*OverrideFixer[T], error) {
	// 在这里需要查询一下数据库中究竟有哪些列
	var t T
	rows, err := base.Model(&t).Limit(1).Rows()
	if err != nil {
		return nil, err
	}
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	return &OverrideFixer[T]{
		base:    base,
		target:  target,
		columns: columns,
	}, nil
}

// Fix 用于修复数据
// 相当于event是一个触发器，不依赖于event的具体内容，只根据id来修改
func (o *OverrideFixer[T]) Fix(ctx context.Context, id int64) error {
	var src T
	// 找出数据
	err := o.base.WithContext(ctx).Where("id = ?", id).
		First(&src).Error
	switch err {
	// 找到了数据
	case nil:
		// 使用 Upsert 操作，如果存在则更新，不存在则插入
		result := o.target.Clauses(&clause.OnConflict{
			// 我们需要 Entity 告诉我们，修复哪些数据
			DoUpdates: clause.AssignmentColumns(o.columns),
		}).Create(&src)
		if result.Error != nil {
			return result.Error
		}
		// 检查是否真的插入了数据
		if result.RowsAffected == 0 {
			return errors.New("修复数据失败：没有影响任何行")
		}
		return nil
	case gorm.ErrRecordNotFound:
		// 源表没有数据，删除目标表中的数据
		result := o.target.Delete("id = ?", id)
		if result.Error != nil {
			return result.Error
		}
		// 检查是否真的删除了数据
		if result.RowsAffected == 0 {
			return errors.New("删除数据失败：没有影响任何行")
		}
		return nil
	default:
		return err
	}
}
