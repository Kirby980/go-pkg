package connpool

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestDoubleWrite(t *testing.T) {
	db, err := gorm.Open(mysql.Open("root:root@tcp(localhost:13316)/webook?parseTime=true&loc=Local"))
	require.NoError(t, err)
	intr, err := gorm.Open(mysql.Open("root:root@tcp(localhost:13316)/webook_intr?parseTime=true&loc=Local"))
	require.NoError(t, err)

	db2, err := gorm.Open(mysql.New(mysql.Config{
		Conn: NewDoubleWritePool(db.ConnPool, intr.ConnPool, PatternSrcFirst),
	}))
	require.NoError(t, err)
	err = db2.Create(&Interactive{
		BizID: 11,
		Biz:   "test1",
		Ctime: time.Now().Unix(),
		Utime: time.Now().Unix(),
	}).Error
	require.NoError(t, err)

	err = db2.Transaction(func(tx *gorm.DB) error {
		return tx.Create(&Interactive{
			BizID: 21,
			Biz:   "test-tx1",
			Ctime: time.Now().Unix(),
			Utime: time.Now().Unix(),
		}).Error
	})
	require.NoError(t, err)
}

type Interactive struct {
	Id int64 `gorm:"column:id;primaryKey;autoIncrement"`
	//业务标识符
	BizID int64  `gorm:"column:biz_id;uniqueIndex:idx_biz"`
	Biz   string `gorm:"type:varchar(128);column:biz;uniqueIndex:idx_biz"`
	//阅读计数
	ReadCnt    int64 `gorm:"column:read_cnt"`
	LikeCnt    int64 `gorm:"column:like_cnt"`
	CollectCnt int64 `gorm:"column:collect_cnt"`
	Ctime      int64 `gorm:"column:ctime"`
	Utime      int64 `gorm:"column:utime"`
}
