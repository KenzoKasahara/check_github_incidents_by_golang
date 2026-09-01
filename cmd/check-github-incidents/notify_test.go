package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

func sampleNoticeMessages(count int) []NoticeMessage {
	noticeMessages := []NoticeMessage{}
	for i := 0; i < count; i++ {
		noticeMessages = append(noticeMessages, NoticeMessage{
			IncidentID:         fmt.Sprintf("incident-%v", i),
			IncidentImpact:     "critical",
			IncidentName:       fmt.Sprintf("Incident %v", i),
			IncidentStatus:     "investigating",
			IncidentShortLink:  "https://stspg.io/example",
			IncidentComponents: []string{"Actions"},
			IncidentCreatedAt:  "2026-08-26T10:00:00.000+09:00",
			IncidentUpdatedAt:  "2026-08-26T10:30:00.000+09:00",
		})
	}

	return noticeMessages
}

// sampleChanges は新規発生として検知したインシデントの変化を組み立てる。
func sampleChanges(count int) []IncidentChange {
	changes := []IncidentChange{}
	for _, noticeMessage := range sampleNoticeMessages(count) {
		changes = append(changes, IncidentChange{
			Type:     CHANGE_NEW,
			ID:       noticeMessage.IncidentID,
			Name:     noticeMessage.IncidentName,
			Incident: &noticeMessage,
		})
	}

	return changes
}

// sampleResolvedChange は復旧として検知したインシデントの変化を組み立てる。
// 解決後の情報を取得できなかった場合を表すため、Resolved は設定しない。
func sampleResolvedChange() IncidentChange {
	previous := NotifiedIncident{
		Name:      "Incident 0",
		Status:    "monitoring",
		UpdatedAt: "2026-08-26T10:30:00.000+09:00",
	}

	return IncidentChange{
		Type:     CHANGE_RESOLVED,
		ID:       "incident-0",
		Name:     previous.Name,
		Previous: &previous,
	}
}

func TestNewNotifiers(t *testing.T) {
	tests := []struct {
		name       string
		discordURL string
		slackURL   string
		want       []string
	}{
		{name: "両方未設定なら通知先なし", want: []string{}},
		{name: "Discord のみ", discordURL: "https://discord.example/webhook", want: []string{"Discord"}},
		{name: "Slack のみ", slackURL: "https://slack.example/webhook", want: []string{"Slack"}},
		{name: "両方設定", discordURL: "https://discord.example/webhook", slackURL: "https://slack.example/webhook", want: []string{"Discord", "Slack"}},
		{name: "空白のみの値は未設定として扱う", discordURL: "   ", want: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(ENV_DISCORD_WEBHOOK_URL, tt.discordURL)
			t.Setenv(ENV_SLACK_WEBHOOK_URL, tt.slackURL)

			names := []string{}
			for _, notifier := range NewNotifiers() {
				names = append(names, notifier.Name())
			}
			if !equalStrings(names, tt.want) {
				t.Errorf("通知先 = %v, want %v", names, tt.want)
			}
		})
	}
}

func TestDiscordBuildPayload(t *testing.T) {
	payload := DiscordNotifier{Webhook: "https://discord.example/webhook"}.BuildPayload(sampleChanges(1))

	discordPayload, ok := payload.(DiscordPayload)
	if !ok {
		t.Fatalf("型 = %T, want DiscordPayload", payload)
	}
	if len(discordPayload.Embeds) != 1 {
		t.Fatalf("embeds = %v 件, want 1", len(discordPayload.Embeds))
	}

	embed := discordPayload.Embeds[0]
	if embed.Title != "🚨 新しいインシデント: Incident 0" {
		t.Errorf("title = %v", embed.Title)
	}
	if embed.URL != "https://stspg.io/example" {
		t.Errorf("url = %v", embed.URL)
	}
	if embed.Color != 0xD32F2F {
		t.Errorf("color = %v, want %v", embed.Color, 0xD32F2F)
	}
	if !strings.Contains(discordPayload.Content, "新規 1 件") {
		t.Errorf("content = %v", discordPayload.Content)
	}
}

