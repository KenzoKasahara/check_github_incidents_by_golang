package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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

	got := state.Diff(noticeMessages, nil)

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

	got := state.Diff([]NoticeMessage{}, nil)

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
	// 過去 50 件から漏れて解決後の情報を取得できない場合を表す
	if change.Resolved != nil {
		t.Error("解決後の情報が無いため Resolved は nil であること")
	}
}

// 復旧の並びが実行ごとに変わらないことを確認する (map の反復順は不定なため)。
func TestNotifiedStateDiffResolvedIsSorted(t *testing.T) {
	state := NotifiedState{Incidents: map[string]NotifiedIncident{
		"c": {Name: "C"}, "a": {Name: "A"}, "b": {Name: "B"},
	}}

	want := []string{"resolved:a", "resolved:b", "resolved:c"}
	for i := 0; i < 5; i++ {
		if got := changeIDs(state.Diff([]NoticeMessage{}, nil)); !equalStrings(got, want) {
			t.Fatalf("変化 = %v, want %v", got, want)
		}
	}
}

// 初回実行 (状態が空) では全件が新規として通知対象になることを確認する。
func TestNotifiedStateDiffFirstRun(t *testing.T) {
	state := NotifiedState{Incidents: map[string]NotifiedIncident{}}
	noticeMessages := []NoticeMessage{{IncidentID: "a"}, {IncidentID: "b"}}

	got := state.Diff(noticeMessages, nil)

	if !equalStrings(changeIDs(got), []string{"new:a", "new:b"}) {
		t.Errorf("変化 = %v", changeIDs(got))
	}
}

