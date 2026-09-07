package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

const NOTICE_FILE_PATH string = "./notice_message.json"

// NoticeMessage は notice_message.json に出力する通知メッセージ 1 件分を表す。
type NoticeMessage struct {
	IncidentID         string   `json:"id"`
	IncidentImpact     string   `json:"impact"`
	IncidentName       string   `json:"name"`
	IncidentStatus     string   `json:"status"`
	IncidentShortLink  string   `json:"shortlink"`
	IncidentComponents []string `json:"components"`
	IncidentCreatedAt  string   `json:"created_at"`
	IncidentUpdatedAt  string   `json:"updated_at"`
}

// BuildNoticeMessages は未解決のインシデントから通知メッセージを組み立てる。
// 発生中かどうかの判断は公式の未解決一覧 (/api/v2/incidents/unresolved.json) を唯一の根拠とし、
// 過去のインシデント一覧は、未解決一覧に含まれない「影響を受けたコンポーネント」の補完だけに使う。
// 過去のインシデント一覧は直近 50 件しか返らず、反映も未解決一覧より遅れることがあるため、
// 両方に載っていることを条件にすると発生を取りこぼし、記録済みのインシデントが
// 一時的に消えたように見えて誤った復旧通知が飛ぶ。
// 0 件でも nil ではなく空スライスを返す (JSON へ null ではなく空配列 [] として出力するため)。
func BuildNoticeMessages(historyIncidents HistoryIncidents, unresolvedIncidents UnresolvedIncidents) []NoticeMessage {
	componentsByID := map[string][]string{}
	for _, historyIncident := range historyIncidents.Incidents {
		componentsByID[historyIncident.ID] = historyIncident.ComponentNames()
	}

	noticeMessages := []NoticeMessage{}
	for _, unresolvedIncident := range unresolvedIncidents.Incidents {
		// 未解決一覧もコンポーネントを返すため、まずはそちらを使う。
		// 空のときだけ過去のインシデント一覧で補う
		components := unresolvedIncident.ComponentNames()
		if len(components) == 0 {
			components = componentsByID[unresolvedIncident.ID]
		}
		if components == nil {
			components = []string{}
		}

		noticeMessages = append(noticeMessages, NoticeMessage{
			IncidentID:         unresolvedIncident.ID,
			IncidentImpact:     unresolvedIncident.Impact,
			IncidentName:       unresolvedIncident.Name,
			IncidentStatus:     unresolvedIncident.Status,
			IncidentShortLink:  unresolvedIncident.ShortLink,
			IncidentComponents: components,
			IncidentCreatedAt:  unresolvedIncident.CreatedAt,
			IncidentUpdatedAt:  unresolvedIncident.UpdatedAt,
		})
	}

	return noticeMessages
}

// AffectedComponentNames は影響を受けたコンポーネントの名称のみを取り出す。
func AffectedComponentNames(affectedComponents []AffectedComponent) []string {
	names := []string{}
	for _, affectedComponent := range affectedComponents {
		names = append(names, affectedComponent.Name)
	}

	return names
}

// WriteNoticeMessages は通知メッセージを整形した JSON としてファイルへ書き出す。
func WriteNoticeMessages(filePath string, noticeMessages []NoticeMessage) error {
	indentJsonData, err := json.MarshalIndent(noticeMessages, "", "    ")
	if err != nil {
		return fmt.Errorf("通知メッセージの JSON エンコードに失敗しました: %w", err)
	}

	if err := os.WriteFile(filePath, indentJsonData, 0666); err != nil {
		return fmt.Errorf("ファイルの書き込みに失敗しました (%v): %w", filePath, err)
	}

	return nil
}

// RESOLVED_BODY_LIMIT は復旧の通知に載せるコメント本文の上限文字数。
// GitHub のコメントは長くなることがあり、そのまま載せると通知が読みにくくなるため切り詰める。
const RESOLVED_BODY_LIMIT int = 300

// ResolvedDetail は解決済みインシデントの詳細を表す。
// 未解決一覧からは消えているが、過去のインシデント一覧には解決後の情報が残っているため、
// 影響度・コンポーネント・復旧日時・復旧コメントはそちらから取得する。
type ResolvedDetail struct {
	Name       string
	Impact     string
	ShortLink  string
	Components []string
	CreatedAt  string
	ResolvedAt string
	Body       string
}