// 解決後の情報を取得できないときに、前回の記録から通知を組み立てられることを確認する。
func TestDiscordBuildPayloadResolvedWithoutDetail(t *testing.T) {
	payload := DiscordNotifier{}.BuildPayload([]IncidentChange{sampleResolvedChange()}).(DiscordPayload)

	if len(payload.Embeds) != 1 {
		t.Fatalf("embeds = %v 件, want 1", len(payload.Embeds))
	}

	embed := payload.Embeds[0]
	if embed.Title != "✅ 復旧しました: Incident 0" {
		t.Errorf("title = %v", embed.Title)
	}
	if embed.Color != DISCORD_RESOLVED_COLOR {
		t.Errorf("color = %v, want %v", embed.Color, DISCORD_RESOLVED_COLOR)
	}
	if len(embed.Fields) != 2 || embed.Fields[0].Value != "monitoring" {
		t.Errorf("fields = %v", embed.Fields)
	}
	if !strings.Contains(payload.Content, "復旧 1 件") {
		t.Errorf("content = %v", payload.Content)
	}
}

func TestSlackBuildPayload(t *testing.T) {
	payload := SlackNotifier{Webhook: "https://slack.example/webhook"}.BuildPayload(sampleChanges(1))

	slackPayload, ok := payload.(SlackPayload)
	if !ok {
		t.Fatalf("型 = %T, want SlackPayload", payload)
	}
	if len(slackPayload.Attachments) != 1 {
		t.Fatalf("attachments = %v 件, want 1", len(slackPayload.Attachments))
	}

	attachment := slackPayload.Attachments[0]
	if attachment.Title != ":rotating_light: 新しいインシデント: Incident 0" {
		t.Errorf("title = %v", attachment.Title)
	}
	if attachment.TitleLink != "https://stspg.io/example" {
		t.Errorf("title_link = %v", attachment.TitleLink)
	}
	if attachment.Color != "#D32F2F" {
		t.Errorf("color = %v, want #D32F2F", attachment.Color)
	}
}

// 解決後の情報を取得できないときに、前回の記録から通知を組み立てられることを確認する。
func TestSlackBuildPayloadResolvedWithoutDetail(t *testing.T) {
	payload := SlackNotifier{}.BuildPayload([]IncidentChange{sampleResolvedChange()}).(SlackPayload)

	if len(payload.Attachments) != 1 {
		t.Fatalf("attachments = %v 件, want 1", len(payload.Attachments))
	}

	attachment := payload.Attachments[0]
	if attachment.Title != ":white_check_mark: 復旧しました: Incident 0" {
		t.Errorf("title = %v", attachment.Title)
	}
	if attachment.Color != SLACK_RESOLVED_COLOR {
		t.Errorf("color = %v, want %v", attachment.Color, SLACK_RESOLVED_COLOR)
	}
	if len(attachment.Fields) != 2 || attachment.Fields[0].Value != "monitoring" {
		t.Errorf("fields = %v", attachment.Fields)
	}
}

// Discord の embeds 上限を超えないこと、超過分が見出しに示されることを確認する。
func TestBuildPayloadLimitsIncidents(t *testing.T) {
	changes := sampleChanges(MAX_NOTIFY_INCIDENTS + 3)

	discordPayload := DiscordNotifier{}.BuildPayload(changes).(DiscordPayload)
	if len(discordPayload.Embeds) != MAX_NOTIFY_INCIDENTS {
		t.Errorf("embeds = %v 件, want %v", len(discordPayload.Embeds), MAX_NOTIFY_INCIDENTS)
	}
	if !strings.Contains(discordPayload.Content, "他 3 件") {
		t.Errorf("content = %v", discordPayload.Content)
	}
	// 表示を省いた分も内訳の件数には含める
	if !strings.Contains(discordPayload.Content, fmt.Sprintf("新規 %v 件", MAX_NOTIFY_INCIDENTS+3)) {
		t.Errorf("content = %v", discordPayload.Content)
	}

	slackPayload := SlackNotifier{}.BuildPayload(changes).(SlackPayload)
	if len(slackPayload.Attachments) != MAX_NOTIFY_INCIDENTS {
		t.Errorf("attachments = %v 件, want %v", len(slackPayload.Attachments), MAX_NOTIFY_INCIDENTS)
	}
}

// 通知先ごとに絵文字の表記が使い分けられていることを確認する。
func TestChangeTitle(t *testing.T) {
	change := IncidentChange{Type: CHANGE_UPDATED, Name: "Incident 0"}

	if got := ChangeTitle(DiscordChangeLabel(change.Type), change); got != "🔄 状況が更新されました: Incident 0" {
		t.Errorf("Discord の見出し = %v", got)
	}
	if got := ChangeTitle(SlackChangeLabel(change.Type), change); got != ":arrows_counterclockwise: 状況が更新されました: Incident 0" {
		t.Errorf("Slack の見出し = %v", got)
	}
	// ラベルが無いときはインシデント名だけにする
	if got := ChangeTitle("", change); got != "Incident 0" {
		t.Errorf("見出し = %v, want Incident 0", got)
	}

	// 名称を取得できないときは、どの障害か分かるようインシデント ID で代替する
	noName := IncidentChange{Type: CHANGE_RESOLVED, ID: "abcdef123456"}
	if got := ChangeTitle("✅ 復旧しました", noName); got != "✅ 復旧しました: abcdef123456" {
		t.Errorf("見出し = %v", got)
	}
}

