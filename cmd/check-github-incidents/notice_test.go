package main

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestBuildNoticeMessages(t *testing.T) {
	historyIncidents := HistoryIncidents{
		Incidents: []HistoryIncident{
			{
				ID:                 "resolved-001",
				AffectedComponents: []AffectedComponent{{Name: "Webhooks"}},
			},
			{
				ID:                 "unresolved-002",
				AffectedComponents: []AffectedComponent{{Name: "Actions"}, {Name: "Pages"}},
			},
		},
	}
	unresolvedIncidents := UnresolvedIncidents{
		Incidents: []UnresolvedIncident{
			{
				ID:        "unresolved-002",
				Impact:    "major",
				Name:      "Incident with a \" quote",
				Status:    "investigating",
				CreatedAt: "2026-08-26T10:00:00.000+09:00",
				UpdatedAt: "2026-08-26T10:30:00.000+09:00",
			},
		},
	}

	got := BuildNoticeMessages(historyIncidents, unresolvedIncidents)

	want := []NoticeMessage{
		{
			IncidentID:         "unresolved-002",
			IncidentImpact:     "major",
			IncidentName:       "Incident with a \" quote",
			IncidentStatus:     "investigating",
			IncidentComponents: []string{"Actions", "Pages"},
			IncidentCreatedAt:  "2026-08-26T10:00:00.000+09:00",
			IncidentUpdatedAt:  "2026-08-26T10:30:00.000+09:00",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("BuildNoticeMessages() = %+v, want %+v", got, want)
	}
}

// 突合したインシデント自身のコンポーネントを参照していることを確認する
// (先頭要素固定で参照していた不具合の回帰テスト)。
func TestBuildNoticeMessagesUsesMatchedIncidentComponents(t *testing.T) {
	historyIncidents := HistoryIncidents{
		Incidents: []HistoryIncident{
			{ID: "first", AffectedComponents: []AffectedComponent{{Name: "NotThisOne"}}},
			{ID: "match", AffectedComponents: []AffectedComponent{{Name: "Codespaces"}}},
		},
	}
	unresolvedIncidents := UnresolvedIncidents{
		Incidents: []UnresolvedIncident{{ID: "match"}},
	}

	got := BuildNoticeMessages(historyIncidents, unresolvedIncidents)

	if len(got) != 1 {
		t.Fatalf("件数 = %v, want 1", len(got))
	}
	if want := []string{"Codespaces"}; !reflect.DeepEqual(got[0].IncidentComponents, want) {
		t.Errorf("components = %v, want %v", got[0].IncidentComponents, want)
	}
}

// 過去のインシデント一覧に無くても、未解決一覧にあれば発生中として扱うことを確認する。
// 直近 50 件しか返らない過去の一覧に載るのを待つと、発生を取りこぼすうえに、
// 記録済みのインシデントが一時的に消えたように見えて誤った復旧通知が飛ぶ。
func TestBuildNoticeMessagesWithoutHistory(t *testing.T) {
	historyIncidents := HistoryIncidents{
		Incidents: []HistoryIncident{{ID: "resolved-001"}},
	}
	unresolvedIncidents := UnresolvedIncidents{
		Incidents: []UnresolvedIncident{{ID: "unresolved-002", Name: "進行中", Status: "investigating"}},
	}

	got := BuildNoticeMessages(historyIncidents, unresolvedIncidents)

	if len(got) != 1 {
		t.Fatalf("件数 = %v, want 1", len(got))
	}
	if got[0].IncidentID != "unresolved-002" || got[0].IncidentName != "進行中" {
		t.Errorf("通知メッセージ = %+v", got[0])
	}
	// コンポーネントは過去の一覧からしか取れないため、空配列となる
	if want := []string{}; !reflect.DeepEqual(got[0].IncidentComponents, want) {
		t.Errorf("components = %v, want %v", got[0].IncidentComponents, want)
	}
}

// 未解決が 0 件のときは、過去のインシデントがあっても発生中とはしない。
func TestBuildNoticeMessagesWithoutUnresolved(t *testing.T) {
	historyIncidents := HistoryIncidents{
		Incidents: []HistoryIncident{{ID: "resolved-001", Status: "resolved"}},
	}

	got := BuildNoticeMessages(historyIncidents, UnresolvedIncidents{})

	if got == nil {
		t.Fatal("nil ではなく空スライスが返ること")
	}
	if len(got) != 0 {
		t.Errorf("件数 = %v, want 0", len(got))
	}
}

// 0 件のときに null ではなく空配列として出力されることを確認する。
func TestNoticeMessagesMarshalEmptyAsArray(t *testing.T) {
	jsonData, err := json.Marshal(BuildNoticeMessages(HistoryIncidents{}, UnresolvedIncidents{}))
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	if string(jsonData) != "[]" {
		t.Errorf("json.Marshal() = %v, want []", string(jsonData))
	}
}

// インシデント名にダブルクォートが含まれても壊れないことを確認する
// (文字列連結で JSON を組み立てていた不具合の回帰テスト)。
func TestNoticeMessagesMarshalEscapesQuotes(t *testing.T) {
	noticeMessages := []NoticeMessage{{IncidentName: `Outage "partial"`}}

	jsonData, err := json.Marshal(noticeMessages)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded []NoticeMessage
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded[0].IncidentName != `Outage "partial"` {
		t.Errorf("name = %v, want %v", decoded[0].IncidentName, `Outage "partial"`)
	}
}

func TestUseLocalSample(t *testing.T) {
	tests := []struct {
		name   string
		option LocalSampleOption
		env    string
		setEnv bool
		want   bool
	}{
		{name: "既定値は実 API", want: false},
		{name: "引数で明示指定した場合は引数を優先", option: LocalSampleOption{Value: true, Explicit: true}, want: true},
		{name: "引数は環境変数より優先", option: LocalSampleOption{Value: false, Explicit: true}, env: "true", setEnv: true, want: false},
		{name: "環境変数 true", env: "true", setEnv: true, want: true},
		{name: "環境変数 1", env: "1", setEnv: true, want: true},
		{name: "環境変数 false", env: "false", setEnv: true, want: false},
		{name: "環境変数が不正な値なら既定値", env: "yes-please", setEnv: true, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				t.Setenv(ENV_USE_LOCAL_SAMPLE, tt.env)
			} else if original, ok := os.LookupEnv(ENV_USE_LOCAL_SAMPLE); ok {
				// 実行環境に設定が残っていてもテスト結果が変わらないよう、一時的に取り除く
				os.Unsetenv(ENV_USE_LOCAL_SAMPLE)
				t.Cleanup(func() { os.Setenv(ENV_USE_LOCAL_SAMPLE, original) })
			}

			if got := UseLocalSample(tt.option); got != tt.want {
				t.Errorf("UseLocalSample(%+v) = %v, want %v", tt.option, got, tt.want)
			}
		})
	}
}

