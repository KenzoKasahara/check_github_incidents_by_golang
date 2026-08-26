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
	payload := DiscordNotifier{Webhook: "https://discord.example/webhook"}.BuildPayload(sampleNoticeMessages(1))

	discordPayload, ok := payload.(DiscordPayload)
	if !ok {
		t.Fatalf("型 = %T, want DiscordPayload", payload)
	}
	if len(discordPayload.Embeds) != 1 {
		t.Fatalf("embeds = %v 件, want 1", len(discordPayload.Embeds))
	}

	embed := discordPayload.Embeds[0]
	if embed.Title != "Incident 0" {
		t.Errorf("title = %v, want Incident 0", embed.Title)
	}
	if embed.URL != "https://stspg.io/example" {
		t.Errorf("url = %v", embed.URL)
	}
	if embed.Color != 0xD32F2F {
		t.Errorf("color = %v, want %v", embed.Color, 0xD32F2F)
	}
	if !strings.Contains(discordPayload.Content, "1 件") {
		t.Errorf("content = %v", discordPayload.Content)
	}
}

func TestSlackBuildPayload(t *testing.T) {
	payload := SlackNotifier{Webhook: "https://slack.example/webhook"}.BuildPayload(sampleNoticeMessages(1))

	slackPayload, ok := payload.(SlackPayload)
	if !ok {
		t.Fatalf("型 = %T, want SlackPayload", payload)
	}
	if len(slackPayload.Attachments) != 1 {
		t.Fatalf("attachments = %v 件, want 1", len(slackPayload.Attachments))
	}

	attachment := slackPayload.Attachments[0]
	if attachment.Title != "Incident 0" {
		t.Errorf("title = %v, want Incident 0", attachment.Title)
	}
	if attachment.TitleLink != "https://stspg.io/example" {
		t.Errorf("title_link = %v", attachment.TitleLink)
	}
	if attachment.Color != "#D32F2F" {
		t.Errorf("color = %v, want #D32F2F", attachment.Color)
	}
}

// Discord の embeds 上限を超えないこと、超過分が見出しに示されることを確認する。
func TestBuildPayloadLimitsIncidents(t *testing.T) {
	noticeMessages := sampleNoticeMessages(MAX_NOTIFY_INCIDENTS + 3)

	discordPayload := DiscordNotifier{}.BuildPayload(noticeMessages).(DiscordPayload)
	if len(discordPayload.Embeds) != MAX_NOTIFY_INCIDENTS {
		t.Errorf("embeds = %v 件, want %v", len(discordPayload.Embeds), MAX_NOTIFY_INCIDENTS)
	}
	if !strings.Contains(discordPayload.Content, "他 3 件") {
		t.Errorf("content = %v", discordPayload.Content)
	}

	slackPayload := SlackNotifier{}.BuildPayload(noticeMessages).(SlackPayload)
	if len(slackPayload.Attachments) != MAX_NOTIFY_INCIDENTS {
		t.Errorf("attachments = %v 件, want %v", len(slackPayload.Attachments), MAX_NOTIFY_INCIDENTS)
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

	payload := DiscordNotifier{}.BuildPayload(sampleNoticeMessages(1))
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
	if err := SendNotifications([]Notifier{}, sampleNoticeMessages(1), false); err != nil {
		t.Errorf("通知先が無い場合はエラーにしないこと: %v", err)
	}
	// インシデントが 0 件の場合は送信しない (送信先が不正でもエラーにならないことで確認する)
	notifiers := []Notifier{DiscordNotifier{Webhook: "http://127.0.0.1:1/unreachable"}}
	if err := SendNotifications(notifiers, []NoticeMessage{}, false); err != nil {
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

	err := SendNotifications(notifiers, sampleNoticeMessages(1), false)
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

	if err := SendNotifications(notifiers, sampleNoticeMessages(1), true); err != nil {
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

	if err := LogPayload(DiscordNotifier{Webhook: secretURL}, sampleNoticeMessages(1)); err != nil {
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