// 名称を取得できないインシデントでも、通知先が受け付ける中身になることを確認する。
// Discord は値が空の項目を 400 で弾き、1 つでも混ざると通知全体が失敗する。
func TestResolvedPayloadHasNoEmptyValues(t *testing.T) {
	// 旧形式の記録から読み込むと、名称もステータスも空のまま復旧を迎えることがある
	changes := []IncidentChange{
		{
			Type:     CHANGE_RESOLVED,
			ID:       "abcdef123456",
			Previous: &NotifiedIncident{},
		},
	}

	discord, ok := DiscordNotifier{}.BuildPayload(changes).(DiscordPayload)
	if !ok {
		t.Fatal("DiscordPayload が返ること")
	}
	if len(discord.Embeds) != 1 {
		t.Fatalf("embeds = %v 件, want 1", len(discord.Embeds))
	}
	// 見出しは名称の代わりにインシデント ID で埋める
	if discord.Embeds[0].Title != "✅ 復旧しました: abcdef123456" {
		t.Errorf("見出し = %v", discord.Embeds[0].Title)
	}
	for _, field := range discord.Embeds[0].Fields {
		if field.Value == "" {
			t.Errorf("Discord の項目 %v の値が空です", field.Name)
		}
	}

	slack, ok := SlackNotifier{}.BuildPayload(changes).(SlackPayload)
	if !ok {
		t.Fatal("SlackPayload が返ること")
	}
	for _, field := range slack.Attachments[0].Fields {
		if field.Value == "" {
			t.Errorf("Slack の項目 %v の値が空です", field.Title)
		}
	}
}

func TestFieldValue(t *testing.T) {
	if got := FieldValue("major"); got != "major" {
		t.Errorf("FieldValue(major) = %v", got)
	}
	if got := FieldValue("   "); got != "-" {
		t.Errorf("FieldValue(空白) = %v, want -", got)
	}
	if got := FieldValue(""); got != "-" {
		t.Errorf("FieldValue(空) = %v, want -", got)
	}
}

func TestNotificationSummary(t *testing.T) {
	changes := []IncidentChange{
		{Type: CHANGE_NEW},
		{Type: CHANGE_UPDATED},
		{Type: CHANGE_RESOLVED},
		{Type: CHANGE_RESOLVED},
	}

	got := NotificationSummary(changes, 0)
	if !strings.Contains(got, "新規 1 件 / 更新 1 件 / 復旧 2 件") {
		t.Errorf("summary = %v", got)
	}
	// 変化が無いものは内訳に出さない
	if got := NotificationSummary([]IncidentChange{{Type: CHANGE_RESOLVED}}, 0); strings.Contains(got, "新規") {
		t.Errorf("summary = %v", got)
	}
}

func TestFormatComponents(t *testing.T) {
	if got := FormatComponents(nil); got != "-" {
		t.Errorf("FormatComponents(nil) = %v, want -", got)
	}
	if got := FormatComponents([]string{"Actions", "Pages"}); got != "Actions, Pages" {
		t.Errorf("FormatComponents() = %v", got)
	}
}

func TestPostWebhook(t *testing.T) {
	var receivedContentType string
	var receivedBody DiscordPayload

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	payload := DiscordNotifier{}.BuildPayload(sampleChanges(1))
	if err := PostWebhook(server.URL, payload); err != nil {
		t.Fatalf("PostWebhook() error = %v", err)
	}
	if receivedContentType != "application/json" {
		t.Errorf("Content-Type = %v, want application/json", receivedContentType)
	}
	if len(receivedBody.Embeds) != 1 {
		t.Errorf("受信した embeds = %v 件, want 1", len(receivedBody.Embeds))
	}
}

func TestPostWebhookErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid payload"))
	}))
	defer server.Close()

	err := PostWebhook(server.URL, DiscordPayload{})
	if err == nil {
		t.Fatal("エラーが返ること")
	}
	if !strings.Contains(err.Error(), "400") || !strings.Contains(err.Error(), "invalid payload") {
		t.Errorf("error = %v", err)
	}
}

