package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDotEnvLine(t *testing.T) {
	tests := []struct {
		line      string
		wantKey   string
		wantValue string
		wantOK    bool
		wantErr   bool
	}{
		{line: `KEY=value`, wantKey: "KEY", wantValue: "value", wantOK: true},
		{line: `  KEY = value  `, wantKey: "KEY", wantValue: "value", wantOK: true},
		{line: `export KEY=value`, wantKey: "KEY", wantValue: "value", wantOK: true},
		{line: `KEY="value"`, wantKey: "KEY", wantValue: "value", wantOK: true},
		{line: `KEY='value'`, wantKey: "KEY", wantValue: "value", wantOK: true},
		{line: `KEY=`, wantKey: "KEY", wantValue: "", wantOK: true},
		// 値に = が含まれても最初の = だけで分割する
		{line: `URL=https://example.com/a?b=c`, wantKey: "URL", wantValue: "https://example.com/a?b=c", wantOK: true},
		// 行途中の # はコメントとして扱わない
		{line: `URL=https://example.com/a#b`, wantKey: "URL", wantValue: "https://example.com/a#b", wantOK: true},
		{line: ``, wantOK: false},
		{line: `   `, wantOK: false},
		{line: `# comment`, wantOK: false},
		{line: `INVALID`, wantErr: true},
		{line: `=value`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			key, value, ok, err := ParseDotEnvLine(tt.line)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("エラーが返ること: line = %v", tt.line)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDotEnvLine(%v) error = %v", tt.line, err)
			}
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if key != tt.wantKey || value != tt.wantValue {
				t.Errorf("= (%v, %v), want (%v, %v)", key, value, tt.wantKey, tt.wantValue)
			}
		})
	}
}

func TestLoadDotEnv(t *testing.T) {
	dotEnvPath := filepath.Join(t.TempDir(), ".env")
	content := "# コメント\n\nDOTENV_TEST_NEW=from-dotenv\nDOTENV_TEST_EXISTING=from-dotenv\n"
	if err := os.WriteFile(dotEnvPath, []byte(content), 0600); err != nil {
		t.Fatalf("テスト用 .env の作成に失敗しました: %v", err)
	}

	// すでに設定済みの環境変数が .env の値で上書きされることを確認する
	t.Setenv("DOTENV_TEST_EXISTING", "from-environment")

	if err := LoadDotEnv(dotEnvPath); err != nil {
		t.Fatalf("LoadDotEnv() error = %v", err)
	}
	t.Cleanup(func() { os.Unsetenv("DOTENV_TEST_NEW") })

	if got := os.Getenv("DOTENV_TEST_NEW"); got != "from-dotenv" {
		t.Errorf("DOTENV_TEST_NEW = %v, want from-dotenv", got)
	}
	if got := os.Getenv("DOTENV_TEST_EXISTING"); got != "from-dotenv" {
		t.Errorf("DOTENV_TEST_EXISTING = %v, want from-dotenv", got)
	}
}

// .env が存在しなくてもエラーにならないことを確認する。
func TestLoadDotEnvMissingFileIsNotError(t *testing.T) {
	if err := LoadDotEnv(filepath.Join(t.TempDir(), "not-exists.env")); err != nil {
		t.Errorf("LoadDotEnv() error = %v, want nil", err)
	}
}
