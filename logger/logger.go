package logger

import (
	"fmt"
	"io"

	"github.com/google/cabbie/cablib"
	googleLogger "github.com/google/logger"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
)

type logImp struct {
	googLogger googleLogger.Logger
	elog       debug.Log
}

func (log logImp) Info(eid uint32, msg string) error {
	if err := log.elog.Info(eid, msg); err != nil {
		return err
	}
	log.googLogger.Info(msg)
	return nil
}

func (log logImp) Close() error {
	if err := log.elog.Close(); err != nil {
		return err
	}
	log.googLogger.Close()
	return nil
}

func (log logImp) Warning(eid uint32, msg string) error {
	if err := log.elog.Warning(eid, msg); err != nil {
		return err
	}
	log.googLogger.Warning(msg)
	return nil
}

func (log logImp) Error(eid uint32, msg string) error {
	if err := log.elog.Error(eid, msg); err != nil {
		return err
	}
	log.googLogger.Error(msg)
	return nil
}

func NewLogger(runInDebug bool) (*logImp, error) {
	var err error
	logger := logImp{}
	var debugLog debug.Log
	if runInDebug {
		debugLog = debug.New(cablib.LogSrcName)
	} else {
		debugLog, err = eventlog.Open(cablib.LogSrcName)
		if err != nil {
			fmt.Printf("Failed to create event: %v", err)
			return nil, err
		}
	}
	if debugLog == nil {
		fmt.Println("debugLog is nil")
	}
	logger.elog = debugLog
	if logger.elog == nil {
		fmt.Println("logger.elog is nil")
	}
	gl := googleLogger.Init(cablib.LogSrcName, true, false, io.Discard)
	logger.googLogger = *gl
	return &logger, nil
}
