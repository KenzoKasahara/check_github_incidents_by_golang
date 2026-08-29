package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSampleIncidentChanges(t *testing.T) {
	now := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)

	changes := SampleIncidentChanges(now)

	want := []ChangeType{CHANGE_NEW, CHANGE_UPDATED, CHANGE_RESOLVED}
	if len(changes) != len(want) {
		t.Fatalf("変化 = %v 件, want %v", len(changes), len(want))
	}
	for i, changeType := range want {
		if changes[i].Type != changeType {
			t.Errorf("%v 件目の種類 = %v, want %v", i+1, changes[i].Type, changeType)
		}
		// 本物のインシデントと取り違えられないよう、名称で判別できること
		if !strings.HasPrefix(changes[i].Name, TEST_NOTIFY_NAME_PREFIX) {
			t.Errorf("%v 件目の名称 = %v", i+1, changes[i].Name)
		}
	}

	// 復旧は未解決一覧から消えた状態を表す
	resolved := changes[2]
	if resolved.Incident != nil {
		t.Error("復旧の Incident は nil であること")
	}
	if resolved.Previous == nil {
		t.Fatal("復旧の Previous が設定されていること")
	}
}

// 通知先が未設定でもエラーにせず、警告だけで終わることを確認する。
func TestRunTestNotifyWithoutWebhook(t *testing.T) {
	t.Setenv(ENV_DISCORD_WEBHOOK_URL, "")
	t.Setenv(ENV_SLACK_WEBHOOK_URL, "")

	if err := RunTestNotify(time.Now(), false); err != nil {
		t.Errorf("RunTestNotify() error = %v", err)
	}
}

// 通知先が設定されていれば、新規・更新・復旧の 3 件が 1 回の送信で届くことを確認する。
func TestRunTestNotifySends(t *testing.T) {
	var received DiscordPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	t.Setenv(ENV_DISCORD_WEBHOOK_URL, server.URL)
	t.Setenv(ENV_SLACK_WEBHOOK_URL, "")

	if err := RunTestNotify(time.Now(), false); err != nil {
		t.Fatalf("RunTestNotify() error = %v", err)
	}
	if len(received.Embeds) != 3 {
		t.Errorf("embeds = %v 件, want 3", len(received.Embeds))
	}
	if !strings.Contains(received.Content, "新規 1 件 / 更新 1 件 / 復旧 1 件") {
		t.Errorf("content = %v", received.Content)
	}
}
