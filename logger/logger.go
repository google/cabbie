package logger

import (
	"fmt"
	"io"

	"github.com/google/cabbie/cablib"
	googleLogger "github.com/google/logger"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
)

type cabbieLogger struct {
	googLogger googleLogger.Logger
	elog       debug.Log
}

func (log cabbieLogger) Info(eid uint32, msg string) error {
	if err := log.elog.Info(eid, msg); err != nil {
		return err
	}
	log.googLogger.Info(msg)
	return nil
}

func (log cabbieLogger) Close() error {
	if err := log.elog.Close(); err != nil {
		return err
	}
	log.googLogger.Close()
	return nil
}

func (log cabbieLogger) Warning(eid uint32, msg string) error {
	if err := log.elog.Warning(eid, msg); err != nil {
		return err
	}
	log.googLogger.Warning(msg)
	return nil
}

func (log cabbieLogger) Error(eid uint32, msg string) error {
	if err := log.elog.Error(eid, msg); err != nil {
		return err
	}
	log.googLogger.Error(msg)
	return nil
}

func NewLogger(stdout bool, runInDebug bool) (*cabbieLogger, error) {
	var err error
	logger := cabbieLogger{}
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
	// if stdout == true, log to stdout but not eventlog; that's handled in logger.elog
	gl := googleLogger.Init(cablib.LogSrcName, stdout, false, io.Discard)
	logger.googLogger = *gl
	return &logger, nil
}
