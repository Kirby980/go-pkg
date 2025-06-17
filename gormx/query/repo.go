package query

import (
	"context"
	"errors"
	"time"

	"github.com/Kirby980/study/webook/pkg/gormx/cache"
	"gorm.io/gorm"
)

type Repo struct {
	DB *gorm.DB
}

// Update 更新数据
func (r *Repo) Update(ctx context.Context, model interface{}) (err error) {
	return r.DB.WithContext(ctx).Updates(model).Error
}

// Delete 删除数据
func (r *Repo) Delete(ctx context.Context, model interface{}) (err error) {
	return r.DB.WithContext(ctx).Delete(model).Error
}

// Insert 插入数据
func (r *Repo) Insert(ctx context.Context, model interface{}) (err error) {
	return r.DB.WithContext(ctx).Create(model).Error
}

// Get 获取数据
func (r *Repo) Get(ctx context.Context, id uint, model interface{}) (err error) {
	err = r.DB.WithContext(ctx).Where("id = ?", id).First(model).Error
	return
}

// Preload 预加载数据
func (r *Repo) Preload(ctx context.Context, id uint, q *QueryParams, model interface{}) (err error) {
	db := r.DB.WithContext(ctx)
	for _, preload := range q.Preload {
		db = db.Preload(preload)
	}
	err = db.Where("id = ?", id).Find(model).Error
	return
}

// List 列表数据
func (r *Repo) List(ctx context.Context, models interface{}) error {
	return r.DB.WithContext(ctx).Find(models).Error
}

// Clone 克隆数据库
func (r *Repo) Clone(db *gorm.DB) *Repo {
	return &Repo{
		DB: db,
	}
}

// WithContext 设置上下文
func (r *Repo) WithContext(ctx context.Context) *Repo {
	return &Repo{
		DB: r.DB.WithContext(ctx),
	}
}

// Debug 调试模式
func (r *Repo) Debug() *Repo {
	return &Repo{
		DB: r.DB.Debug(),
	}
}

// WithCache 设置缓存
func (r *Repo) WithCache(key string, expire time.Duration) *Repo {
	return r.Clone(r.DB.Set(cache.CacheParamKey, cache.CacheParam{
		Key:     key,
		Expires: expire,
	}))
}

// ExecSQL 执行SQL
func (r *Repo) ExecSQL(ctx context.Context, sql string, values ...interface{}) (err error) {
	db := r.DB.Exec(sql, values...)
	if db.Error != nil {
		err = db.Error
		return
	}
	if db.RowsAffected == 0 {
		err = errors.New("no rows affected")
	}
	return

}

// Query 查询数据
func (r *Repo) Query(ctx context.Context, models interface{}, params *QueryParams, queryable ...string) (total int64, err error) {
	// 默认排序
	if len(params.Order) == 0 {
		params.Order = "id desc"
	}

	db := r.DB.WithContext(ctx).Limit(params.Limit).Offset(params.Offset).Order(params.Order)

	if params.Select != "" {
		db = db.Select(params.Select)
	}

	if len(params.Joins) > 0 {
		db = db.Joins(params.Joins)
	}

	if len(params.Group) > 0 {
		db = db.Group(params.Group)
	}

	if len(params.Having) > 0 {
		db = db.Having(params.Having)
	}

	plains := params.Query.Plains(queryable...)
	if len(plains) > 0 {
		db = db.Where(plains[0], plains[1:]...)
	}

	if len(params.CustomQuery) != 0 {
		for queryStr, queryValue := range params.CustomQuery {
			if len(queryValue) == 0 {
				db = db.Where(queryStr)
			} else {
				db = db.Where(queryStr, queryValue...)
			}
		}
	}
	if len(params.Preload) > 0 {
		for _, populate := range params.Preload {
			db = db.Preload(populate)
		}
	}
	if len(params.TableName) > 0 {
		db = db.Table(params.TableName)
	}

	err = db.Find(models).Error
	if err != nil {
		return
	}

	err = db.Limit(-1).Offset(-1).Count(&total).Error
	return
}
