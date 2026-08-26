package main

import (
	"encoding/json"
	"fmt"
	"os"
)

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

// BuildNoticeMessages は過去のインシデントと未解決のインシデントをインシデント ID で突合し、
// 重複するインシデントから通知メッセージを組み立てる。
// 影響を受けたコンポーネントは未解決インシデント側に含まれないため、突合した過去のインシデントから取得する。
// 0 件でも nil ではなく空スライスを返す (JSON へ null ではなく空配列 [] として出力するため)。
func BuildNoticeMessages(historyIncidents HistoryIncidents, unresolvedIncidents UnresolvedIncidents) []NoticeMessage {
	noticeMessages := []NoticeMessage{}

	for _, historyIncident := range historyIncidents.Incidents {
		for _, unresolvedIncident := range unresolvedIncidents.Incidents {
			if historyIncident.ID != unresolvedIncident.ID {
				continue
			}

			noticeMessages = append(noticeMessages, NoticeMessage{
				IncidentID:         unresolvedIncident.ID,
				IncidentImpact:     unresolvedIncident.Impact,
				IncidentName:       unresolvedIncident.Name,
				IncidentStatus:     unresolvedIncident.Status,
				IncidentShortLink:  unresolvedIncident.ShortLink,
				IncidentComponents: AffectedComponentNames(historyIncident.AffectedComponents),
				IncidentCreatedAt:  unresolvedIncident.CreatedAt,
				IncidentUpdatedAt:  unresolvedIncident.UpdatedAt,
			})
		}
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
