package main

// DiscordNotifier は Discord の Incoming Webhook へ通知する。
type DiscordNotifier struct {
	Webhook string
}

func (notifier DiscordNotifier) Name() string {
	return "Discord"
}

func (notifier DiscordNotifier) WebhookURL() string {
	return notifier.Webhook
}

// DiscordPayload は Discord Webhook のリクエストボディを表す。
type DiscordPayload struct {
	Content string         `json:"content"`
	Embeds  []DiscordEmbed `json:"embeds,omitempty"`
}

type DiscordEmbed struct {
	Title  string              `json:"title"`
	URL    string              `json:"url,omitempty"`
	Color  int                 `json:"color"`
	Fields []DiscordEmbedField `json:"fields"`
}

type DiscordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

// BuildPayload はインシデント 1 件を 1 つの embed として組み立てる。
func (notifier DiscordNotifier) BuildPayload(noticeMessages []NoticeMessage) any {
	limited, omitted := LimitNoticeMessages(noticeMessages)

	embeds := []DiscordEmbed{}
	for _, noticeMessage := range limited {
		embeds = append(embeds, DiscordEmbed{
			Title: noticeMessage.IncidentName,
			URL:   noticeMessage.IncidentShortLink,
			Color: DiscordImpactColor(noticeMessage.IncidentImpact),
			Fields: []DiscordEmbedField{
				{Name: "影響度", Value: noticeMessage.IncidentImpact, Inline: true},
				{Name: "ステータス", Value: noticeMessage.IncidentStatus, Inline: true},
				{Name: "コンポーネント", Value: FormatComponents(noticeMessage.IncidentComponents), Inline: false},
				{Name: "発生日時", Value: noticeMessage.IncidentCreatedAt, Inline: true},
				{Name: "最終更新", Value: noticeMessage.IncidentUpdatedAt, Inline: true},
			},
		})
	}

	return DiscordPayload{
		Content: NotificationSummary(limited, omitted),
		Embeds:  embeds,
	}
}

// DiscordImpactColor は影響度に対応する embed の色を返す。
func DiscordImpactColor(impact string) int {
	switch impact {
	case "critical":
		return 0xD32F2F
	case "major":
		return 0xF57C00
	case "minor":
		return 0xFBC02D
	default:
		return 0x9E9E9E
	}
}