func TestNotifiedStateDiffReturnsEmptySlice(t *testing.T) {
	state := NotifiedState{Incidents: map[string]NotifiedIncident{"a": {}}}

	if got := state.Diff([]NoticeMessage{{IncidentID: "a"}}, nil); got == nil {
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

	state := NewNotifiedState(noticeMessages, "2026-08-26T12:00:00Z")

	if state.LastCheckedAt != "2026-08-26T12:00:00Z" {
		t.Errorf("last_checked_at = %v", state.LastCheckedAt)
	}
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
	}, "2026-08-26T12:00:00Z")
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
	if got.LastCheckedAt != want.LastCheckedAt {
		t.Errorf("last_checked_at = %v, want %v", got.LastCheckedAt, want.LastCheckedAt)
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

// 未解決の状態を観測しないまま解決したインシデントも復旧として通知することを確認する。
// 実行間隔より短い障害や、停止していた間に始まって終わった障害がこれにあたる。
func TestNotifiedStateDiffResolvedWithoutRecord(t *testing.T) {
	state := NotifiedState{
		Incidents:     map[string]NotifiedIncident{},
		LastCheckedAt: "2026-08-31T09:00:00Z",
	}
	resolvedDetails := map[string]ResolvedDetail{
		"unseen": {Name: "見逃した障害", ResolvedAt: "2026-08-31T09:58:14.157Z", Impact: "major"},
	}

	got := state.Diff([]NoticeMessage{}, resolvedDetails)

	if len(got) != 1 {
		t.Fatalf("変化 = %v 件, want 1", len(got))
	}
	change := got[0]
	if change.Type != CHANGE_RESOLVED || change.ID != "unseen" {
		t.Errorf("変化 = %v:%v", change.Type, change.ID)
	}
	if change.Name != "見逃した障害" {
		t.Errorf("名称 = %v", change.Name)
	}
	// 記録が無いため、通知の中身は解決後の情報だけで組み立てる
	if change.Previous != nil {
		t.Error("記録が無いため Previous は nil であること")
	}
	if change.Resolved == nil || change.Resolved.Impact != "major" {
		t.Errorf("解決後の情報 = %v", change.Resolved)
	}
}

// 前回の実行より前に解決したインシデントを通知しないことを確認する。
// 過去 50 件には数日前の分も含まれるため、これが無いと毎回通知してしまう。
func TestNotifiedStateDiffIgnoresOldResolved(t *testing.T) {
	state := NotifiedState{
		Incidents:     map[string]NotifiedIncident{},
		LastCheckedAt: "2026-08-31T10:00:00Z",
	}
	resolvedDetails := map[string]ResolvedDetail{
		"old":      {Name: "前回より前", ResolvedAt: "2026-08-31T09:58:14.157Z"},
		"same":     {Name: "前回と同時刻", ResolvedAt: "2026-08-31T10:00:00Z"},
		"unparsed": {Name: "日時が読めない", ResolvedAt: ""},
	}

	if got := state.Diff([]NoticeMessage{}, resolvedDetails); len(got) != 0 {
		t.Errorf("変化 = %v, want 0 件", changeIDs(got))
	}
}

// 初回実行では過去の復旧をまとめて通知しないことを確認する。
func TestNotifiedStateDiffFirstRunSkipsResolved(t *testing.T) {
	state := NotifiedState{Incidents: map[string]NotifiedIncident{}}
	resolvedDetails := map[string]ResolvedDetail{
		"old": {Name: "過去の障害", ResolvedAt: "2026-08-31T09:58:14.157Z"},
	}

	if got := state.Diff([]NoticeMessage{}, resolvedDetails); len(got) != 0 {
		t.Errorf("変化 = %v, want 0 件", changeIDs(got))
	}
}

// 記録にあるインシデントの復旧では、解決後の情報が通知に使われることを確認する。
func TestNotifiedStateDiffResolvedUsesDetail(t *testing.T) {
	state := NotifiedState{
		Incidents:     map[string]NotifiedIncident{"resolved": {Name: "古い名称", Status: "monitoring"}},
		LastCheckedAt: "2026-08-31T09:00:00Z",
	}
	resolvedDetails := map[string]ResolvedDetail{
		"resolved": {Name: "新しい名称", ShortLink: "https://stspg.io/abc", ResolvedAt: "2026-08-31T09:58:14.157Z"},
	}

	got := state.Diff([]NoticeMessage{}, resolvedDetails)

	if len(got) != 1 {
		t.Fatalf("変化 = %v 件, want 1", len(got))
	}
	// 記録は通知した時点のもので名称が変わっていることがあるため、解決後の情報を優先する
	if got[0].Name != "新しい名称" {
		t.Errorf("名称 = %v", got[0].Name)
	}
	if got[0].Resolved == nil || got[0].Resolved.ShortLink != "https://stspg.io/abc" {
		t.Errorf("解決後の情報 = %v", got[0].Resolved)
	}
	if got[0].Previous == nil {
		t.Error("記録があるため Previous も設定されること")
	}
}

// 未解決のインシデントは、過去のインシデント一覧に解決済みで載っていても復旧としないことを確認する。
func TestNotifiedStateDiffKeepsUnresolvedOutOfResolved(t *testing.T) {
	state := NotifiedState{
		Incidents:     map[string]NotifiedIncident{"a": {Name: "継続中", UpdatedAt: "2026-08-31T09:00:00Z"}},
		LastCheckedAt: "2026-08-31T09:00:00Z",
	}
	// 2 つのエンドポイントを別々に取得するため、取得の合間に解決すると食い違うことがある
	resolvedDetails := map[string]ResolvedDetail{
		"a": {Name: "継続中", ResolvedAt: "2026-08-31T09:58:14.157Z"},
	}
	noticeMessages := []NoticeMessage{{IncidentID: "a", IncidentUpdatedAt: "2026-08-31T09:00:00Z"}}

	if got := state.Diff(noticeMessages, resolvedDetails); len(got) != 0 {
		t.Errorf("変化 = %v, want 0 件", changeIDs(got))
	}
}

func TestCheckedAt(t *testing.T) {
	now := time.Date(2026, 8, 31, 23, 0, 0, 0, time.UTC)

	tests := []struct {
		name             string
		historyIncidents HistoryIncidents
		want             string
	}{
		{
			name: "ページの更新日時を使う",
			historyIncidents: HistoryIncidents{
				Page: IncidentPage{UpdateAt: "2026-08-31T12:39:04.252Z"},
			},
			want: "2026-08-31T12:39:04.252Z",
		},
		{
			// ページの更新日時が復旧日時に追いついていないと、同じ復旧を次回も通知してしまう
			name: "ページの更新日時より新しい復旧日時があればそちらを使う",
			historyIncidents: HistoryIncidents{
				Page: IncidentPage{UpdateAt: "2026-08-31T09:00:00Z"},
				Incidents: []HistoryIncident{
					{Status: "resolved", ResolvedAt: "2026-08-31T09:58:14.157Z"},
					{Status: "resolved", ResolvedAt: "2026-08-30T10:00:00Z"},
					{Status: "investigating", ResolvedAt: ""},
				},
			},
			want: "2026-08-31T09:58:14.157Z",
		},
		{
			name:             "GitHub 側の時刻を読めなければ現在時刻を使う",
			historyIncidents: HistoryIncidents{Page: IncidentPage{UpdateAt: "unknown"}},
			want:             "2026-08-31T23:00:00Z",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := CheckedAt(test.historyIncidents, now)

			// 保存する形式は UTC へ揃えているため、文字列ではなく時刻として比べる
			gotTime, err := time.Parse(time.RFC3339, got)
			if err != nil {
				t.Fatalf("CheckedAt() = %v, 解析できません: %v", got, err)
			}
			wantTime, err := time.Parse(time.RFC3339, test.want)
			if err != nil {
				t.Fatalf("want が不正です: %v", err)
			}
			if !gotTime.Equal(wantTime) {
				t.Errorf("CheckedAt() = %v, want %v", got, test.want)
			}
		})
	}
}
