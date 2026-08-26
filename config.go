package main

import (
	"flag"
	"log"
	"os"
	"strconv"
	"time"
)

const (
	GITHUB_COMMON_URL           string = "https://www.githubstatus.com"
	GITHUB_ALL_INCIDENTS        string = "/api/v2/incidents.json"
	GITHUB_UNRESOLVED_INCIDENTS string = "/api/v2/incidents/unresolved.json"

	SAMPLE_ALL_INCIDENTS        string = "./testdata/all_incidents.json"
	SAMPLE_UNRESOLVED_INCIDENTS string = "./testdata/unresolved_incidents.json"

	ENV_USE_LOCAL_SAMPLE string = "USE_LOCAL_SAMPLE"

	LOG_FOLDER_PATH  string = "./logs/"
	NOTICE_FILE_PATH string = "./notice_message.json"

	HTTP_TIMEOUT time.Duration = 10 * time.Second
)

// LocalSampleOption はコマンドライン引数 -local の指定状態を表す。
type LocalSampleOption struct {
	Value    bool // 指定された値
	Explicit bool // コマンドライン引数で明示的に指定されたか
}

// ParseLocalSampleFlag はコマンドライン引数を解析する。
// flag.Parse() を伴うため、main から一度だけ呼び出すこと。
func ParseLocalSampleFlag() LocalSampleOption {
	localFlag := flag.Bool("local", false, "ローカルのサンプルファイル(./testdata/*.json)を使用する")
	flag.Parse()

	explicit := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "local" {
			explicit = true
		}
	})

	return LocalSampleOption{Value: *localFlag, Explicit: explicit}
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
