package logger

import "time"

func String(key string, value string) Field {
	return Field{
		Key:   key,
		Value: value,
	}
}

func Int32(key string, value int32) Field {
	return Field{
		Key:   key,
		Value: value,
	}
}
func Int64(key string, val int64) Field {
	return Field{
		Key:   key,
		Value: val,
	}
}

func Error(err error) Field {
	return Field{
		Key:   "error",
		Value: err,
	}
}

func Bool(key string, val bool) Field {
	return Field{
		Key:   key,
		Value: val,
	}

}

func Float64(key string, val float64) Field {
	return Field{
		Key:   key,
		Value: val,
	}
}

func Duration(key string, val time.Duration) Field {
	return Field{
		Key:   key,
		Value: val,
	}
}
