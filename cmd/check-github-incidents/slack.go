package main

// SlackNotifier は Slack の Incoming Webhook へ通知する。
type SlackNotifier struct {
	Webhook string
}

func (notifier SlackNotifier) Name() string {
	return "Slack"
}

func (notifier SlackNotifier) WebhookURL() string {
	return notifier.Webhook
}

// SlackPayload は Slack Webhook のリクエストボディを表す。
type SlackPayload struct {
	Text        string            `json:"text"`
	Attachments []SlackAttachment `json:"attachments,omitempty"`
}

type SlackAttachment struct {
	Color     string       `json:"color"`
	Title     string       `json:"title"`
	TitleLink string       `json:"title_link,omitempty"`
	Fields    []SlackField `json:"fields"`
}

type SlackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// SLACK_RESOLVED_COLOR は復旧を知らせる attachment の色 (緑)。
// 復旧の通知は影響度に関わらずこの色を用いる。
const SLACK_RESOLVED_COLOR string = "#43A047"

// SlackChangeLabel は変化の種類に対応する見出しのラベルを返す。
// Slack はショートコード (:name:) を絵文字へ変換して表示するため、そちらの表記で埋め込む。
func SlackChangeLabel(changeType ChangeType) string {
	switch changeType {
	case CHANGE_NEW:
		return ":rotating_light: 新しいインシデント"
	case CHANGE_UPDATED:
		return ":arrows_counterclockwise: 状況が更新されました"
	case CHANGE_RESOLVED:
		return ":white_check_mark: 復旧しました"
	default:
		return ""
	}
}

// BuildPayload は変化したインシデント 1 件を 1 つの attachment として組み立てる。
func (notifier SlackNotifier) BuildPayload(changes []IncidentChange) any {
	limited, omitted := LimitChanges(changes)

	attachments := []SlackAttachment{}
	for _, change := range limited {
		attachments = append(attachments, SlackChangeAttachment(change))
	}

	return SlackPayload{
		Text:        NotificationSummary(changes, omitted),
		Attachments: attachments,
	}
}

// SlackChangeAttachment は変化 1 件を attachment へ変換する。
// 復旧したインシデントは未解決一覧から消えており影響度やコンポーネントを取得できないため、
// 前回通知した時点の記録から組み立てる。
func SlackChangeAttachment(change IncidentChange) SlackAttachment {
	if change.Type == CHANGE_RESOLVED || change.Incident == nil {
		attachment := SlackAttachment{
			Title:  ChangeTitle(SlackChangeLabel(change.Type), change),
			Color:  SLACK_RESOLVED_COLOR,
			Fields: []SlackField{},
		}
		if change.Previous != nil {
			attachment.Fields = append(attachment.Fields,
				SlackField{Title: "直前のステータス", Value: change.Previous.Status, Short: true},
				SlackField{Title: "前回の更新", Value: change.Previous.UpdatedAt, Short: true},
			)
		}
		return attachment
	}

	incident := change.Incident

	return SlackAttachment{
		Color:     SlackImpactColor(incident.IncidentImpact),
		Title:     ChangeTitle(SlackChangeLabel(change.Type), change),
		TitleLink: incident.IncidentShortLink,
		Fields: []SlackField{
			{Title: "影響度", Value: incident.IncidentImpact, Short: true},
			{Title: "ステータス", Value: incident.IncidentStatus, Short: true},
			{Title: "コンポーネント", Value: FormatComponents(incident.IncidentComponents), Short: false},
			{Title: "発生日時", Value: incident.IncidentCreatedAt, Short: true},
			{Title: "最終更新", Value: incident.IncidentUpdatedAt, Short: true},
		},
	}
}

// SlackImpactColor は影響度に対応する attachment の色を返す。
func SlackImpactColor(impact string) string {
	switch impact {
	case "critical":
		return "#D32F2F"
	case "major":
		return "#F57C00"
	case "minor":
		return "#FBC02D"
	default:
		return "#9E9E9E"
	}
}
