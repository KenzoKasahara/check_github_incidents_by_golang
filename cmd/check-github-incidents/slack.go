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

// BuildPayload はインシデント 1 件を 1 つの attachment として組み立てる。
func (notifier SlackNotifier) BuildPayload(noticeMessages []NoticeMessage) any {
	limited, omitted := LimitNoticeMessages(noticeMessages)

	attachments := []SlackAttachment{}
	for _, noticeMessage := range limited {
		attachments = append(attachments, SlackAttachment{
			Color:     SlackImpactColor(noticeMessage.IncidentImpact),
			Title:     noticeMessage.IncidentName,
			TitleLink: noticeMessage.IncidentShortLink,
			Fields: []SlackField{
				{Title: "影響度", Value: noticeMessage.IncidentImpact, Short: true},
				{Title: "ステータス", Value: noticeMessage.IncidentStatus, Short: true},
				{Title: "コンポーネント", Value: FormatComponents(noticeMessage.IncidentComponents), Short: false},
				{Title: "発生日時", Value: noticeMessage.IncidentCreatedAt, Short: true},
				{Title: "最終更新", Value: noticeMessage.IncidentUpdatedAt, Short: true},
			},
		})
	}

	return SlackPayload{
		Text:        NotificationSummary(limited, omitted),
		Attachments: attachments,
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
