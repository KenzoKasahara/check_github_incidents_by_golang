package main

import (
	"testing"
	"time"
)

// runOnce は 1 回分の実行を模す。main の流れ (取得 → 突合 → 記録) と同じ順序で呼ぶ。
func runOnce(state NotifiedState, page string, history []HistoryIncident, unresolved []UnresolvedIncident) (NotifiedState, []IncidentChange) {
	historyIncidents := HistoryIncidents{Page: IncidentPage{UpdateAt: page}, Incidents: history}
	unresolvedIncidents := UnresolvedIncidents{Incidents: unresolved}

	snapshot := BuildIncidentSnapshot(historyIncidents, unresolvedIncidents)
	changes := state.Diff(snapshot)
	checkedAt := CheckedAt(historyIncidents, state.LastCheckedAt, time.Now())

	return NewNotifiedState(state, snapshot, changes, checkedAt), changes
}

// 発生から復旧までを通しで実行し、復旧が 1 度だけ通知されることを確認する。
// 記録が空 ({"incidents": {}} だけの状態) から始めるのは、記録を作り直した直後や
// 以前の版で作られた記録が残っている場合を再現するため。
func TestIncidentLifecycleNotifiesRecovery(t *testing.T) {
	ongoing := HistoryIncident{
		ID:         "x",
		Name:       "Disruption with Actions",
		Status:     "investigating",
		Impact:     "major",
		CreatedAt:  "2026-09-04T20:50:00Z",
		UpdatedAt:  "2026-09-04T20:55:00Z",
		Components: []AffectedComponent{{Name: "Actions"}},
	}
	resolved := ongoing
	resolved.Status = "resolved"
	resolved.UpdatedAt = "2026-09-04T21:05:00Z"
	resolved.ResolvedAt = "2026-09-04T21:05:00Z"

	unresolved := UnresolvedIncident{
		ID:        ongoing.ID,
		Name:      ongoing.Name,
		Status:    ongoing.Status,
		Impact:    ongoing.Impact,
		CreatedAt: ongoing.CreatedAt,
		UpdatedAt: ongoing.UpdatedAt,
	}

	// 以前の版が残した記録。目印 (last_checked_at) を持たない
	state := NotifiedState{Incidents: map[string]NotifiedIncident{}}

	state, changes := runOnce(state, "2026-09-04T20:00:00Z", []HistoryIncident{}, nil)
	if len(changes) != 0 {
		t.Fatalf("平常時の変化 = %v, want 0 件", changeIDs(changes))
	}

	state, changes = runOnce(state, "2026-09-04T21:00:00Z", []HistoryIncident{ongoing}, []UnresolvedIncident{unresolved})
	if !equalStrings(changeIDs(changes), []string{"new:x"}) {
		t.Fatalf("発生時の変化 = %v, want 新規 1 件", changeIDs(changes))
	}

	state, changes = runOnce(state, "2026-09-04T21:10:00Z", []HistoryIncident{resolved}, nil)
	if !equalStrings(changeIDs(changes), []string{"resolved:x"}) {
		t.Fatalf("復旧時の変化 = %v, want 復旧 1 件", changeIDs(changes))
	}
	if changes[0].Resolved == nil || !equalStrings(changes[0].Resolved.Components, []string{"Actions"}) {
		t.Errorf("復旧の通知内容 = %+v", changes[0].Resolved)
	}

	// 解決済みのインシデントは過去の一覧に残り続けるため、次回以降に再通知しないこと
	for i := range 3 {
		state, changes = runOnce(state, "2026-09-04T21:20:00Z", []HistoryIncident{resolved}, nil)
		if len(changes) != 0 {
			t.Fatalf("%v 回目の変化 = %v, want 0 件", i+1, changeIDs(changes))
		}
	}
}

// 未解決一覧への反映が遅れて一時的にインシデントが消えても、
// 誤った復旧通知を出さず、本当に解決したときに通知することを確認する。
func TestIncidentLifecycleSurvivesMissingUnresolved(t *testing.T) {
	ongoing := HistoryIncident{
		ID:        "y",
		Name:      "Degraded performance",
		Status:    "monitoring",
		CreatedAt: "2026-09-04T20:50:00Z",
		UpdatedAt: "2026-09-04T20:55:00Z",
	}
	unresolved := UnresolvedIncident{
		ID: ongoing.ID, Name: ongoing.Name, Status: ongoing.Status,
		CreatedAt: ongoing.CreatedAt, UpdatedAt: ongoing.UpdatedAt,
	}
	resolved := ongoing
	resolved.Status = "resolved"
	resolved.ResolvedAt = "2026-09-04T21:30:00Z"

	state := NotifiedState{Incidents: map[string]NotifiedIncident{}}

	state, _ = runOnce(state, "2026-09-04T21:00:00Z", []HistoryIncident{ongoing}, []UnresolvedIncident{unresolved})

	// 未解決一覧からだけ消え、過去の一覧では未解決のまま残っている状態
	state, changes := runOnce(state, "2026-09-04T21:10:00Z", []HistoryIncident{ongoing}, nil)
	if len(changes) != 0 {
		t.Fatalf("反映待ちの変化 = %v, want 0 件", changeIDs(changes))
	}

	state, changes = runOnce(state, "2026-09-04T21:20:00Z", []HistoryIncident{ongoing}, []UnresolvedIncident{unresolved})
	if len(changes) != 0 {
		t.Fatalf("復帰後の変化 = %v, want 0 件", changeIDs(changes))
	}

	_, changes = runOnce(state, "2026-09-04T21:40:00Z", []HistoryIncident{resolved}, nil)
	if !equalStrings(changeIDs(changes), []string{"resolved:y"}) {
		t.Fatalf("復旧時の変化 = %v, want 復旧 1 件", changeIDs(changes))
	}
}
