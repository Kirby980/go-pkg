package logger

import "sync"

var l Logger

var lMutex sync.RWMutex

func SetGolableLogger(logger Logger) {
	lMutex.Lock()
	defer lMutex.Unlock()
	l = logger
}

func L() Logger {
	lMutex.RLock()
	defer lMutex.RUnlock()
	if l == nil {
		panic("logger not set")
	}
	return l
}

var GL Logger = &NopLogger{}
