package migrator

// Entity 实体
// 所有需要迁移的实体，都要实现这个接口
type Entity interface {
	ID() int64
	CompareTo(dst Entity) bool
}
