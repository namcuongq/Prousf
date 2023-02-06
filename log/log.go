package log

import (
	"log"
)

const (
	VERSION = "1.3.0"
	RELEASE = "(04/02/2023)"

	LevelDebug = iota
	LevelInfo
	LevelWarning
	LevelError
)

var level = LevelDebug

func SetLevel(l int) {
	level = l
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
