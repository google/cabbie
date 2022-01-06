package logger

import (
	"fmt"
	"io"

	"github.com/google/cabbie/cablib"
	googleLogger "github.com/google/logger"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
)

// cabbieLogger enables logging to both googleLogger.Logger and debug.Log
// cabbieLogger satisfies debug.Log interface
type cabbieLogger struct {
	googLogger googleLogger.Logger
	elog       debug.Log
}

// Info logs with the Info severity.
// msg is handled in the manner of fmt.Print.
func (log cabbieLogger) Info(eid uint32, msg string) error {
	if err := log.elog.Info(eid, msg); err != nil {
		return err
	}
	log.googLogger.Info(msg)
	return nil
}

// Close closes log.elog and log.googLogger
func (log cabbieLogger) Close() error {
	if err := log.elog.Close(); err != nil {
		return err
	}
	log.googLogger.Close()
	return nil
}

// Warning logs with the Warning severity.
// msg is handled in the manner of fmt.Print.
func (log cabbieLogger) Warning(eid uint32, msg string) error {
	if err := log.elog.Warning(eid, msg); err != nil {
		return err
	}
	log.googLogger.Warning(msg)
	return nil
}

// Error logs with the Error severity.
// msg is handled in the manner of fmt.Print.
func (log cabbieLogger) Error(eid uint32, msg string) error {
	if err := log.elog.Error(eid, msg); err != nil {
		return err
	}
	log.googLogger.Error(msg)
	return nil
}

// NewLogger returns an initialized cabbieLogger or an error
func NewLogger(showOutput bool, runInDebug bool) (debug.Log, error) {
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
	if !showOutput {
		return debugLog, nil
	}

	logger.elog = debugLog
	// if stdout == true, log to stdout but not eventlog; that's handled in logger.elog
	gl := googleLogger.Init(cablib.LogSrcName, showOutput, false, io.Discard)
	logger.googLogger = *gl
	return logger, nil
}
