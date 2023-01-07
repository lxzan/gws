package internal

import "log"

type Logger struct{}

func (c *Logger) Errorf(format string, v ...interface{}) {
	log.Default().Printf(format, v...)
}
