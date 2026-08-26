package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNotifiedStateUnnotified(t *testing.T) {
	state := NotifiedState{Incidents: map[string]string{
		"already-notified": "2026-08-26T10:00:00Z",
		"updated":          "2026-08-26T10:00:00Z",
	}}

	noticeMessages := []NoticeMessage{
		{IncidentID: "already-notified", IncidentUpdatedAt: "2026-08-26T10:00:00Z"},
		{IncidentID: "updated", IncidentUpdatedAt: "2026-08-26T11:00:00Z"},
		{IncidentID: "new", IncidentUpdatedAt: "2026-08-26T12:00:00Z"},
	}

	got := state.Unnotified(noticeMessages)

	wantIDs := []string{"updated", "new"}
	gotIDs := []string{}
	for _, noticeMessage := range got {
		gotIDs = append(gotIDs, noticeMessage.IncidentID)
	}
	if !equalStrings(gotIDs, wantIDs) {
		t.Errorf("通知対象 = %v, want %v", gotIDs, wantIDs)
	}
}

// 初回実行 (状態が空) では全件が通知対象になることを確認する。
func TestNotifiedStateUnnotifiedFirstRun(t *testing.T) {
	state := NotifiedState{Incidents: map[string]string{}}
	noticeMessages := []NoticeMessage{{IncidentID: "a"}, {IncidentID: "b"}}

	if got := state.Unnotified(noticeMessages); len(got) != 2 {
		t.Errorf("通知対象 = %v 件, want 2", len(got))
	}
}

func TestNotifiedStateUnnotifiedReturnsEmptySlice(t *testing.T) {
	state := NotifiedState{Incidents: map[string]string{"a": ""}}

	if got := state.Unnotified([]NoticeMessage{{IncidentID: "a"}}); got == nil {
		t.Error("nil ではなく空スライスが返ること")
	}
}

// 解決済みとなったインシデントの記録が引き継がれないことを確認する。
func TestNewNotifiedStateDropsResolved(t *testing.T) {
	noticeMessages := []NoticeMessage{{IncidentID: "still-open", IncidentUpdatedAt: "2026-08-26T10:00:00Z"}}

	state := NewNotifiedState(noticeMessages)

	if len(state.Incidents) != 1 {
		t.Fatalf("記録件数 = %v, want 1", len(state.Incidents))
	}
	if state.Incidents["still-open"] != "2026-08-26T10:00:00Z" {
		t.Errorf("updated_at = %v", state.Incidents["still-open"])
	}
}

func TestSaveAndLoadNotifiedState(t *testing.T) {
	stateFilePath := filepath.Join(t.TempDir(), "notified_incidents.json")

	want := NewNotifiedState([]NoticeMessage{
		{IncidentID: "a", IncidentUpdatedAt: "2026-08-26T10:00:00Z"},
		{IncidentID: "b", IncidentUpdatedAt: "2026-08-26T11:00:00Z"},
	})
	if err := SaveNotifiedState(stateFilePath, want); err != nil {
		t.Fatalf("SaveNotifiedState() error = %v", err)
	}

	got, err := LoadNotifiedState(stateFilePath)
	if err != nil {
		t.Fatalf("LoadNotifiedState() error = %v", err)
	}
	if len(got.Incidents) != len(want.Incidents) {
		t.Fatalf("記録件数 = %v, want %v", len(got.Incidents), len(want.Incidents))
	}
	for id, updatedAt := range want.Incidents {
		if got.Incidents[id] != updatedAt {
			t.Errorf("%v = %v, want %v", id, got.Incidents[id], updatedAt)
		}
	}
}

// 初回実行時にファイルが無くてもエラーにならないことを確認する。
func TestLoadNotifiedStateMissingFile(t *testing.T) {
	state, err := LoadNotifiedState(filepath.Join(t.TempDir(), "not-exists.json"))
	if err != nil {
		t.Fatalf("LoadNotifiedState() error = %v", err)
	}
	if state.Incidents == nil {
		t.Error("Incidents が初期化されていること")
	}
	if len(state.Incidents) != 0 {
		t.Errorf("記録件数 = %v, want 0", len(state.Incidents))
	}
}

func TestLoadNotifiedStateBrokenFile(t *testing.T) {
	stateFilePath := filepath.Join(t.TempDir(), "notified_incidents.json")
	if err := os.WriteFile(stateFilePath, []byte("{broken"), 0600); err != nil {
		t.Fatalf("テスト用ファイルの作成に失敗しました: %v", err)
	}

	if _, err := LoadNotifiedState(stateFilePath); err == nil {
		t.Error("エラーが返ること")
	}
}
