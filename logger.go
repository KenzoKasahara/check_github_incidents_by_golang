package main

import (
	"fmt"
	"io"
	"log"
	"os"
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
