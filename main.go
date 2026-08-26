package main

import (
	"fmt"
	"log"
	"strings"
	"time"
)

func main() {
	repeatedStars := strings.Repeat("*", 70)

	// コマンドライン引数を解析 (-h もここで処理される)
	localSampleOption := ParseLocalSampleFlag()

	// ログフォルダ作成
	if err := CreateFolder(LOG_FOLDER_PATH); err != nil {
		log.Fatalln("[ERROR]", err)
	}

	// ログファイル名を日付で作成
	now := time.Now()
	logFileName := fmt.Sprintf("%vlog-%v.log", LOG_FOLDER_PATH, now.Format("20060102"))
	LoggingSettings(logFileName)

	// 【処理開始】
	log.Println(repeatedStars)
	log.Printf("%v %v\n", "[INFO]", "【start process】")

	// データの取得元を判定 (コマンドライン引数 > 環境変数 > 既定値)
	useLocal := UseLocalSample(localSampleOption)
	if useLocal {
		log.Printf("%v %v\n", "[INFO]", "data source: local sample files")
	} else {
		log.Printf("%v %v\n", "[INFO]", "data source: "+GITHUB_COMMON_URL)
	}

	// 未解決のインシデントを取得
	unresolvedIncidents, err := GetUnResolvedIncidents(useLocal)
	if err != nil {
		log.Fatalln("[ERROR]", err)
	}

	// 過去のインシデントを取得
	historyIncidents, err := GetHistoryIncidents(useLocal)
	if err != nil {
		log.Fatalln("[ERROR]", err)
	}

	// 過去のインシデントと未解決のインシデントより、重複するインシデント情報を取得
	noticeMessages := BuildNoticeMessages(historyIncidents, unresolvedIncidents)

	// 通知メッセージをファイルに書き込む
	if err := WriteNoticeMessages(NOTICE_FILE_PATH, noticeMessages); err != nil {
		log.Fatalln("[ERROR]", err)
	}
	log.Printf("%v %v\n", "[INFO]", fmt.Sprintf("メッセージをファイルに書き込みました。(%v 件)", len(noticeMessages)))

	// 【処理終了】
	log.Printf("%v %v\n", "[INFO]", "【end process】")
	log.Println(repeatedStars)
}
