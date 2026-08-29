package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
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
	// 新規 (CHANGE_NEW) の場合は記録が無いため nil となる。
	Previous *NotifiedIncident
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

// Diff は前回の記録と今回の未解決インシデントを突合し、変化したものだけを返す。
//   - 新規:   今回にあり、前回の記録に無いインシデント
//   - 更新:   両方にあり、updated_at が変化したインシデント
//   - 復旧:   前回の記録にあり、今回の未解決一覧から消えたインシデント
//
// 0 件でも nil ではなく空スライスを返す。
func (state NotifiedState) Diff(noticeMessages []NoticeMessage) []IncidentChange {
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

	// 復旧したインシデントは今回のレスポンスに含まれないため、名称は前回の記録から取る。
	// map の反復順は不定なので、通知の並びが実行ごとに変わらないよう ID 順に並べる
	resolvedIDs := []string{}
	for id := range state.Incidents {
		if !currentIDs[id] {
			resolvedIDs = append(resolvedIDs, id)
		}
	}
	sort.Strings(resolvedIDs)

	for _, id := range resolvedIDs {
		previous := state.Incidents[id]
		changes = append(changes, IncidentChange{
			Type:     CHANGE_RESOLVED,
			ID:       id,
			Name:     previous.Name,
			Previous: &previous,
		})
	}

	return changes
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
func NewNotifiedState(noticeMessages []NoticeMessage) NotifiedState {
	incidents := map[string]NotifiedIncident{}
	for _, noticeMessage := range noticeMessages {
		incidents[noticeMessage.IncidentID] = NotifiedIncident{
			Name:      noticeMessage.IncidentName,
			Status:    noticeMessage.IncidentStatus,
			UpdatedAt: noticeMessage.IncidentUpdatedAt,
		}
	}

	return NotifiedState{Incidents: incidents}
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
