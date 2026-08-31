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
	Title string `json:"title"`
	URL   string `json:"url,omitempty"`
	// Description は GitHub が投稿したコメント本文。無いときは項目ごと省く
	Description string              `json:"description,omitempty"`
	Color       int                 `json:"color"`
	Fields      []DiscordEmbedField `json:"fields"`
}

type DiscordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

// DISCORD_RESOLVED_COLOR は復旧を知らせる embed の色 (緑)。
// 復旧の通知は影響度に関わらずこの色を用いる。
const DISCORD_RESOLVED_COLOR int = 0x43A047

// DiscordChangeLabel は変化の種類に対応する見出しのラベルを返す。
// Discord は Unicode の絵文字をそのまま表示できるため、文字として埋め込む。
func DiscordChangeLabel(changeType ChangeType) string {
	switch changeType {
	case CHANGE_NEW:
		return "🚨 新しいインシデント"
	case CHANGE_UPDATED:
		return "🔄 状況が更新されました"
	case CHANGE_RESOLVED:
		return "✅ 復旧しました"
	default:
		return ""
	}
}

// BuildPayload は変化したインシデント 1 件を 1 つの embed として組み立てる。
func (notifier DiscordNotifier) BuildPayload(changes []IncidentChange) any {
	limited, omitted := LimitChanges(changes)

	embeds := []DiscordEmbed{}
	for _, change := range limited {
		embeds = append(embeds, DiscordChangeEmbed(change))
	}

	return DiscordPayload{
		Content: NotificationSummary(changes, omitted),
		Embeds:  embeds,
	}
}

// DiscordChangeEmbed は変化 1 件を embed へ変換する。
func DiscordChangeEmbed(change IncidentChange) DiscordEmbed {
	if change.Type == CHANGE_RESOLVED || change.Incident == nil {
		return DiscordResolvedEmbed(change)
	}

	incident := change.Incident

	return DiscordEmbed{
		Title: ChangeTitle(DiscordChangeLabel(change.Type), change),
		URL:   incident.IncidentShortLink,
		Color: DiscordImpactColor(incident.IncidentImpact),
		Fields: []DiscordEmbedField{
			{Name: "影響度", Value: incident.IncidentImpact, Inline: true},
			{Name: "ステータス", Value: incident.IncidentStatus, Inline: true},
			{Name: "コンポーネント", Value: FormatComponents(incident.IncidentComponents), Inline: false},
			{Name: "発生日時", Value: incident.IncidentCreatedAt, Inline: true},
			{Name: "最終更新", Value: incident.IncidentUpdatedAt, Inline: true},
		},
	}
}

// DiscordResolvedEmbed は復旧 1 件を embed へ変換する。
// 復旧したインシデントは未解決一覧から消えるが、過去のインシデント一覧には
// 解決後の情報が残っているため、そちらから組み立てる。
// 取得できなかった場合だけ、前回通知した時点の記録で代替する。
func DiscordResolvedEmbed(change IncidentChange) DiscordEmbed {
	embed := DiscordEmbed{
		Title:  ChangeTitle(DiscordChangeLabel(change.Type), change),
		Color:  DISCORD_RESOLVED_COLOR,
		Fields: []DiscordEmbedField{},
	}

	if resolved := change.Resolved; resolved != nil {
		embed.URL = resolved.ShortLink
		embed.Description = resolved.Body
		embed.Fields = append(embed.Fields,
			DiscordEmbedField{Name: "影響度", Value: resolved.Impact, Inline: true},
			DiscordEmbedField{Name: "コンポーネント", Value: FormatComponents(resolved.Components), Inline: false},
			DiscordEmbedField{Name: "発生日時", Value: resolved.CreatedAt, Inline: true},
			DiscordEmbedField{Name: "復旧日時", Value: resolved.ResolvedAt, Inline: true},
		)
		return embed
	}

	if change.Previous != nil {
		embed.Fields = append(embed.Fields,
			DiscordEmbedField{Name: "直前のステータス", Value: change.Previous.Status, Inline: true},
			DiscordEmbedField{Name: "前回の更新", Value: change.Previous.UpdatedAt, Inline: true},
		)
	}

	return embed
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
