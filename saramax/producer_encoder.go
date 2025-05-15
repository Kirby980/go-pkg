package saramax

import (
	"encoding/json"

	"github.com/Kirby980/study/webook/pkg/logger"
)

type JsonEncoder struct {
	l    logger.LoggerV1
	Data any
}

func (j JsonEncoder) Encode() ([]byte, error) {
	s, err := json.Marshal(j.Data)
	if err != nil {
		j.l.Error("反序列化失败", logger.Error(err))
		return nil, err
	}
	return []byte(s), nil
}

func (j JsonEncoder) Length() int {
	s, err := json.Marshal(j.Data)
	if err != nil {
		j.l.Error("反序列化失败", logger.Error(err))
		return 0
	}
	return len(s)
}
func NewJsonEncoder(l logger.LoggerV1, data any) JsonEncoder {
	return JsonEncoder{
		l:    l,
		Data: data,
	}
}