// Webhook URL がエラーメッセージ経由でログに残らないことを確認する。
func TestPostWebhookDoesNotLeakURL(t *testing.T) {
	const secretURL = "http://127.0.0.1:1/super-secret-webhook-token"

	err := PostWebhook(secretURL, DiscordPayload{})
	if err == nil {
		t.Fatal("エラーが返ること")
	}
	if strings.Contains(err.Error(), "super-secret-webhook-token") {
		t.Errorf("Webhook URL が含まれています: %v", err)
	}
}

func TestRedactURL(t *testing.T) {
	inner := fmt.Errorf("connection refused")
	urlErr := &url.Error{Op: "Post", URL: "https://example.com/secret-token", Err: inner}

	if got := RedactURL(urlErr); got != inner {
		t.Errorf("RedactURL() = %v, want %v", got, inner)
	}
	if got := RedactURL(inner); got != inner {
		t.Errorf("url.Error 以外はそのまま返すこと: %v", got)
	}
}

func TestSendNotificationsSkips(t *testing.T) {
	// 通知先が無い場合
	if err := SendNotifications([]Notifier{}, sampleChanges(1), false); err != nil {
		t.Errorf("通知先が無い場合はエラーにしないこと: %v", err)
	}
	// インシデントが 0 件の場合は送信しない (送信先が不正でもエラーにならないことで確認する)
	notifiers := []Notifier{DiscordNotifier{Webhook: "http://127.0.0.1:1/unreachable"}}
	if err := SendNotifications(notifiers, []IncidentChange{}, false); err != nil {
		t.Errorf("インシデントが 0 件の場合はエラーにしないこと: %v", err)
	}
}

// 一方が失敗しても他方の送信が継続されることを確認する。
func TestSendNotificationsContinuesOnFailure(t *testing.T) {
	received := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifiers := []Notifier{
		DiscordNotifier{Webhook: "http://127.0.0.1:1/unreachable"},
		SlackNotifier{Webhook: server.URL},
	}

	err := SendNotifications(notifiers, sampleChanges(1), false)
	if err == nil {
		t.Fatal("失敗した通知先がある場合はエラーを返すこと")
	}
	if !strings.Contains(err.Error(), "Discord") || strings.Contains(err.Error(), "Slack") {
		t.Errorf("失敗した通知先のみが含まれること: %v", err)
	}
	if received != 1 {
		t.Errorf("Slack への送信回数 = %v, want 1", received)
	}
}

// dry-run では実際に送信されないことを確認する。
func TestSendNotificationsDryRun(t *testing.T) {
	received := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifiers := []Notifier{
		DiscordNotifier{Webhook: server.URL},
		SlackNotifier{Webhook: server.URL},
	}

	if err := SendNotifications(notifiers, sampleChanges(1), true); err != nil {
		t.Fatalf("SendNotifications() error = %v", err)
	}
	if received != 0 {
		t.Errorf("送信回数 = %v, want 0", received)
	}
}

// dry-run のログ出力に Webhook URL が含まれないことを確認する。
func TestLogPayloadDoesNotLeakURL(t *testing.T) {
	const secretURL = "https://discord.example/webhooks/super-secret-token"

	var buffer bytes.Buffer
	originalFlags := log.Flags()
	log.SetOutput(&buffer)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(os.Stderr)
		log.SetFlags(originalFlags)
	})

	if err := LogPayload(DiscordNotifier{Webhook: secretURL}, sampleChanges(1)); err != nil {
		t.Fatalf("LogPayload() error = %v", err)
	}

	logged := buffer.String()
	if strings.Contains(logged, "super-secret-token") {
		t.Errorf("Webhook URL が出力されています: %v", logged)
	}
	if !strings.Contains(logged, "Incident 0") {
		t.Errorf("送信内容が出力されていること: %v", logged)
	}
}

// sampleResolvedDetail は過去のインシデント一覧から取得した解決後の情報を組み立てる。
func sampleResolvedDetail() ResolvedDetail {
	return ResolvedDetail{
		Name:       "Incident 0",
		Impact:     "major",
		ShortLink:  "https://stspg.io/example",
		Components: []string{"Actions", "Pages"},
		CreatedAt:  "2026-08-31T09:15:48.908Z",
		ResolvedAt: "2026-08-31T09:58:14.157Z",
		Body:       "This incident has been resolved.",
	}
}

