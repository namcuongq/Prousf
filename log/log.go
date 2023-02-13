package log

import (
	"log"
)

const (
	LevelTrace = iota
	LevelDebug
	LevelInfo
	LevelWarning
	LevelError
)

var level = LevelDebug

func SetLevel(l int) {
	level = l
}

func Trace(v ...interface{}) {
	if level <= LevelTrace {
		log.Println(append([]interface{}{"[Trace]"}, v...)...)
	}
}

func Debug(v ...interface{}) {
	if level <= LevelDebug {
		log.Println(append([]interface{}{"[DEBUG]"}, v...)...)
	}
}

func Info(v ...interface{}) {
	if level <= LevelInfo {
		log.Println(append([]interface{}{"[INFO]"}, v...)...)
	}
}

func Error(v ...interface{}) {
	if level <= LevelError {
		log.Println(append([]interface{}{"[ERROR]"}, v...)...)
	}
}
