package logger

import "sync"

var l LoggerV1

var lMutex sync.RWMutex

func SetGolableLogger(logger LoggerV1) {
	lMutex.Lock()
	defer lMutex.Unlock()
	l = logger
}

func L() LoggerV1 {
	lMutex.RLock()
	defer lMutex.RUnlock()
	if l == nil {
		panic("logger not set")
	}
	return l
}

var GL LoggerV1 = &NopLogger{}
