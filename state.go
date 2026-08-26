package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// NotifiedState は前回までに通知したインシデントを記録する。
// キーはインシデント ID、値はそのとき通知した updated_at。
type NotifiedState struct {
	Incidents map[string]string `json:"incidents"`
}

// LoadNotifiedState は通知済み状態をファイルから読み込む。
// ファイルが存在しない場合は空の状態を返す (初回実行時)。
func LoadNotifiedState(filePath string) (NotifiedState, error) {
	state := NotifiedState{Incidents: map[string]string{}}

	jsonData, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return state, fmt.Errorf("ファイルの読み込みに失敗しました (%v): %w", filePath, err)
	}

	if err := json.Unmarshal(jsonData, &state); err != nil {
		return NotifiedState{Incidents: map[string]string{}}, fmt.Errorf("JSON の解析に失敗しました (%v): %w", filePath, err)
	}
	if state.Incidents == nil {
		state.Incidents = map[string]string{}
	}

	return state, nil
}

// Unnotified は未通知のインシデントだけを絞り込む。
// 未記録のインシデント、および記録時から updated_at が変化したインシデントを対象とする。
// 0 件でも nil ではなく空スライスを返す。
func (state NotifiedState) Unnotified(noticeMessages []NoticeMessage) []NoticeMessage {
	unnotified := []NoticeMessage{}

	for _, noticeMessage := range noticeMessages {
		notifiedUpdatedAt, ok := state.Incidents[noticeMessage.IncidentID]
		if ok && notifiedUpdatedAt == noticeMessage.IncidentUpdatedAt {
			continue
		}
		unnotified = append(unnotified, noticeMessage)
	}

	return unnotified
}

// NewNotifiedState は現在のインシデントから通知済み状態を組み立てる。
// 解決済みとなり未解決一覧から消えたインシデントは引き継がず、記録が際限なく増えるのを防ぐ。
func NewNotifiedState(noticeMessages []NoticeMessage) NotifiedState {
	incidents := map[string]string{}
	for _, noticeMessage := range noticeMessages {
		incidents[noticeMessage.IncidentID] = noticeMessage.IncidentUpdatedAt
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
