package main

import (
	"flag"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	ENV_USE_LOCAL_SAMPLE   string = "USE_LOCAL_SAMPLE"
	ENV_LOG_RETENTION_DAYS string = "LOG_RETENTION_DAYS"

	// DEFAULT_LOG_RETENTION_DAYS はログの既定の保持日数 (当日を含む)。
	DEFAULT_LOG_RETENTION_DAYS int = 30

	// HTTP_TIMEOUT は外部への HTTP リクエストのタイムアウト。
	// Status API の取得と Webhook の送信の両方で用いるため、ここに置いている。
	HTTP_TIMEOUT time.Duration = 10 * time.Second
)

// LocalSampleOption はコマンドライン引数 -local の指定状態を表す。
type LocalSampleOption struct {
	Value    bool // 指定された値
	Explicit bool // コマンドライン引数で明示的に指定されたか
}

// CommandLineOptions はコマンドライン引数の指定内容を表す。
type CommandLineOptions struct {
	LocalSample LocalSampleOption
	DryRun      bool
}

// ParseCommandLineOptions はコマンドライン引数を解析する。
// flag.Parse() を伴うため、main から一度だけ呼び出すこと。
func ParseCommandLineOptions() CommandLineOptions {
	localFlag := flag.Bool("local", false, "ローカルのサンプルファイル(./testdata/*.json)を使用する")
	dryRunFlag := flag.Bool("dry-run", false, "通知を送信せず、送信内容をログに出力する")
	flag.Parse()

	explicit := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "local" {
			explicit = true
		}
	})

	return CommandLineOptions{
		LocalSample: LocalSampleOption{Value: *localFlag, Explicit: explicit},
		DryRun:      *dryRunFlag,
	}
}

// UseLocalSample はローカルのサンプルファイルを使用するかどうかを判定する。
// 優先順位: コマンドライン引数 (-local) > 環境変数 (USE_LOCAL_SAMPLE) > 既定値 (false = 実 API)
func UseLocalSample(option LocalSampleOption) bool {
	// コマンドライン引数で明示的に指定された場合は、その値を優先する
	if option.Explicit {
		return option.Value
	}

	// 環境変数での指定 (true/false, 1/0, yes/no などを許容)
	if env, ok := os.LookupEnv(ENV_USE_LOCAL_SAMPLE); ok {
		value, err := strconv.ParseBool(env)
		if err != nil {
			log.Printf("%v %v\n", "[WARNING]", ENV_USE_LOCAL_SAMPLE+" の値が不正なため無視します: "+env)
			return false
		}
		return value
	}

	return false
}

// LogRetentionDays はログの保持日数を環境変数 (LOG_RETENTION_DAYS) から取得する。
// 未設定または不正な値の場合は既定値を用いる。0 を指定した場合は削除しない。
func LogRetentionDays() int {
	env, ok := os.LookupEnv(ENV_LOG_RETENTION_DAYS)
	if !ok {
		return DEFAULT_LOG_RETENTION_DAYS
	}

	days, err := strconv.Atoi(strings.TrimSpace(env))
	if err != nil || days < 0 {
		log.Printf("%v %v\n", "[WARNING]", ENV_LOG_RETENTION_DAYS+" の値が不正なため既定値を使用します: "+env)
		return DEFAULT_LOG_RETENTION_DAYS
	}

	return days
}