// IsResolvedStatus は解決済みとみなすステータスかどうかを返す。
// postmortem は解決後に事後報告が付いた状態であり、障害としては解消しているため解決済みに含める。
func IsResolvedStatus(status string) bool {
	return status == "resolved" || status == "postmortem"
}

// IncidentSnapshot は 1 回の実行で GitHub から取得した状態をまとめたもの。
// 復旧の判定には未解決一覧と過去のインシデント一覧の両方が要るため、まとめて受け渡す。
type IncidentSnapshot struct {
	// ObservedAt は GitHub 側で観測した時刻 (incidents.json の page.updated_at)。
	// 実行するマシンの時計のずれに左右されないよう、現在時刻の代わりに用いる。
	ObservedAt string
	// Notices は未解決一覧から組み立てた通知メッセージ。
	Notices []NoticeMessage
	// Resolved は過去のインシデント一覧で解決済みになっているもの。キーはインシデント ID。
	Resolved map[string]ResolvedDetail
	// Ongoing は過去のインシデント一覧にあるが、まだ解決済みになっていないもの。キーはインシデント ID。
	// 未解決一覧への反映が遅れているだけのインシデントを、復旧と取り違えないために使う。
	Ongoing map[string]bool
}

// BuildIncidentSnapshot は取得した 2 つの一覧から、変化の判定に必要な情報を組み立てる。
func BuildIncidentSnapshot(historyIncidents HistoryIncidents, unresolvedIncidents UnresolvedIncidents) IncidentSnapshot {
	return IncidentSnapshot{
		ObservedAt: historyIncidents.Page.UpdateAt,
		Notices:    BuildNoticeMessages(historyIncidents, unresolvedIncidents),
		Resolved:   BuildResolvedDetails(historyIncidents),
		Ongoing:    BuildOngoingIDs(historyIncidents),
	}
}

// BuildOngoingIDs は過去のインシデント一覧のうち、まだ解決済みになっていないものの ID を返す。
func BuildOngoingIDs(historyIncidents HistoryIncidents) map[string]bool {
	ongoing := map[string]bool{}

	for _, historyIncident := range historyIncidents.Incidents {
		if !IsResolvedStatus(historyIncident.Status) {
			ongoing[historyIncident.ID] = true
		}
	}

	return ongoing
}

// BuildResolvedDetails は過去のインシデントから解決済みのものを取り出し、インシデント ID で引けるようにする。
// 復旧の判定と通知内容の両方に用いる。
func BuildResolvedDetails(historyIncidents HistoryIncidents) map[string]ResolvedDetail {
	details := map[string]ResolvedDetail{}

	for _, historyIncident := range historyIncidents.Incidents {
		if !IsResolvedStatus(historyIncident.Status) {
			continue
		}

		details[historyIncident.ID] = ResolvedDetail{
			Name:       historyIncident.Name,
			Impact:     historyIncident.Impact,
			ShortLink:  historyIncident.ShortLink,
			Components: historyIncident.ComponentNames(),
			CreatedAt:  historyIncident.CreatedAt,
			ResolvedAt: historyIncident.ResolvedAt,
			Body:       TruncateBody(LatestUpdateBody(historyIncident.IncidentUpdates), RESOLVED_BODY_LIMIT),
		}
	}

	return details
}

// LatestUpdateBody は最後に投稿された更新のコメント本文を返す。
// API は新しい順に並べて返すが、その並びに頼らず日時で選ぶ。
// 日時を 1 件も解析できなかった場合だけ、API の並びの先頭を使う。
func LatestUpdateBody(incidentUpdates []IncidentUpdate) string {
	body := ""
	var latest time.Time
	found := false

	for _, incidentUpdate := range incidentUpdates {
		createdAt, err := time.Parse(time.RFC3339, incidentUpdate.CreatedAt)
		if err != nil {
			continue
		}
		if !found || createdAt.After(latest) {
			latest, body, found = createdAt, incidentUpdate.Body, true
		}
	}

	if !found && len(incidentUpdates) > 0 {
		return incidentUpdates[0].Body
	}

	return body
}

// TruncateBody は本文を上限まで切り詰める。
// 日本語が混ざっても文字の途中で切れないよう、バイト数ではなく文字数で数える。
func TruncateBody(body string, limit int) string {
	body = strings.TrimSpace(body)

	runes := []rune(body)
	if limit <= 0 || len(runes) <= limit {
		return body
	}

	return string(runes[:limit]) + "…"
}