// 復旧の通知に、解決後の情報 (リンク・復旧日時・コメント本文) が載ることを確認する。
func TestDiscordBuildPayloadResolvedWithDetail(t *testing.T) {
	resolved := sampleResolvedDetail()
	change := IncidentChange{Type: CHANGE_RESOLVED, ID: "incident-0", Name: resolved.Name, Resolved: &resolved}

	payload := DiscordNotifier{}.BuildPayload([]IncidentChange{change}).(DiscordPayload)

	if len(payload.Embeds) != 1 {
		t.Fatalf("embeds = %v 件, want 1", len(payload.Embeds))
	}

	embed := payload.Embeds[0]
	if embed.Title != "✅ 復旧しました: Incident 0" {
		t.Errorf("title = %v", embed.Title)
	}
	// 復旧しても shortlink は残るため、リンクを付けられる
	if embed.URL != "https://stspg.io/example" {
		t.Errorf("url = %v", embed.URL)
	}
	if embed.Description != "This incident has been resolved." {
		t.Errorf("description = %v", embed.Description)
	}
	if embed.Color != DISCORD_RESOLVED_COLOR {
		t.Errorf("color = %v, want %v", embed.Color, DISCORD_RESOLVED_COLOR)
	}

	values := map[string]string{}
	for _, field := range embed.Fields {
		values[field.Name] = field.Value
	}
	want := map[string]string{
		"影響度":     "major",
		"コンポーネント": "Actions, Pages",
		"発生日時":    "2026-08-31T09:15:48.908Z",
		"復旧日時":    "2026-08-31T09:58:14.157Z",
	}
	for name, value := range want {
		if values[name] != value {
			t.Errorf("%v = %v, want %v", name, values[name], value)
		}
	}
}

// 復旧の通知に、解決後の情報 (リンク・復旧日時・コメント本文) が載ることを確認する。
func TestSlackBuildPayloadResolvedWithDetail(t *testing.T) {
	resolved := sampleResolvedDetail()
	change := IncidentChange{Type: CHANGE_RESOLVED, ID: "incident-0", Name: resolved.Name, Resolved: &resolved}

	payload := SlackNotifier{}.BuildPayload([]IncidentChange{change}).(SlackPayload)

	if len(payload.Attachments) != 1 {
		t.Fatalf("attachments = %v 件, want 1", len(payload.Attachments))
	}

	attachment := payload.Attachments[0]
	if attachment.Title != ":white_check_mark: 復旧しました: Incident 0" {
		t.Errorf("title = %v", attachment.Title)
	}
	if attachment.TitleLink != "https://stspg.io/example" {
		t.Errorf("title_link = %v", attachment.TitleLink)
	}
	if attachment.Text != "This incident has been resolved." {
		t.Errorf("text = %v", attachment.Text)
	}
	if attachment.Color != SLACK_RESOLVED_COLOR {
		t.Errorf("color = %v, want %v", attachment.Color, SLACK_RESOLVED_COLOR)
	}

	values := map[string]string{}
	for _, field := range attachment.Fields {
		values[field.Title] = field.Value
	}
	if values["復旧日時"] != "2026-08-31T09:58:14.157Z" {
		t.Errorf("復旧日時 = %v", values["復旧日時"])
	}
	if values["コンポーネント"] != "Actions, Pages" {
		t.Errorf("コンポーネント = %v", values["コンポーネント"])
	}
}

// 本文が無いときに、通知の項目ごと省かれることを確認する (空欄が並ぶのを避けるため)。
func TestResolvedPayloadOmitsEmptyBody(t *testing.T) {
	resolved := sampleResolvedDetail()
	resolved.Body = ""
	change := IncidentChange{Type: CHANGE_RESOLVED, ID: "incident-0", Name: resolved.Name, Resolved: &resolved}

	discordJson, err := json.Marshal(DiscordNotifier{}.BuildPayload([]IncidentChange{change}).(DiscordPayload).Embeds[0])
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if strings.Contains(string(discordJson), "description") {
		t.Errorf("embed の description が出力されないこと: %v", string(discordJson))
	}

	// 見出しの text と紛れないよう、attachment だけを取り出して確かめる
	slackJson, err := json.Marshal(SlackNotifier{}.BuildPayload([]IncidentChange{change}).(SlackPayload).Attachments[0])
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if strings.Contains(string(slackJson), `"text"`) {
		t.Errorf("attachment の text が出力されないこと: %v", string(slackJson))
	}
}
