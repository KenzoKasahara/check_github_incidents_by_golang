package main

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

func TestParseLogFileDate(t *testing.T) {
	tests := []struct {
		fileName string
		want     string
		wantOK   bool
	}{
		{fileName: "log-20260826.log", want: "2026-08-26", wantOK: true},
		{fileName: "log-20240921.log", want: "2024-09-21", wantOK: true},
		{fileName: "log-2026082.log", wantOK: false},
		{fileName: "log-notadate.log", wantOK: false},
		{fileName: "log-20261332.log", wantOK: false},
		{fileName: "notice_message.json", wantOK: false},
		{fileName: "log-20260826.log.bak", wantOK: false},
		{fileName: "access-20260826.log", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.fileName, func(t *testing.T) {
			logDate, ok := ParseLogFileDate(tt.fileName)

			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if got := logDate.Format("2006-01-02"); got != tt.want {
				t.Errorf("date = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCleanupOldLogs(t *testing.T) {
	logFolderPath := t.TempDir()
	fileNames := []string{
		"log-20260826.log", // 当日
		"log-20260825.log",
		"log-20260824.log", // 保持期間 3 日ならここまで残る
		"log-20260823.log", // 削除対象
		"log-20260101.log", // 削除対象
		"log-invalid.log",  // 形式が一致しないため対象外
		"notice.json",      // ログ以外のファイルは対象外
	}
	for _, fileName := range fileNames {
		if err := os.WriteFile(filepath.Join(logFolderPath, fileName), []byte("x"), 0600); err != nil {
			t.Fatalf("テスト用ファイルの作成に失敗しました: %v", err)
		}
	}

	now := time.Date(2026, 8, 26, 18, 30, 0, 0, time.Local)
	deleted, err := CleanupOldLogs(logFolderPath, 3, now)
	if err != nil {
		t.Fatalf("CleanupOldLogs() error = %v", err)
	}
	if deleted != 2 {
		t.Errorf("削除件数 = %v, want 2", deleted)
	}

	want := []string{
		"log-20260824.log",
		"log-20260825.log",
		"log-20260826.log",
		"log-invalid.log",
		"notice.json",
	}
	if got := listFileNames(t, logFolderPath); !equalStrings(got, want) {
		t.Errorf("残存ファイル = %v, want %v", got, want)
	}
}

// 保持日数に 0 を指定した場合は削除しないことを確認する。
func TestCleanupOldLogsDisabled(t *testing.T) {
	logFolderPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(logFolderPath, "log-20200101.log"), []byte("x"), 0600); err != nil {
		t.Fatalf("テスト用ファイルの作成に失敗しました: %v", err)
	}

	deleted, err := CleanupOldLogs(logFolderPath, 0, time.Date(2026, 8, 26, 0, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatalf("CleanupOldLogs() error = %v", err)
	}
	if deleted != 0 {
		t.Errorf("削除件数 = %v, want 0", deleted)
	}
	if got := listFileNames(t, logFolderPath); len(got) != 1 {
		t.Errorf("残存ファイル = %v, want 1 件", got)
	}
}

// 実行中のログファイル (当日分) が削除されないことを確認する。
func TestCleanupOldLogsKeepsToday(t *testing.T) {
	logFolderPath := t.TempDir()
	now := time.Date(2026, 8, 26, 0, 0, 0, 0, time.Local)
	todayLog := LogFileName(logFolderPath, now)
	if err := os.WriteFile(todayLog, []byte("x"), 0600); err != nil {
		t.Fatalf("テスト用ファイルの作成に失敗しました: %v", err)
	}

	if _, err := CleanupOldLogs(logFolderPath, 1, now); err != nil {
		t.Fatalf("CleanupOldLogs() error = %v", err)
	}
	if _, err := os.Stat(todayLog); err != nil {
		t.Errorf("当日のログが残っていること: %v", err)
	}
}

func TestLogFileName(t *testing.T) {
	now := time.Date(2026, 8, 26, 9, 0, 0, 0, time.Local)

	want := filepath.Join("./logs/", "log-20260826.log")
	if got := LogFileName("./logs/", now); got != want {
		t.Errorf("LogFileName() = %v, want %v", got, want)
	}
}

func listFileNames(t *testing.T, folderPath string) []string {
	t.Helper()

	entries, err := os.ReadDir(folderPath)
	if err != nil {
		t.Fatalf("フォルダの読み込みに失敗しました: %v", err)
	}

	fileNames := []string{}
	for _, entry := range entries {
		fileNames = append(fileNames, entry.Name())
	}
	sort.Strings(fileNames)

	return fileNames
}

func equalStrings(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}

	return true
}
