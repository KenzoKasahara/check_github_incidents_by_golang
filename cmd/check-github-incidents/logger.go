package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	LOG_FOLDER_PATH string = "./logs/"

	LOG_FILE_PREFIX      string = "log-"
	LOG_FILE_EXTENSION   string = ".log"
	LOG_FILE_DATE_FORMAT string = "20060102"
)

// LoggingSettings は標準出力とログファイルの両方へログを出力するよう設定する。
func LoggingSettings(logFile string) {
	logfile, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalln("[ERROR]", "log file open error:", err)
	}

	multiLogFile := io.MultiWriter(os.Stdout, logfile)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.SetOutput(multiLogFile)
}

// CreateFolder はカレントディレクトリと同じパーミッションでフォルダを作成する。
func CreateFolder(createFolderPath string) error {
	fileInfo, err := os.Lstat("./")
	if err != nil {
		return fmt.Errorf("カレントディレクトリの情報取得に失敗しました: %w", err)
	}

	fileMode := fileInfo.Mode()
	unixPerms := fileMode & os.ModePerm
	if err := os.MkdirAll(createFolderPath, unixPerms); err != nil {
		return fmt.Errorf("フォルダの作成に失敗しました (%v): %w", createFolderPath, err)
	}

	return nil
}

// LogFileName は指定日のログファイル名を組み立てる。
func LogFileName(logFolderPath string, now time.Time) string {
	return filepath.Join(logFolderPath, LOG_FILE_PREFIX+now.Format(LOG_FILE_DATE_FORMAT)+LOG_FILE_EXTENSION)
}

// CleanupOldLogs は保持期間を過ぎたログファイルを削除する。
// 保持期間は当日を含めた日数で、retentionDays が 0 以下の場合は削除しない。
// log-YYYYMMDD.log の形式に一致しないファイルは対象外とする。
func CleanupOldLogs(logFolderPath string, retentionDays int, now time.Time) (int, error) {
	if retentionDays <= 0 {
		return 0, nil
	}

	entries, err := os.ReadDir(logFolderPath)
	if err != nil {
		return 0, fmt.Errorf("ログフォルダの読み込みに失敗しました (%v): %w", logFolderPath, err)
	}

	// 当日を含めて retentionDays 日分を残すため、この日付より前のログを削除する
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	threshold := today.AddDate(0, 0, -(retentionDays - 1))

	deleted := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		logDate, ok := ParseLogFileDate(entry.Name())
		if !ok || !logDate.Before(threshold) {
			continue
		}

		logFilePath := filepath.Join(logFolderPath, entry.Name())
		if err := os.Remove(logFilePath); err != nil {
			return deleted, fmt.Errorf("ログファイルの削除に失敗しました (%v): %w", logFilePath, err)
		}
		deleted++
	}

	return deleted, nil
}

// ParseLogFileDate は log-YYYYMMDD.log というファイル名から日付を取り出す。
func ParseLogFileDate(fileName string) (time.Time, bool) {
	if !strings.HasPrefix(fileName, LOG_FILE_PREFIX) || !strings.HasSuffix(fileName, LOG_FILE_EXTENSION) {
		return time.Time{}, false
	}

	dateText := strings.TrimSuffix(strings.TrimPrefix(fileName, LOG_FILE_PREFIX), LOG_FILE_EXTENSION)
	logDate, err := time.Parse(LOG_FILE_DATE_FORMAT, dateText)
	if err != nil {
		return time.Time{}, false
	}

	return logDate, true
}
