package main

import (
	"os"
	"path/filepath"
	"testing"
)

// changeIDs は検証しやすいよう「種類:ID」の並びへ変換する。
func changeIDs(changes []IncidentChange) []string {
	ids := []string{}
	for _, change := range changes {
		ids = append(ids, string(change.Type)+":"+change.ID)
	}

	return ids
}

func TestNotifiedStateDiff(t *testing.T) {
	state := NotifiedState{Incidents: map[string]NotifiedIncident{
		"already-notified": {Name: "変化なし", Status: "investigating", UpdatedAt: "2026-08-26T10:00:00Z"},
		"updated":          {Name: "更新あり", Status: "investigating", UpdatedAt: "2026-08-26T10:00:00Z"},
		"resolved":         {Name: "復旧済み", Status: "monitoring", UpdatedAt: "2026-08-26T09:00:00Z"},
	}}

	noticeMessages := []NoticeMessage{
		{IncidentID: "already-notified", IncidentUpdatedAt: "2026-08-26T10:00:00Z"},
		{IncidentID: "updated", IncidentUpdatedAt: "2026-08-26T11:00:00Z"},
		{IncidentID: "new", IncidentUpdatedAt: "2026-08-26T12:00:00Z"},
	}

	got := state.Diff(noticeMessages)

	want := []string{"updated:updated", "new:new", "resolved:resolved"}
	if !equalStrings(changeIDs(got), want) {
		t.Errorf("変化 = %v, want %v", changeIDs(got), want)
	}
}

// 復旧したインシデントの名称とステータスが前回の記録から引き継がれることを確認する。
func TestNotifiedStateDiffResolved(t *testing.T) {
	state := NotifiedState{Incidents: map[string]NotifiedIncident{
		"resolved": {Name: "Unplanned Database Outage", Status: "monitoring", UpdatedAt: "2026-08-26T09:00:00Z"},
	}}

	got := state.Diff([]NoticeMessage{})

	if len(got) != 1 {
		t.Fatalf("変化 = %v 件, want 1", len(got))
	}
	change := got[0]
	if change.Type != CHANGE_RESOLVED {
		t.Errorf("種類 = %v, want %v", change.Type, CHANGE_RESOLVED)
	}
	if change.Name != "Unplanned Database Outage" {
		t.Errorf("名称 = %v", change.Name)
	}
	if change.Incident != nil {
		t.Error("未解決一覧に無いため Incident は nil であること")
	}
	if change.Previous == nil || change.Previous.Status != "monitoring" {
		t.Errorf("前回の記録 = %v", change.Previous)
	}
}

// 復旧の並びが実行ごとに変わらないことを確認する (map の反復順は不定なため)。
func TestNotifiedStateDiffResolvedIsSorted(t *testing.T) {
	state := NotifiedState{Incidents: map[string]NotifiedIncident{
		"c": {Name: "C"}, "a": {Name: "A"}, "b": {Name: "B"},
	}}

	want := []string{"resolved:a", "resolved:b", "resolved:c"}
	for i := 0; i < 5; i++ {
		if got := changeIDs(state.Diff([]NoticeMessage{})); !equalStrings(got, want) {
			t.Fatalf("変化 = %v, want %v", got, want)
		}
	}
}

// 初回実行 (状態が空) では全件が新規として通知対象になることを確認する。
func TestNotifiedStateDiffFirstRun(t *testing.T) {
	state := NotifiedState{Incidents: map[string]NotifiedIncident{}}
	noticeMessages := []NoticeMessage{{IncidentID: "a"}, {IncidentID: "b"}}

	got := state.Diff(noticeMessages)

	if !equalStrings(changeIDs(got), []string{"new:a", "new:b"}) {
		t.Errorf("変化 = %v", changeIDs(got))
	}
}

func TestNotifiedStateDiffReturnsEmptySlice(t *testing.T) {
	state := NotifiedState{Incidents: map[string]NotifiedIncident{"a": {}}}

	if got := state.Diff([]NoticeMessage{{IncidentID: "a"}}); got == nil {
		t.Error("nil ではなく空スライスが返ること")
	}
}

func TestCountChanges(t *testing.T) {
	changes := []IncidentChange{
		{Type: CHANGE_NEW}, {Type: CHANGE_RESOLVED}, {Type: CHANGE_NEW}, {Type: CHANGE_UPDATED},
	}

	newCount, updatedCount, resolvedCount := CountChanges(changes)

	if newCount != 2 || updatedCount != 1 || resolvedCount != 1 {
		t.Errorf("件数 = 新規 %v / 更新 %v / 復旧 %v, want 2 / 1 / 1", newCount, updatedCount, resolvedCount)
	}
}

// 解決済みとなったインシデントの記録が引き継がれないことを確認する。
func TestNewNotifiedStateDropsResolved(t *testing.T) {
	noticeMessages := []NoticeMessage{
		{IncidentID: "still-open", IncidentName: "継続中", IncidentStatus: "identified", IncidentUpdatedAt: "2026-08-26T10:00:00Z"},
	}

	state := NewNotifiedState(noticeMessages)

	if len(state.Incidents) != 1 {
		t.Fatalf("記録件数 = %v, want 1", len(state.Incidents))
	}
	got := state.Incidents["still-open"]
	if got.UpdatedAt != "2026-08-26T10:00:00Z" {
		t.Errorf("updated_at = %v", got.UpdatedAt)
	}
	// 復旧の通知に使うため、名称とステータスも記録する
	if got.Name != "継続中" || got.Status != "identified" {
		t.Errorf("記録 = %v", got)
	}
}

func TestSaveAndLoadNotifiedState(t *testing.T) {
	stateFilePath := filepath.Join(t.TempDir(), "notified_incidents.json")

	want := NewNotifiedState([]NoticeMessage{
		{IncidentID: "a", IncidentName: "A", IncidentStatus: "investigating", IncidentUpdatedAt: "2026-08-26T10:00:00Z"},
		{IncidentID: "b", IncidentName: "B", IncidentStatus: "monitoring", IncidentUpdatedAt: "2026-08-26T11:00:00Z"},
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
	for id, incident := range want.Incidents {
		if got.Incidents[id] != incident {
			t.Errorf("%v = %v, want %v", id, got.Incidents[id], incident)
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

// 復旧通知に対応する前の形式 (値が updated_at の文字列だけ) も読めることを確認する。
func TestLoadNotifiedStateLegacyFormat(t *testing.T) {
	stateFilePath := filepath.Join(t.TempDir(), "notified_incidents.json")
	legacy := `{"incidents": {"cp306tmzcl0y": "2014-05-14T14:35:21.711-06:00"}}`
	if err := os.WriteFile(stateFilePath, []byte(legacy), 0600); err != nil {
		t.Fatalf("テスト用ファイルの作成に失敗しました: %v", err)
	}

	state, err := LoadNotifiedState(stateFilePath)
	if err != nil {
		t.Fatalf("LoadNotifiedState() error = %v", err)
	}
	if got := state.Incidents["cp306tmzcl0y"].UpdatedAt; got != "2014-05-14T14:35:21.711-06:00" {
		t.Errorf("updated_at = %v", got)
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
