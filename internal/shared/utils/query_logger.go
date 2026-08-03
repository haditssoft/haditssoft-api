package utils

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/gorm"
)

var (
	LogMu      sync.Mutex
	logFile    *os.File
	LogWriter  *log.Logger
	reqCounter atomic.Int64
	logEnabled = true
)

func InitQueryLogger(dir string) error {
	LogMu.Lock()
	defer LogMu.Unlock()

	if logFile != nil {
		logFile.Close()
	}

	if dir != "" && !strings.HasSuffix(dir, "/") && !strings.HasSuffix(dir, "\\") {
		dir += string(os.PathSeparator)
	}
	filePath := dir + "query_" + time.Now().Format("20060102") + ".log"

	var err error
	logFile, err = os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	LogWriter = log.New(logFile, "", 0)
	logEnabled = true
	return nil
}

type QueryLogEntry struct {
	RequestID  int64  `json:"request_id"`
	Timestamp  string `json:"timestamp"`
	Tag        string `json:"tag"`
	SQL        string `json:"sql"`
	Params     string `json:"params,omitempty"`
	RowCount   int    `json:"row_count"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

func NextRequestID() int64 {
	return reqCounter.Add(1)
}

func LogQuery(requestID int64, tag, sql string, params []interface{}, rowCount int, err error, start time.Time) {
	if !logEnabled {
		return
	}

	entry := QueryLogEntry{
		RequestID:  requestID,
		Timestamp:  start.Format(time.RFC3339Nano),
		Tag:        tag,
		SQL:        sql,
		RowCount:   rowCount,
		DurationMs: time.Since(start).Milliseconds(),
	}
	if len(params) > 0 {
		parts := make([]string, len(params))
		for i, p := range params {
			parts[i] = fmt.Sprintf("%v", p)
		}
		entry.Params = "[" + strings.Join(parts, ", ") + "]"
	}
	if err != nil {
		entry.Error = err.Error()
	}

	LogMu.Lock()
	if LogWriter != nil {
		LogWriter.Printf(
			"%s\t%d\t%s\t%s\t%s\t%d\t%dms\t%s",
			entry.Timestamp,
			entry.RequestID,
			entry.Tag,
			entry.SQL,
			entry.Params,
			entry.RowCount,
			entry.DurationMs,
			entry.Error,
		)
	}
	LogMu.Unlock()
}

func LoggedQuery(requestID int64, tag, sql string, params []interface{}, DB *gorm.DB) ([]map[string]interface{}, error) {
	start := time.Now()
	var rows []map[string]interface{}
	err := DB.Raw(sql, params...).Scan(&rows).Error
	rowCount := len(rows)
	LogQuery(requestID, tag, sql, params, rowCount, err, start)
	return rows, err
}

func LoggedQueryNoParams(requestID int64, tag, sql string, DB *gorm.DB) ([]map[string]interface{}, error) {
	start := time.Now()
	var rows []map[string]interface{}
	err := DB.Raw(sql).Scan(&rows).Error
	rowCount := len(rows)
	LogQuery(requestID, tag, sql, nil, rowCount, err, start)
	return rows, err
}

func LogSearchStart(requestID int64, kitabName, column string, originalKeywords, filtered []string) {
	start := time.Now()
	LogMu.Lock()
	if LogWriter != nil {
		LogWriter.Printf(
			"%s\t%d\t%s\t%s\t%s\t%d\t%d\t%s",
			start.Format(time.RFC3339Nano),
			requestID,
			"SEARCH_START",
			fmt.Sprintf("kitab=%s column=%s", kitabName, column),
			fmt.Sprintf("original=%s filtered=%s",
				strings.Join(originalKeywords, ","),
				strings.Join(filtered, ",")),
			0, 0, "",
		)
	}
	LogMu.Unlock()
}

func LogSearchResult(requestID int64, totalRows int) {
	start := time.Now()
	LogMu.Lock()
	if LogWriter != nil {
		LogWriter.Printf(
			"%s\t%d\t%s\t%s\t%s\t%d\t%d\t%s",
			start.Format(time.RFC3339Nano),
			requestID,
			"SEARCH_RESULT",
			"",
			"",
			totalRows, 0, "",
		)
	}
	LogMu.Unlock()
}
