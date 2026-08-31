package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"
)

const NOTIFIED_STATE_FILE_PATH string = "./notified_incidents.json"

// ChangeType は前回の通知内容と比べたときの変化の種類を表す。
type ChangeType string

const (
	// CHANGE_NEW は前回の記録に無い、新しく発生したインシデント。
	CHANGE_NEW ChangeType = "new"
	// CHANGE_UPDATED は前回の記録から updated_at が変化したインシデント。
	CHANGE_UPDATED ChangeType = "updated"
	// CHANGE_RESOLVED は前回の記録にあり、今回の未解決一覧から消えたインシデント。
	CHANGE_RESOLVED ChangeType = "resolved"
)

// NotifiedIncident は通知済みインシデント 1 件分の記録を表す。
// 復旧したインシデントは未解決一覧から消えて名称を取得できなくなるため、
// 差分の判定に使う updated_at だけでなく、通知に必要な name と status も残しておく。
type NotifiedIncident struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updated_at"`
}

// NotifiedState は前回までに通知したインシデントを記録する。
// キーはインシデント ID。
type NotifiedState struct {
	Incidents map[string]NotifiedIncident `json:"incidents"`
	// LastCheckedAt は前回の実行でどこまで確認したかを表す目印。
	// 未解決の状態を一度も観測できなかったインシデントの復旧を、
	// 過去の分までさかのぼって通知してしまわないための下限として使う。
	// 実行するマシンの時計のずれに影響されないよう、GitHub 側の時刻を記録する。
	LastCheckedAt string `json:"last_checked_at,omitempty"`
}

// IncidentChange は前回の通知内容と比べて変化したインシデント 1 件分を表す。
type IncidentChange struct {
	Type ChangeType
	ID   string
	Name string
	// Incident は今回取得したインシデント情報。
	// 復旧 (CHANGE_RESOLVED) の場合は未解決一覧に含まれないため nil となる。
	Incident *NoticeMessage
	// Previous は前回通知した時点の記録。
	// 新規 (CHANGE_NEW) の場合や、未解決の状態を観測しないまま解決した場合は nil となる。
	Previous *NotifiedIncident
	// Resolved は過去のインシデント一覧から取得した解決後の情報。
	// 復旧 (CHANGE_RESOLVED) 以外では nil となる。
	// 復旧であっても、過去 50 件から漏れたインシデントでは取得できず nil となる。
	Resolved *ResolvedDetail
}

// LoadNotifiedState は通知済み状態をファイルから読み込む。
// ファイルが存在しない場合は空の状態を返す (初回実行時)。
func LoadNotifiedState(filePath string) (NotifiedState, error) {
	state := NotifiedState{Incidents: map[string]NotifiedIncident{}}

	jsonData, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return state, fmt.Errorf("ファイルの読み込みに失敗しました (%v): %w", filePath, err)
	}

	if err := json.Unmarshal(jsonData, &state); err != nil {
		// 復旧通知に対応する前は値が updated_at の文字列だけだったため、旧形式も読めるようにしておく。
		// 読めなければ通知が全件やり直しになるだけで済むが、いきなり失敗させると定期実行が止まってしまう。
		legacy, legacyErr := parseLegacyNotifiedState(jsonData)
		if legacyErr != nil {
			return NotifiedState{Incidents: map[string]NotifiedIncident{}}, fmt.Errorf("JSON の解析に失敗しました (%v): %w", filePath, err)
		}
		return legacy, nil
	}
	if state.Incidents == nil {
		state.Incidents = map[string]NotifiedIncident{}
	}

	return state, nil
}

// parseLegacyNotifiedState は旧形式 (ID をキー、updated_at を値とする辞書) の記録を読み込む。
// 旧形式には name と status が無いため、次回の保存で新形式へ置き換わるまでは空のままとなる。
func parseLegacyNotifiedState(jsonData []byte) (NotifiedState, error) {
	var legacy struct {
		Incidents map[string]string `json:"incidents"`
	}
	if err := json.Unmarshal(jsonData, &legacy); err != nil {
		return NotifiedState{}, err
	}

	state := NotifiedState{Incidents: map[string]NotifiedIncident{}}
	for id, updatedAt := range legacy.Incidents {
		state.Incidents[id] = NotifiedIncident{UpdatedAt: updatedAt}
	}

	return state, nil
}

// Diff は前回の記録と今回のインシデントを突合し、変化したものだけを返す。
//   - 新規:   今回にあり、前回の記録に無いインシデント
//   - 更新:   両方にあり、updated_at が変化したインシデント
//   - 復旧:   解決済みになったインシデント (判定は resolvedIDs を参照)
//
// 0 件でも nil ではなく空スライスを返す。
func (state NotifiedState) Diff(noticeMessages []NoticeMessage, resolvedDetails map[string]ResolvedDetail) []IncidentChange {
	changes := []IncidentChange{}
	currentIDs := map[string]bool{}

	for _, noticeMessage := range noticeMessages {
		currentIDs[noticeMessage.IncidentID] = true

		previous, ok := state.Incidents[noticeMessage.IncidentID]
		if !ok {
			changes = append(changes, IncidentChange{
				Type:     CHANGE_NEW,
				ID:       noticeMessage.IncidentID,
				Name:     noticeMessage.IncidentName,
				Incident: &noticeMessage,
			})
			continue
		}
		if previous.UpdatedAt == noticeMessage.IncidentUpdatedAt {
			continue
		}
		changes = append(changes, IncidentChange{
			Type:     CHANGE_UPDATED,
			ID:       noticeMessage.IncidentID,
			Name:     noticeMessage.IncidentName,
			Incident: &noticeMessage,
			Previous: &previous,
		})
	}

	// map の反復順は不定なので、通知の並びが実行ごとに変わらないよう ID 順に並べる
	resolvedIDs := state.resolvedIDs(currentIDs, resolvedDetails)
	sort.Strings(resolvedIDs)

	for _, id := range resolvedIDs {
		change := IncidentChange{Type: CHANGE_RESOLVED, ID: id}

		// 名称は解決後の情報を優先する。記録側は通知した時点のもので、名称が変わっていることがあるため
		if previous, ok := state.Incidents[id]; ok {
			change.Name = previous.Name
			change.Previous = &previous
		}
		if detail, ok := resolvedDetails[id]; ok {
			change.Resolved = &detail
			change.Name = detail.Name
		}

		changes = append(changes, change)
	}

	return changes
}

