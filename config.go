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
	GITHUB_COMMON_URL           string = "https://www.githubstatus.com"
	GITHUB_ALL_INCIDENTS        string = "/api/v2/incidents.json"
	GITHUB_UNRESOLVED_INCIDENTS string = "/api/v2/incidents/unresolved.json"

	SAMPLE_ALL_INCIDENTS        string = "./testdata/all_incidents.json"
	SAMPLE_UNRESOLVED_INCIDENTS string = "./testdata/unresolved_incidents.json"

	ENV_USE_LOCAL_SAMPLE    string = "USE_LOCAL_SAMPLE"
	ENV_LOG_RETENTION_DAYS  string = "LOG_RETENTION_DAYS"
	ENV_DISCORD_WEBHOOK_URL string = "DISCORD_WEBHOOK_URL"
	ENV_SLACK_WEBHOOK_URL   string = "SLACK_WEBHOOK_URL"

	DOTENV_FILE_PATH         string = "./.env"
	LOG_FOLDER_PATH          string = "./logs/"
	NOTICE_FILE_PATH         string = "./notice_message.json"
	NOTIFIED_STATE_FILE_PATH string = "./notified_incidents.json"

	LOG_FILE_PREFIX      string = "log-"
	LOG_FILE_EXTENSION   string = ".log"
	LOG_FILE_DATE_FORMAT string = "20060102"

	// DEFAULT_LOG_RETENTION_DAYS はログの既定の保持日数 (当日を含む)。
	DEFAULT_LOG_RETENTION_DAYS int = 30

	// MAX_NOTIFY_INCIDENTS は 1 回の通知に含めるインシデントの上限 (Discord の embeds 上限に合わせる)。
	MAX_NOTIFY_INCIDENTS int = 10

	// WEBHOOK_ERROR_BODY_LIMIT はエラー時に読み取るレスポンスボディの上限バイト数。
	WEBHOOK_ERROR_BODY_LIMIT int64 = 512

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
