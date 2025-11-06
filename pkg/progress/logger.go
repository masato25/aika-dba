package progress

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

// PhaseLogger Phase 專用日誌器
type PhaseLogger struct {
	phase        string
	progressMgr  *ProgressManager
	baseLogger   *log.Logger
	debugEnabled bool
}

// NewPhaseLogger 創建 Phase 日誌器
func NewPhaseLogger(phase string, progressMgr *ProgressManager, debugEnabled bool) *PhaseLogger {
	return &PhaseLogger{
		phase:        phase,
		progressMgr:  progressMgr,
		baseLogger:   log.New(os.Stdout, fmt.Sprintf("[%s] ", strings.ToUpper(phase)), log.LstdFlags),
		debugEnabled: debugEnabled,
	}
}

// Info 記錄信息級別日誌
func (pl *PhaseLogger) Info(message string) {
	pl.baseLogger.Println("INFO:", message)
	if pl.progressMgr != nil {
		pl.progressMgr.AddLog(pl.phase, "info", message)
	}
}

// Debug 記錄調試級別日誌
func (pl *PhaseLogger) Debug(message string) {
	if pl.debugEnabled {
		pl.baseLogger.Println("DEBUG:", message)
		pl.progressMgr.AddLog(pl.phase, "debug", message)
	}
}

// Warn 記錄警告級別日誌
func (pl *PhaseLogger) Warn(message string) {
	pl.baseLogger.Println("WARN:", message)
	if pl.progressMgr != nil {
		pl.progressMgr.AddLog(pl.phase, "warn", message)
	}
}

// Error 記錄錯誤級別日誌
func (pl *PhaseLogger) Error(message string) {
	pl.baseLogger.Println("ERROR:", message)
	if pl.progressMgr != nil {
		pl.progressMgr.AddLog(pl.phase, "error", message)
	}
}

// Printf 格式化輸出
func (pl *PhaseLogger) Printf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	pl.baseLogger.Println(message)
	if pl.progressMgr != nil {
		pl.progressMgr.AddLog(pl.phase, "info", message)
	}
}

// MultiWriter 多重寫入器
type MultiWriter struct {
	writers []io.Writer
}

// NewMultiWriter 創建多重寫入器
func NewMultiWriter(writers ...io.Writer) *MultiWriter {
	return &MultiWriter{writers: writers}
}

// Write 寫入數據到所有寫入器
func (mw *MultiWriter) Write(p []byte) (n int, err error) {
	for _, w := range mw.writers {
		if n, err = w.Write(p); err != nil {
			return
		}
	}
	return len(p), nil
}