// resolvedIDs は復旧として通知するインシデントの ID を返す。
// currentIDs には今回の未解決一覧にあるインシデントの ID を渡すこと。
func (state NotifiedState) resolvedIDs(currentIDs map[string]bool, resolvedDetails map[string]ResolvedDetail) []string {
	ids := []string{}

	// 前回の記録にあり、今回の未解決一覧から消えたもの
	for id := range state.Incidents {
		if !currentIDs[id] {
			ids = append(ids, id)
		}
	}

	// 未解決の状態を観測しないまま解決したもの。
	// 実行間隔より短い障害や、停止していた間に始まって終わった障害は
	// 消えたことでは気づけないため、解決済みかどうかを直接見る
	for id, detail := range resolvedDetails {
		if currentIDs[id] {
			continue
		}
		// 上で拾い済み
		if _, recorded := state.Incidents[id]; recorded {
			continue
		}
		if !state.isNewlyResolved(detail) {
			continue
		}
		ids = append(ids, id)
	}

	return ids
}

// isNewlyResolved は前回の実行より後に解決したかどうかを返す。
// 過去 50 件には数日前のインシデントも含まれるため、この判定を挟まないと
// すでに終わった障害を毎回通知してしまう。
// 記録が無いとき (初回実行) と日時を解析できないときは、
// 過去の分がまとめて飛ぶのを避けるため通知しない。
func (state NotifiedState) isNewlyResolved(detail ResolvedDetail) bool {
	if state.LastCheckedAt == "" {
		return false
	}

	lastCheckedAt, err := time.Parse(time.RFC3339, state.LastCheckedAt)
	if err != nil {
		return false
	}
	resolvedAt, err := time.Parse(time.RFC3339, detail.ResolvedAt)
	if err != nil {
		return false
	}

	return resolvedAt.After(lastCheckedAt)
}

// CountChanges は変化の種類ごとの件数を数える。
func CountChanges(changes []IncidentChange) (newCount, updatedCount, resolvedCount int) {
	for _, change := range changes {
		switch change.Type {
		case CHANGE_NEW:
			newCount++
		case CHANGE_UPDATED:
			updatedCount++
		case CHANGE_RESOLVED:
			resolvedCount++
		}
	}

	return newCount, updatedCount, resolvedCount
}

// NewNotifiedState は現在のインシデントから通知済み状態を組み立てる。
// 解決済みとなり未解決一覧から消えたインシデントは引き継がず、記録が際限なく増えるのを防ぐ。
// (復旧の通知は、記録から消える前の実行で済ませている)
// checkedAt には CheckedAt が返す、今回どこまで確認したかを表す目印を渡す。
func NewNotifiedState(noticeMessages []NoticeMessage, checkedAt string) NotifiedState {
	incidents := map[string]NotifiedIncident{}
	for _, noticeMessage := range noticeMessages {
		incidents[noticeMessage.IncidentID] = NotifiedIncident{
			Name:      noticeMessage.IncidentName,
			Status:    noticeMessage.IncidentStatus,
			UpdatedAt: noticeMessage.IncidentUpdatedAt,
		}
	}

	return NotifiedState{Incidents: incidents, LastCheckedAt: checkedAt}
}

// CheckedAt は今回どこまで確認したかを表す目印を返す。
// 実行するマシンの時計がずれていても復旧の取りこぼしや二重通知が起きないよう、
// 現在時刻ではなく GitHub が返した時刻を使う。
// ページの更新日時が復旧日時に追いついていないことがあるため、
// 両者のうち最も新しいものを採る。
// GitHub 側の時刻を 1 つも解析できなかった場合のみ、現在時刻を用いる。
func CheckedAt(historyIncidents HistoryIncidents, now time.Time) string {
	var latest time.Time
	found := false

	candidates := []string{historyIncidents.Page.UpdateAt}
	for _, historyIncident := range historyIncidents.Incidents {
		if IsResolvedStatus(historyIncident.Status) {
			candidates = append(candidates, historyIncident.ResolvedAt)
		}
	}

	for _, candidate := range candidates {
		parsed, err := time.Parse(time.RFC3339, candidate)
		if err != nil {
			continue
		}
		if !found || parsed.After(latest) {
			latest, found = parsed, true
		}
	}

	if !found {
		return now.UTC().Format(time.RFC3339Nano)
	}

	return latest.UTC().Format(time.RFC3339Nano)
}

// SaveNotifiedState は通知済み状態をファイルへ書き出す。
func SaveNotifiedState(filePath string, state NotifiedState) error {
	indentJsonData, err := json.MarshalIndent(state, "", "    ")
	if err != nil {
		return fmt.Errorf("通知済み状態の JSON エンコードに失敗しました: %w", err)
	}

	if err := os.WriteFile(filePath, indentJsonData, 0600); err != nil {
		return fmt.Errorf("ファイルの書き込みに失敗しました (%v): %w", filePath, err)
	}

	return nil
}
