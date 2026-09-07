package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"
)

const (
	// NOTIFIED_STATE_FILE_PATH は実 API を見たときの記録ファイル。
	NOTIFIED_STATE_FILE_PATH string = "./notified_incidents.json"
	// NOTIFIED_STATE_LOCAL_FILE_PATH はローカルのサンプルを見たとき (-local) の記録ファイル。
	// サンプルの日付は実 API とかけ離れているため、同じファイルに混ぜると
	// 次の実 API 実行で過去のインシデントが一斉に復旧として飛ぶ。
	// 記録を分けておけば、サンプルでも発生から復旧までの流れを一通り確認できる。
	NOTIFIED_STATE_LOCAL_FILE_PATH string = "./notified_incidents.local.json"
)

// RESOLVED_LOOKBACK は last_checked_at がまだ無いときに、復旧を通知する範囲として遡る時間。
// 記録が無くても直前に解決した障害は通知できるようにしつつ、
// 過去のインシデント一覧に残る古い障害がまとめて飛ばないよう範囲を区切る。
const RESOLVED_LOOKBACK time.Duration = 24 * time.Hour

// NotifiedStateFilePath はデータの取得元に応じた記録ファイルのパスを返す。
func NotifiedStateFilePath(useLocal bool) string {
	if useLocal {
		return NOTIFIED_STATE_LOCAL_FILE_PATH
	}

	return NOTIFIED_STATE_FILE_PATH
}

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
	// 一度進めた時点は巻き戻さない (CheckedAt を参照)。
	LastCheckedAt string `json:"last_checked_at,omitempty"`
	// ResolvedNotified は復旧を通知済みのインシデント。キーはインシデント ID、値は復旧日時。
	// 復旧の判定を公式の status に任せる以上、「記録から消えたこと」では二重通知を防げないため、
	// 通知した復旧をここに残して次回以降の対象から外す。
	// 判定に影響しなくなったものは保存時に取り除くので、際限なく増えることはない。
	ResolvedNotified map[string]string `json:"resolved_notified,omitempty"`
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
	state := NotifiedState{Incidents: map[string]NotifiedIncident{}, ResolvedNotified: map[string]string{}}

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
			return NotifiedState{Incidents: map[string]NotifiedIncident{}, ResolvedNotified: map[string]string{}}, fmt.Errorf("JSON の解析に失敗しました (%v): %w", filePath, err)
		}
		return legacy, nil
	}
	if state.Incidents == nil {
		state.Incidents = map[string]NotifiedIncident{}
	}
	if state.ResolvedNotified == nil {
		state.ResolvedNotified = map[string]string{}
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

	state := NotifiedState{Incidents: map[string]NotifiedIncident{}, ResolvedNotified: map[string]string{}}
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
func (state NotifiedState) Diff(snapshot IncidentSnapshot) []IncidentChange {
	changes := []IncidentChange{}
	currentIDs := map[string]bool{}

	for _, noticeMessage := range snapshot.Notices {
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
	resolvedIDs := state.resolvedIDs(currentIDs, snapshot)
	sort.Strings(resolvedIDs)

	for _, id := range resolvedIDs {
		change := IncidentChange{Type: CHANGE_RESOLVED, ID: id}

		// 名称は解決後の情報を優先する。記録側は通知した時点のもので、名称が変わっていることがあるため
		if previous, ok := state.Incidents[id]; ok {
			change.Name = previous.Name
			change.Previous = &previous
		}
		if detail, ok := snapshot.Resolved[id]; ok {
			change.Resolved = &detail
			change.Name = detail.Name
		}

		changes = append(changes, change)
	}

	return changes
}

// resolvedIDs は復旧として通知するインシデントの ID を返す。
// 判定の主軸は公式の過去のインシデント一覧 (/api/v2/incidents.json) が返す status であり、
// 記録の連続性には依存しない。記録が失われても、公式が解決済みとしていれば復旧を通知できる。
// currentIDs には今回の未解決一覧にあるインシデントの ID を渡すこと。
func (state NotifiedState) resolvedIDs(currentIDs map[string]bool, snapshot IncidentSnapshot) []string {
	ids := []string{}
	lowerBound, hasLowerBound := state.resolvedLowerBound(snapshot.ObservedAt)

	// 公式が解決済み (resolved / postmortem) としているもの
	for id, detail := range snapshot.Resolved {
		// 未解決一覧にも残っている間はまだ収束していないとみなし、次回以降に回す
		if currentIDs[id] {
			continue
		}
		if _, notified := state.ResolvedNotified[id]; notified {
			continue
		}

		// 未解決の状態を観測できていたものは、いつ解決したかに関わらず通知する。
		// 観測できていないものは、過去 50 件に残る古い障害がまとめて飛ばないよう、
		// 下限より後に解決したものだけを通知する
		if _, recorded := state.Incidents[id]; !recorded && !IsResolvedAfter(detail, lowerBound, hasLowerBound) {
			continue
		}

		ids = append(ids, id)
	}

	// 記録にあるのに、未解決一覧にも過去のインシデント一覧にも現れないもの。
	// 過去 50 件から漏れるほど長く続いたインシデントがこれにあたる
	for id := range state.Incidents {
		if currentIDs[id] {
			continue
		}
		// 上で拾い済み
		if _, ok := snapshot.Resolved[id]; ok {
			continue
		}
		// 過去のインシデント一覧に未解決のまま残っているうちは、未解決一覧への反映待ちとみなす。
		// ここで復旧としてしまうと、誤った復旧通知が飛ぶうえに ResolvedNotified へ記録され、
		// 本当に解決したときの通知まで握りつぶしてしまう
		if snapshot.Ongoing[id] {
			continue
		}
		if _, notified := state.ResolvedNotified[id]; notified {
			continue
		}
		ids = append(ids, id)
	}

	return ids
}

// resolvedLowerBound は、未解決の状態を観測できなかったインシデントの復旧を
// どこまで遡って通知するかの下限を返す。
//   - 前回の確認時点 (last_checked_at) があれば、それを下限とする
//   - 無いとき (初回実行や記録を消した直後) は、観測時点から RESOLVED_LOOKBACK 遡った時点を下限とする
//
// 記録が無いという理由だけで復旧を一切通知しないと、記録が残るまでの間に解決した
// 障害の通知が届かないままになるため、範囲を区切ったうえで通知する。
// GitHub 側の時刻を解析できなかった場合のみ、判断できないものとして false を返す。
func (state NotifiedState) resolvedLowerBound(observedAt string) (time.Time, bool) {
	if lastCheckedAt, err := time.Parse(time.RFC3339, state.LastCheckedAt); err == nil {
		return lastCheckedAt, true
	}

	observed, err := time.Parse(time.RFC3339, observedAt)
	if err != nil {
		return time.Time{}, false
	}

	return observed.Add(-RESOLVED_LOOKBACK), true
}

// IsResolvedAfter は下限より後に解決したかどうかを返す。
// 過去 50 件には数日前のインシデントも含まれるため、この判定を挟まないと
// すでに終わった障害を毎回通知してしまう。
// 下限を決められなかったときと、復旧日時を解析できないときは通知しない。
func IsResolvedAfter(detail ResolvedDetail, lowerBound time.Time, hasLowerBound bool) bool {
	if !hasLowerBound {
		return false
	}

	resolvedAt, err := time.Parse(time.RFC3339, detail.ResolvedAt)
	if err != nil {
		return false
	}

	return resolvedAt.After(lowerBound)
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

// NewNotifiedState は今回の実行結果から、次回へ引き継ぐ通知済み状態を組み立てる。
// 解決済みとなり未解決一覧から消えたインシデントは引き継がず、記録が際限なく増えるのを防ぐ。
// 代わりに、今回通知した復旧を ResolvedNotified に残して二重通知を防ぐ。
// previous には今回読み込んだ通知済み状態を、changes には今回通知した変化を、
// checkedAt には CheckedAt が返す、今回どこまで確認したかを表す目印を渡す。
func NewNotifiedState(previous NotifiedState, snapshot IncidentSnapshot, changes []IncidentChange, checkedAt string) NotifiedState {
	incidents := map[string]NotifiedIncident{}
	for _, noticeMessage := range snapshot.Notices {
		incidents[noticeMessage.IncidentID] = NotifiedIncident{
			Name:      noticeMessage.IncidentName,
			Status:    noticeMessage.IncidentStatus,
			UpdatedAt: noticeMessage.IncidentUpdatedAt,
		}
	}
	// 未解決一覧から一時的に消えただけで、過去のインシデント一覧では未解決のまま
	// 残っているインシデントは記録を持ち越す。ここで落とすと、次に現れたときに
	// 新規として通知してしまう
	for id, previousIncident := range previous.Incidents {
		if _, ok := incidents[id]; ok {
			continue
		}
		if snapshot.Ongoing[id] {
			incidents[id] = previousIncident
		}
	}

	resolvedNotified := map[string]string{}
	for id, resolvedAt := range previous.ResolvedNotified {
		// 再び未解決として観測されたものは、次に解決したとき通知できるよう記録から外す。
		// 反映の遅れで一時的に消えたインシデントを復旧として通知してしまった場合に、
		// 本当の復旧まで握りつぶされるのを防ぐ
		if _, unresolved := incidents[id]; unresolved {
			continue
		}
		resolvedNotified[id] = resolvedAt
	}
	for _, change := range changes {
		if change.Type != CHANGE_RESOLVED {
			continue
		}
		// 復旧日時を取得できなかった場合は今回の確認時点で代用する。
		// 空のままにすると日時として解析できず、PruneResolvedNotified で取り除けなくなるため
		resolvedAt := checkedAt
		if change.Resolved != nil && change.Resolved.ResolvedAt != "" {
			resolvedAt = change.Resolved.ResolvedAt
		}
		resolvedNotified[change.ID] = resolvedAt
	}

	return NotifiedState{
		Incidents:        incidents,
		LastCheckedAt:    checkedAt,
		ResolvedNotified: PruneResolvedNotified(resolvedNotified, checkedAt),
	}
}

// PruneResolvedNotified は、もう判定に影響しなくなった復旧の記録を取り除く。
// 復旧日時が確認済みの時点より前になったものは isNewlyResolved が false を返すため、
// 記録から外しても再通知されることはない。
// 日時を解析できないものは、二重通知を避ける側に倒して残す。
func PruneResolvedNotified(resolvedNotified map[string]string, checkedAt string) map[string]string {
	checked, err := time.Parse(time.RFC3339, checkedAt)
	if err != nil {
		return resolvedNotified
	}

	pruned := map[string]string{}
	for id, resolvedAt := range resolvedNotified {
		parsed, err := time.Parse(time.RFC3339, resolvedAt)
		if err != nil {
			pruned[id] = resolvedAt
			continue
		}
		if parsed.Before(checked) {
			continue
		}
		pruned[id] = resolvedAt
	}

	return pruned
}

// CheckedAt は今回どこまで確認したかを表す目印を返す。
// 実行するマシンの時計がずれていても復旧の取りこぼしや二重通知が起きないよう、
// 現在時刻ではなく GitHub が返した時刻を使う。
// ページの更新日時が復旧日時に追いついていないことがあるため、両者のうち最も新しいものを採る。
// 前回の値も候補に含めるのは、解決済みのインシデントが直近 50 件から外れたときに
// 目印が巻き戻り、通知済みの復旧が再び飛ぶのを防ぐため。
// GitHub 側の時刻を 1 つも解析できなかった場合のみ、現在時刻を用いる。
func CheckedAt(historyIncidents HistoryIncidents, previousCheckedAt string, now time.Time) string {
	var latest time.Time
	found := false

	candidates := []string{historyIncidents.Page.UpdateAt, previousCheckedAt}
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
