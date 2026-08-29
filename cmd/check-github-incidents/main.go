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
	options := ParseCommandLineOptions()

	// ログフォルダ作成
	if err := CreateFolder(LOG_FOLDER_PATH); err != nil {
		log.Fatalln("[ERROR]", err)
	}

	// ログファイル名を日付で作成
	now := time.Now()
	LoggingSettings(LogFileName(LOG_FOLDER_PATH, now))

	// 【処理開始】
	log.Println(repeatedStars)
	log.Printf("%v %v\n", "[INFO]", "【start process】")

	// .env の読み込み (.env の値がシェルの環境変数より優先される)
	if err := LoadDotEnv(DOTENV_FILE_PATH); err != nil {
		log.Fatalln("[ERROR]", err)
	}

	if options.DryRun {
		log.Printf("%v %v\n", "[INFO]", "dry-run: 通知は送信せず、送信内容のログ出力のみを行います。")
	}

	// 保持期間を過ぎたログを削除
	retentionDays := LogRetentionDays()
	deleted, err := CleanupOldLogs(LOG_FOLDER_PATH, retentionDays, now)
	if err != nil {
		log.Fatalln("[ERROR]", err)
	}
	if deleted > 0 {
		log.Printf("%v %v\n", "[INFO]", fmt.Sprintf("保持期間 %v 日を過ぎたログを削除しました。(%v 件)", retentionDays, deleted))
	}

	// テスト通知 (-test-notify) はここで完結させ、インシデントの取得へは進まない
	if options.TestNotify {
		notifyErr := RunTestNotify(now, options.DryRun)

		log.Printf("%v %v\n", "[INFO]", "【end process】")
		log.Println(repeatedStars)

		if notifyErr != nil {
			log.Fatalln("[ERROR]", notifyErr)
		}
		return
	}

	// データの取得元を判定 (コマンドライン引数 > 環境変数 > 既定値)
	useLocal := UseLocalSample(options.LocalSample)
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

	// 通知メッセージをファイルに書き込む (通知の有無にかかわらず全件を出力する)
	if err := WriteNoticeMessages(NOTICE_FILE_PATH, noticeMessages); err != nil {
		log.Fatalln("[ERROR]", err)
	}
	log.Printf("%v %v\n", "[INFO]", fmt.Sprintf("メッセージをファイルに書き込みました。(%v 件)", len(noticeMessages)))

	// 前回通知した内容と突合し、新規・更新・復旧したインシデントだけに絞り込む
	notifiedState, err := LoadNotifiedState(NOTIFIED_STATE_FILE_PATH)
	if err != nil {
		log.Fatalln("[ERROR]", err)
	}
	changes := notifiedState.Diff(noticeMessages)
	if len(changes) == 0 {
		log.Printf("%v %v\n", "[INFO]", "前回から変化がありません。")
	} else {
		newCount, updatedCount, resolvedCount := CountChanges(changes)
		log.Printf("%v %v\n", "[INFO]", fmt.Sprintf("前回からの変化を検知しました。(新規 %v 件 / 更新 %v 件 / 復旧 %v 件)", newCount, updatedCount, resolvedCount))
	}

	// Discord / Slack へ通知
	notifiers := NewNotifiers()
	notifyErr := SendNotifications(notifiers, changes, options.DryRun)

	// 通知済み状態を更新する。
	// 送信に失敗した場合は次回に再通知させるため更新しない。
	// dry-run と通知先未設定のときも、実際には送信していないため更新しない。
	if notifyErr == nil && !options.DryRun && len(notifiers) > 0 {
		if err := SaveNotifiedState(NOTIFIED_STATE_FILE_PATH, NewNotifiedState(noticeMessages)); err != nil {
			log.Fatalln("[ERROR]", err)
		}
	}

	// 【処理終了】
	log.Printf("%v %v\n", "[INFO]", "【end process】")
	log.Println(repeatedStars)

	// 通知の失敗は個別にログへ出力済みのため、終了コードのみをここで確定させる
	if notifyErr != nil {
		log.Fatalln("[ERROR]", notifyErr)
	}
}