func TestBuildResolvedDetails(t *testing.T) {
	historyIncidents := HistoryIncidents{
		Incidents: []HistoryIncident{
			{
				ID:                 "resolved-001",
				Name:               "Incident with Actions",
				Impact:             "major",
				Status:             "resolved",
				ShortLink:          "https://stspg.io/abc",
				ResolvedAt:         "2026-08-31T09:58:14.157Z",
				CreatedAt:          "2026-08-31T09:15:48.908Z",
				AffectedComponents: []AffectedComponent{{Name: "Actions"}, {Name: "Pages"}},
				IncidentUpdates: []IncidentUpdate{
					{Status: "resolved", Body: "This incident has been resolved.", CreatedAt: "2026-08-31T09:58:14.157Z"},
					{Status: "monitoring", Body: "We are monitoring.", CreatedAt: "2026-08-31T09:51:13.994Z"},
				},
			},
			{ID: "postmortem-002", Name: "事後報告つき", Status: "postmortem", ResolvedAt: "2026-08-30T10:00:00Z"},
			{ID: "unresolved-003", Name: "継続中", Status: "investigating"},
		},
	}

	details := BuildResolvedDetails(historyIncidents)

	// postmortem は解決後に事後報告が付いた状態なので、解決済みに含める
	if len(details) != 2 {
		t.Fatalf("件数 = %v, want 2", len(details))
	}
	if _, ok := details["unresolved-003"]; ok {
		t.Error("未解決のインシデントは含めないこと")
	}

	got := details["resolved-001"]
	want := ResolvedDetail{
		Name:       "Incident with Actions",
		Impact:     "major",
		ShortLink:  "https://stspg.io/abc",
		Components: []string{"Actions", "Pages"},
		CreatedAt:  "2026-08-31T09:15:48.908Z",
		ResolvedAt: "2026-08-31T09:58:14.157Z",
		Body:       "This incident has been resolved.",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("解決後の情報 = %+v, want %+v", got, want)
	}
}

