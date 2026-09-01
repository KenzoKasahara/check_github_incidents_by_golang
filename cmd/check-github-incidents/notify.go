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
	BuildPayload(changes []IncidentChange) any
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
func SendNotifications(notifiers []Notifier, changes []IncidentChange, dryRun bool) error {
	if len(notifiers) == 0 {
		log.Printf("%v %v\n", "[INFO]", "Webhook URL が未設定のため、通知をスキップします。")
		return nil
	}
	if len(changes) == 0 {
		log.Printf("%v %v\n", "[INFO]", "通知対象のインシデントが無いため、通知をスキップします。")
		return nil
	}

	failures := []string{}
	for _, notifier := range notifiers {
		if dryRun {
			if err := LogPayload(notifier, changes); err != nil {
				log.Printf("%v %v\n", "[ERROR]", fmt.Sprintf("%v の送信内容の出力に失敗しました: %v", notifier.Name(), err))
				failures = append(failures, notifier.Name())
			}
			continue
		}

		if err := PostWebhook(notifier.WebhookURL(), notifier.BuildPayload(changes)); err != nil {
			log.Printf("%v %v\n", "[ERROR]", fmt.Sprintf("%v への通知に失敗しました: %v", notifier.Name(), err))
			failures = append(failures, notifier.Name())
			continue
		}
		log.Printf("%v %v\n", "[INFO]", fmt.Sprintf("%v へ通知しました。(%v 件)", notifier.Name(), len(changes)))
	}

	if len(failures) > 0 {
		return fmt.Errorf("通知に失敗しました: %v", strings.Join(failures, ", "))
	}

	return nil
}

// LogPayload は送信内容をログへ出力する (dry-run 用)。
// Webhook URL は秘匿情報のため出力しない。
func LogPayload(notifier Notifier, changes []IncidentChange) error {
	indentJsonData, err := json.MarshalIndent(notifier.BuildPayload(changes), "", "    ")
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

// LimitChanges は 1 回の通知に含める件数を上限まで絞り込む。
// Discord の embeds は 10 件までという制約があるため、両サービスで同じ上限を用いる。
func LimitChanges(changes []IncidentChange) (limited []IncidentChange, omitted int) {
	if len(changes) <= MAX_NOTIFY_INCIDENTS {
		return changes, 0
	}

	return changes[:MAX_NOTIFY_INCIDENTS], len(changes) - MAX_NOTIFY_INCIDENTS
}

// ChangeTitle は変化の種類を表すラベルとインシデント名から見出しを組み立てる。
// 絵文字の書き方が通知先によって異なるため、ラベルは呼び出し側から渡す。
// 名称を取得できなかった場合はインシデント ID で代替し、どの障害の通知か分かるようにする。
func ChangeTitle(label string, change IncidentChange) string {
	name := strings.TrimSpace(change.Name)
	if name == "" {
		name = change.ID
	}
	if label == "" {
		return name
	}

	return label + ": " + name
}

// FieldValue は通知に載せる項目の値を整える。
// Discord は値が空の項目を受け付けず、1 つでも含まれるとリクエスト全体が 400 で失敗する。
// 記録に名称やステータスが残っていないインシデントでも通知だけは届くよう、空のときは "-" を送る。
func FieldValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}

	return value
}

// NotificationSummary は通知の見出しを組み立てる。
// 発生中の件数ではなく変化の内訳を示す (通知するのは前回から変化したものだけのため)。
// changes には上限で表示を省いた分も含めて渡すこと (内訳から漏れないようにするため)。
func NotificationSummary(changes []IncidentChange, omitted int) string {
	newCount, updatedCount, resolvedCount := CountChanges(changes)

	parts := []string{}
	if newCount > 0 {
		parts = append(parts, fmt.Sprintf("新規 %v 件", newCount))
	}
	if updatedCount > 0 {
		parts = append(parts, fmt.Sprintf("更新 %v 件", updatedCount))
	}
	if resolvedCount > 0 {
		parts = append(parts, fmt.Sprintf("復旧 %v 件", resolvedCount))
	}

	summary := "GitHub のインシデント情報に変化がありました。"
	if len(parts) > 0 {
		summary += " (" + strings.Join(parts, " / ") + ")"
	}
	if omitted > 0 {
		summary += fmt.Sprintf(" (先頭 %v 件のみ表示 / 他 %v 件)", len(changes)-omitted, omitted)
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
