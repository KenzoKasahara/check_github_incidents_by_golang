package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// LoadDotEnv は .env ファイルを読み込み、環境変数として設定する。
// .env に書かれた値を優先し、すでに設定されている環境変数を上書きする。
// シェルに残った古い設定が意図せず使われるのを防ぐため、この優先順位としている。
// ファイルが存在しない場合はエラーとせず、何もしない。
//
// 対応する記法は以下のとおり。
//   - KEY=VALUE / export KEY=VALUE
//   - 空行、および # で始まるコメント行
//   - 値全体を "" または ” で囲んだ場合はクォートを取り除く
//
// 行途中の # はコメントとして扱わない (URL などに含まれる場合があるため)。
func LoadDotEnv(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("ファイルのオープンに失敗しました (%v): %w", filePath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++

		key, value, ok, err := ParseDotEnvLine(scanner.Text())
		if err != nil {
			return fmt.Errorf("%v の %v 行目の解析に失敗しました: %w", filePath, lineNumber, err)
		}
		if !ok {
			continue
		}

		// .env の値でシェルの環境変数を上書きする
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("環境変数の設定に失敗しました (%v): %w", key, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("ファイルの読み込みに失敗しました (%v): %w", filePath, err)
	}

	return nil
}

// ParseDotEnvLine は .env の 1 行を解析する。
// 空行とコメント行は ok = false を返す。
func ParseDotEnvLine(line string) (key string, value string, ok bool, err error) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false, nil
	}
	trimmed = strings.TrimPrefix(trimmed, "export ")

	separator := strings.Index(trimmed, "=")
	if separator < 0 {
		return "", "", false, fmt.Errorf("KEY=VALUE の形式ではありません: %v", line)
	}

	key = strings.TrimSpace(trimmed[:separator])
	if key == "" {
		return "", "", false, fmt.Errorf("キーが空です: %v", line)
	}

	value = TrimSurroundingQuotes(strings.TrimSpace(trimmed[separator+1:]))

	return key, value, true, nil
}

// TrimSurroundingQuotes は値全体を囲むクォートを取り除く。
func TrimSurroundingQuotes(value string) string {
	if len(value) < 2 {
		return value
	}

	for _, quote := range []string{`"`, `'`} {
		if strings.HasPrefix(value, quote) && strings.HasSuffix(value, quote) {
			return value[1 : len(value)-1]
		}
	}

	return value
}
