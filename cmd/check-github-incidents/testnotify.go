package main

import (
	"log"
	"time"
)

// TEST_NOTIFY_NAME_PREFIX はテスト通知のインシデント名に付ける接頭辞。
// 本物のインシデントと取り違えられないよう、通知を見ただけで分かるようにする。
const TEST_NOTIFY_NAME_PREFIX string = "[テスト通知] "

// SampleIncidentChanges はテスト通知で送るサンプルの変化を組み立てる。
// 実際の通知と同じ見た目を確認できるよう、新規・更新・復旧を 1 件ずつ含める。
// 復旧は未解決一覧から消えた状態を表すため、Incident を持たせず、
// 前回の記録 (Previous) と解決後の情報 (Resolved) を設定する。
func SampleIncidentChanges(now time.Time) []IncidentChange {
	createdAt := now.Add(-30 * time.Minute).Format(time.RFC3339)
	updatedAt := now.Format(time.RFC3339)

	sample := func(id string, status string) NoticeMessage {
		return NoticeMessage{
			IncidentID:         id,
			IncidentImpact:     "major",
			IncidentName:       TEST_NOTIFY_NAME_PREFIX + "Example Incident",
			IncidentStatus:     status,
			IncidentShortLink:  GITHUB_COMMON_URL,
			IncidentComponents: []string{"Actions", "Pages"},
			IncidentCreatedAt:  createdAt,
			IncidentUpdatedAt:  updatedAt,
		}
	}

	newIncident := sample("test-notify-new", "investigating")
	updatedIncident := sample("test-notify-updated", "identified")
	resolvedPrevious := NotifiedIncident{
		Name:      TEST_NOTIFY_NAME_PREFIX + "Example Incident",
		Status:    "monitoring",
		UpdatedAt: createdAt,
	}
	resolvedDetail := ResolvedDetail{
		Name:       resolvedPrevious.Name,
		Impact:     "major",
		ShortLink:  GITHUB_COMMON_URL,
		Components: []string{"Actions", "Pages"},
		CreatedAt:  createdAt,
		ResolvedAt: updatedAt,
		Body:       "This incident has been resolved.",
	}

	return []IncidentChange{
		{
			Type:     CHANGE_NEW,
			ID:       newIncident.IncidentID,
			Name:     newIncident.IncidentName,
			Incident: &newIncident,
		},
		{
			Type:     CHANGE_UPDATED,
			ID:       updatedIncident.IncidentID,
			Name:     updatedIncident.IncidentName,
			Incident: &updatedIncident,
			Previous: &NotifiedIncident{Name: updatedIncident.IncidentName, Status: "investigating", UpdatedAt: createdAt},
		},
		{
			Type:     CHANGE_RESOLVED,
			ID:       "test-notify-resolved",
			Name:     resolvedDetail.Name,
			Previous: &resolvedPrevious,
			Resolved: &resolvedDetail,
		},
	}
}

// RunTestNotify はサンプルのインシデントを通知先へ送る。
// Status API は参照せず、notice_message.json と notified_incidents.json にも触れないため、
// 本番の記録を壊さずに通知先の疎通と見た目を確認できる。
func RunTestNotify(now time.Time, dryRun bool) error {
	log.Printf("%v %v\n", "[INFO]", "test-notify: サンプルのインシデントを通知します。記録ファイルと出力ファイルは更新しません。")

	notifiers := NewNotifiers()
	if len(notifiers) == 0 {
		// 通知先が無いと何も送られないまま正常終了してしまうため、確認できるよう警告を出す
		log.Printf("%v %v\n", "[WARNING]", ".env に "+ENV_DISCORD_WEBHOOK_URL+" または "+ENV_SLACK_WEBHOOK_URL+" を設定してください。")
	}

	return SendNotifications(notifiers, SampleIncidentChanges(now), dryRun)
}
