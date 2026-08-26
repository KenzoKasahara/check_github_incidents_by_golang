package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const (
	ENV_DISCORD_WEBHOOK_URL string = "DISCORD_WEBHOOK_URL"
	ENV_SLACK_WEBHOOK_URL   string = "SLACK_WEBHOOK_URL"

	// MAX_NOTIFY_INCIDENTS は 1 回の通知に含めるインシデントの上限 (Discord の embeds 上限に合わせる)。
	MAX_NOTIFY_INCIDENTS int = 10

	// WEBHOOK_ERROR_BODY_LIMIT はエラー時に読み取るレスポンスボディの上限バイト数。
	WEBHOOK_ERROR_BODY_LIMIT int64 = 512
)

// Notifier は通知先ごとの送信内容を表す。
type Notifier interface {
	// Name は通知先の名称を返す (ログ出力用)。
	Name() string
	// WebhookURL は送信先の Webhook URL を返す。秘匿情報のためログには出力しない。
	WebhookURL() string
	// BuildPayload は通知先の形式に合わせたリクエストボディを組み立てる。
	BuildPayload(noticeMessages []NoticeMessage) any
}

// NewNotifiers は Webhook URL が設定されている通知先だけを返す。
// URL が未設定の通知先は対象外となり、両方とも未設定であれば空スライスを返す。
func NewNotifiers() []Notifier {
	notifiers := []Notifier{}

	if webhookURL := strings.TrimSpace(os.Getenv(ENV_DISCORD_WEBHOOK_URL)); webhookURL != "" {
		notifiers = append(notifiers, DiscordNotifier{Webhook: webhookURL})
	}
	if webhookURL := strings.TrimSpace(os.Getenv(ENV_SLACK_WEBHOOK_URL)); webhookURL != "" {
		notifiers = append(notifiers, SlackNotifier{Webhook: webhookURL})
	}

	return notifiers
}

// SendNotifications はすべての通知先へ送信する。
// 一部の通知先が失敗しても残りの送信は継続し、失敗をまとめてエラーとして返す。
// dryRun が true の場合は送信せず、送信内容をログに出力する。
func SendNotifications(notifiers []Notifier, noticeMessages []NoticeMessage, dryRun bool) error {
	if len(notifiers) == 0 {
		log.Printf("%v %v\n", "[INFO]", "Webhook URL が未設定のため、通知をスキップします。")
		return nil
	}
	if len(noticeMessages) == 0 {
		log.Printf("%v %v\n", "[INFO]", "通知対象のインシデントが無いため、通知をスキップします。")
		return nil
	}

	failures := []string{}
	for _, notifier := range notifiers {
		if dryRun {
			if err := LogPayload(notifier, noticeMessages); err != nil {
				log.Printf("%v %v\n", "[ERROR]", fmt.Sprintf("%v の送信内容の出力に失敗しました: %v", notifier.Name(), err))
				failures = append(failures, notifier.Name())
			}
			continue
		}

		if err := PostWebhook(notifier.WebhookURL(), notifier.BuildPayload(noticeMessages)); err != nil {
			log.Printf("%v %v\n", "[ERROR]", fmt.Sprintf("%v への通知に失敗しました: %v", notifier.Name(), err))
			failures = append(failures, notifier.Name())
			continue
		}
		log.Printf("%v %v\n", "[INFO]", fmt.Sprintf("%v へ通知しました。(%v 件)", notifier.Name(), len(noticeMessages)))
	}

	if len(failures) > 0 {
		return fmt.Errorf("通知に失敗しました: %v", strings.Join(failures, ", "))
	}

	return nil
}

// LogPayload は送信内容をログへ出力する (dry-run 用)。
// Webhook URL は秘匿情報のため出力しない。
func LogPayload(notifier Notifier, noticeMessages []NoticeMessage) error {
	indentJsonData, err := json.MarshalIndent(notifier.BuildPayload(noticeMessages), "", "    ")
	if err != nil {
		return fmt.Errorf("ペイロードの JSON エンコードに失敗しました: %w", err)
	}

	log.Printf("%v %v\n%v\n", "[INFO]", fmt.Sprintf("[dry-run] %v へは送信しません。送信内容:", notifier.Name()), string(indentJsonData))

	return nil
}

// PostWebhook は Webhook へ JSON を POST する。
func PostWebhook(webhookURL string, payload any) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("ペイロードの JSON エンコードに失敗しました: %w", err)
	}

	req, err := http.NewRequest("POST", webhookURL, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("リクエストの作成に失敗しました: %w", RedactURL(err))
	}
	req.Header.Set("Content-Type", "application/json")

	var client *http.Client = &http.Client{Timeout: HTTP_TIMEOUT}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("リクエストの送信に失敗しました: %w", RedactURL(err))
	}
	defer resp.Body.Close()

	// Discord は 204、Slack は 200 を返す
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, WEBHOOK_ERROR_BODY_LIMIT))
		return fmt.Errorf("予期しないステータスコードが返却されました: %v (%v)", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return nil
}

// RedactURL は url.Error から URL を取り除く。
// Webhook URL は秘匿情報であり、そのままではエラーメッセージ経由でログに残るため。
func RedactURL(err error) error {
	if urlErr, ok := err.(*url.Error); ok {
		return urlErr.Err
	}

	return err
}

// LimitNoticeMessages は 1 回の通知に含める件数を上限まで絞り込む。
// Discord の embeds は 10 件までという制約があるため、両サービスで同じ上限を用いる。
func LimitNoticeMessages(noticeMessages []NoticeMessage) (limited []NoticeMessage, omitted int) {
	if len(noticeMessages) <= MAX_NOTIFY_INCIDENTS {
		return noticeMessages, 0
	}

	return noticeMessages[:MAX_NOTIFY_INCIDENTS], len(noticeMessages) - MAX_NOTIFY_INCIDENTS
}

// NotificationSummary は通知の見出しを組み立てる。
func NotificationSummary(noticeMessages []NoticeMessage, omitted int) string {
	summary := fmt.Sprintf("GitHub で %v 件のインシデントが発生中です。", len(noticeMessages)+omitted)
	if omitted > 0 {
		summary += fmt.Sprintf(" (先頭 %v 件のみ表示 / 他 %v 件)", len(noticeMessages), omitted)
	}

	return summary
}

// FormatComponents はコンポーネント名の配列を通知用の文字列に整形する。
func FormatComponents(components []string) string {
	if len(components) == 0 {
		return "-"
	}

	return strings.Join(components, ", ")
}