func TestLatestUpdateBody(t *testing.T) {
	tests := []struct {
		name            string
		incidentUpdates []IncidentUpdate
		want            string
	}{
		{name: "更新が無ければ空", incidentUpdates: []IncidentUpdate{}, want: ""},
		{
			// API は新しい順で返すが、その並びには頼らず日時で選ぶ
			name: "日時が最も新しい本文を選ぶ",
			incidentUpdates: []IncidentUpdate{
				{Body: "古い", CreatedAt: "2026-08-31T09:15:48.908Z"},
				{Body: "新しい", CreatedAt: "2026-08-31T09:58:14.157Z"},
				{Body: "中間", CreatedAt: "2026-08-31T09:51:13.994Z"},
			},
			want: "新しい",
		},
		{
			name: "時差があっても時刻として比べる",
			incidentUpdates: []IncidentUpdate{
				{Body: "UTC の 10 時", CreatedAt: "2026-08-31T10:00:00Z"},
				{Body: "JST の 18 時 (UTC 9 時)", CreatedAt: "2026-08-31T18:00:00+09:00"},
			},
			want: "UTC の 10 時",
		},
		{
			name: "日時を読めなければ API の並びの先頭を使う",
			incidentUpdates: []IncidentUpdate{
				{Body: "先頭", CreatedAt: ""},
				{Body: "2 件目", CreatedAt: "不正な日時"},
			},
			want: "先頭",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := LatestUpdateBody(test.incidentUpdates); got != test.want {
				t.Errorf("LatestUpdateBody() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestTruncateBody(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		limit int
		want  string
	}{
		{name: "上限以内はそのまま", body: "abcde", limit: 5, want: "abcde"},
		{name: "前後の空白を落とす", body: "  abc\n", limit: 5, want: "abc"},
		{name: "上限を超えたら切り詰める", body: "abcdef", limit: 5, want: "abcde…"},
		// バイト数で切ると文字の途中で切れて文字化けする
		{name: "日本語は文字数で数える", body: "あいうえお", limit: 3, want: "あいう…"},
		{name: "上限が 0 以下なら切り詰めない", body: "abcdef", limit: 0, want: "abcdef"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := TruncateBody(test.body, test.limit); got != test.want {
				t.Errorf("TruncateBody() = %v, want %v", got, test.want)
			}
		})
	}
}

// 実 API が返す components を読めることを確認する。
// 以前は affected_components だけを見ていたため、通知のコンポーネント欄が常に空だった。
func TestComponentNamesUsesComponentsField(t *testing.T) {
	historyIncident := HistoryIncident{
		Components: []AffectedComponent{{Name: "Actions"}, {Name: "Pages"}},
	}
	if got := historyIncident.ComponentNames(); !equalStrings(got, []string{"Actions", "Pages"}) {
		t.Errorf("過去の一覧 = %v", got)
	}

	unresolvedIncident := UnresolvedIncident{
		Components: []AffectedComponent{{Name: "Codespaces"}},
	}
	if got := unresolvedIncident.ComponentNames(); !equalStrings(got, []string{"Codespaces"}) {
		t.Errorf("未解決一覧 = %v", got)
	}

	// components が空のときは affected_components を見る (サンプルデータがこの形)
	legacy := HistoryIncident{
		AffectedComponents: []AffectedComponent{{Name: "Webhooks"}},
	}
	if got := legacy.ComponentNames(); !equalStrings(got, []string{"Webhooks"}) {
		t.Errorf("旧形式 = %v", got)
	}
}

// 復旧の通知に載せるコンポーネントが、実 API の components から取れることを確認する。
func TestBuildResolvedDetailsUsesComponentsField(t *testing.T) {
	historyIncidents := HistoryIncidents{
		Incidents: []HistoryIncident{
			{
				ID:         "resolved",
				Status:     "resolved",
				Components: []AffectedComponent{{Name: "Pull Requests"}},
			},
		},
	}

	details := BuildResolvedDetails(historyIncidents)

	if got := details["resolved"].Components; !equalStrings(got, []string{"Pull Requests"}) {
		t.Errorf("コンポーネント = %v", got)
	}
}

// 未解決一覧が返すコンポーネントを優先し、空のときだけ過去の一覧で補うことを確認する。
func TestBuildNoticeMessagesPrefersUnresolvedComponents(t *testing.T) {
	historyIncidents := HistoryIncidents{
		Incidents: []HistoryIncident{
			{ID: "a", Components: []AffectedComponent{{Name: "過去の一覧"}}},
			{ID: "b", Components: []AffectedComponent{{Name: "補完された値"}}},
		},
	}
	unresolvedIncidents := UnresolvedIncidents{
		Incidents: []UnresolvedIncident{
			{ID: "a", Components: []AffectedComponent{{Name: "未解決一覧"}}},
			{ID: "b"},
		},
	}

	got := BuildNoticeMessages(historyIncidents, unresolvedIncidents)

	if len(got) != 2 {
		t.Fatalf("件数 = %v, want 2", len(got))
	}
	if !equalStrings(got[0].IncidentComponents, []string{"未解決一覧"}) {
		t.Errorf("a のコンポーネント = %v", got[0].IncidentComponents)
	}
	if !equalStrings(got[1].IncidentComponents, []string{"補完された値"}) {
		t.Errorf("b のコンポーネント = %v", got[1].IncidentComponents)
	}
}

// 変化の判定に必要な 3 つの情報がまとめて組み立てられることを確認する。
func TestBuildIncidentSnapshot(t *testing.T) {
	historyIncidents := HistoryIncidents{
		Page: IncidentPage{UpdateAt: "2026-08-31T10:00:00Z"},
		Incidents: []HistoryIncident{
			{ID: "resolved", Status: "resolved", ResolvedAt: "2026-08-31T09:00:00Z"},
			{ID: "postmortem", Status: "postmortem", ResolvedAt: "2026-08-30T09:00:00Z"},
			{ID: "ongoing", Status: "monitoring"},
		},
	}
	unresolvedIncidents := UnresolvedIncidents{
		Incidents: []UnresolvedIncident{{ID: "ongoing", Status: "monitoring"}},
	}

	got := BuildIncidentSnapshot(historyIncidents, unresolvedIncidents)

	if got.ObservedAt != "2026-08-31T10:00:00Z" {
		t.Errorf("観測時点 = %v", got.ObservedAt)
	}
	if len(got.Notices) != 1 || got.Notices[0].IncidentID != "ongoing" {
		t.Errorf("通知メッセージ = %+v", got.Notices)
	}
	// postmortem は解決後に事後報告が付いた状態のため解決済みに含める
	if len(got.Resolved) != 2 {
		t.Errorf("解決済み = %v, want 2 件", len(got.Resolved))
	}
	if !got.Ongoing["ongoing"] || got.Ongoing["resolved"] {
		t.Errorf("未解決 = %v", got.Ongoing)
	}
}
