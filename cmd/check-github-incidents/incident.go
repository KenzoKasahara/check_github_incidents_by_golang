package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
)

const (
	GITHUB_COMMON_URL           string = "https://www.githubstatus.com"
	GITHUB_ALL_INCIDENTS        string = "/api/v2/incidents.json"
	GITHUB_UNRESOLVED_INCIDENTS string = "/api/v2/incidents/unresolved.json"

	SAMPLE_ALL_INCIDENTS        string = "./testdata/all_incidents.json"
	SAMPLE_UNRESOLVED_INCIDENTS string = "./testdata/unresolved_incidents.json"
)

// IncidentPage は Status API のレスポンスに含まれるページ情報を表す。
type IncidentPage struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	TimeZone string `json:"time_zone"`
	UpdateAt string `json:"updated_at"`
}

// IncidentUpdate はインシデントの更新履歴 1 件分を表す。
type IncidentUpdate struct {
	Body       string `json:"body"`
	CreatedAt  string `json:"created_at"`
	DisplayAt  string `json:"display_at"`
	ID         string `json:"id"`
	IncidentID string `json:"incident_id"`
	Status     string `json:"status"`
	UpdatedAt  string `json:"updated_at"`
}

// AffectedComponent はインシデントの影響を受けたコンポーネントを表す。
type AffectedComponent struct {
	Name string `json:"name"`
}

// PickComponents は影響を受けたコンポーネントとして使う方を選ぶ。
// 実 API (statuspage) が返すのは components で、affected_components は
// 更新履歴側の項目名のため、インシデント本体では null になる。
// 過去に作ったサンプルデータは affected_components 側に持っているため、両方を見る。
func PickComponents(components []AffectedComponent, affectedComponents []AffectedComponent) []AffectedComponent {
	if len(components) > 0 {
		return components
	}

	return affectedComponents
}

// UnresolvedIncident は未解決インシデント 1 件分を表す。
type UnresolvedIncident struct {
	CreatedAt          string              `json:"created_at"`
	ID                 string              `json:"id"`
	Impact             string              `json:"impact"`
	IncidentUpdates    []IncidentUpdate    `json:"incident_updates"`
	MonitoringAt       string              `json:"monitoring_at"`
	Name               string              `json:"name"`
	PageID             string              `json:"page_id"`
	ResolvedAt         string              `json:"resolved_at"`
	ShortLink          string              `json:"shortlink"`
	Status             string              `json:"status"`
	UpdatedAt          string              `json:"updated_at"`
	Components         []AffectedComponent `json:"components"`
	AffectedComponents []AffectedComponent `json:"affected_components"`
}

// ComponentNames は影響を受けたコンポーネントの名称のみを取り出す。
func (incident UnresolvedIncident) ComponentNames() []string {
	return AffectedComponentNames(PickComponents(incident.Components, incident.AffectedComponents))
}

// HistoryIncident は過去のインシデント 1 件分を表す。
type HistoryIncident struct {
	CreatedAt          string              `json:"created_at"`
	ID                 string              `json:"id"`
	Impact             string              `json:"impact"`
	IncidentUpdates    []IncidentUpdate    `json:"incident_updates"`
	MonitoringAt       string              `json:"monitoring_at"`
	Name               string              `json:"name"`
	PageID             string              `json:"page_id"`
	ResolvedAt         string              `json:"resolved_at"`
	ShortLink          string              `json:"shortlink"`
	Status             string              `json:"status"`
	UpdatedAt          string              `json:"updated_at"`
	Components         []AffectedComponent `json:"components"`
	AffectedComponents []AffectedComponent `json:"affected_components"`
}

// ComponentNames は影響を受けたコンポーネントの名称のみを取り出す。
func (incident HistoryIncident) ComponentNames() []string {
	return AffectedComponentNames(PickComponents(incident.Components, incident.AffectedComponents))
}

// UnresolvedIncidents は /api/v2/incidents/unresolved.json のレスポンスを表す。
type UnresolvedIncidents struct {
	Page      IncidentPage         `json:"page"`
	Incidents []UnresolvedIncident `json:"incidents"`
}

// HistoryIncidents は /api/v2/incidents.json のレスポンスを表す。
type HistoryIncidents struct {
	Page      IncidentPage      `json:"page"`
	Incidents []HistoryIncident `json:"incidents"`
}

// FetchJsonApi は GitHub Status API へリクエストし、レスポンスボディを返す。
func FetchJsonApi(github_api_uri string) ([]byte, error) {
	base, err := url.Parse(GITHUB_COMMON_URL)
	if err != nil {
		return nil, fmt.Errorf("ベース URL の解析に失敗しました (%v): %w", GITHUB_COMMON_URL, err)
	}
	reference, err := url.Parse(github_api_uri)
	if err != nil {
		return nil, fmt.Errorf("エンドポイントの解析に失敗しました (%v): %w", github_api_uri, err)
	}
	endpoint := base.ResolveReference(reference)

	log.Printf("%v %v\n", "[INFO]", "endpoint: "+endpoint.String())

	req, err := http.NewRequest("GET", endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("リクエストの作成に失敗しました (%v): %w", endpoint.String(), err)
	}
	req.Header.Add("Accept", "application/json")

	var client *http.Client = &http.Client{Timeout: HTTP_TIMEOUT}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("リクエストの送信に失敗しました (%v): %w", endpoint.String(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("予期しないステータスコードが返却されました (%v): %v", endpoint.String(), resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("レスポンスの読み込みに失敗しました (%v): %w", endpoint.String(), err)
	}

	return body, nil
}

// ReadLocalJson はローカルの JSON ファイルを読み込み、target にデコードする。
func ReadLocalJson(filePath string, target any) error {
	jsonFile, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("ファイルのオープンに失敗しました (%v): %w", filePath, err)
	}
	defer jsonFile.Close()

	jsonData, err := io.ReadAll(jsonFile)
	if err != nil {
		return fmt.Errorf("ファイルの読み込みに失敗しました (%v): %w", filePath, err)
	}

	if err := json.Unmarshal(jsonData, target); err != nil {
		return fmt.Errorf("JSON の解析に失敗しました (%v): %w", filePath, err)
	}

	return nil
}

// GetUnResolvedIncidents は未解決のインシデントを取得する。
func GetUnResolvedIncidents(useLocal bool) (UnresolvedIncidents, error) {
	var unresolvedIncidents UnresolvedIncidents

	// 未解決のインシデントを取得
	if useLocal {
		if err := ReadLocalJson(SAMPLE_UNRESOLVED_INCIDENTS, &unresolvedIncidents); err != nil {
			return unresolvedIncidents, err
		}
		return unresolvedIncidents, nil
	}

	unresolvedIncidentsData, err := FetchJsonApi(GITHUB_UNRESOLVED_INCIDENTS)
	if err != nil {
		return unresolvedIncidents, err
	}
	if err := json.Unmarshal(unresolvedIncidentsData, &unresolvedIncidents); err != nil {
		return unresolvedIncidents, fmt.Errorf("JSON の解析に失敗しました (%v): %w", GITHUB_UNRESOLVED_INCIDENTS, err)
	}

	if condition := len(unresolvedIncidents.Incidents); condition == 0 {
		log.Printf("%v %v", "[WARNING]", "No incidents found.")
	} else {
		for _, incident := range unresolvedIncidents.Incidents {
			log.Printf("%v %v\n", "[INFO]", "ID: "+incident.ID)
		}
	}

	return unresolvedIncidents, nil
}

// GetHistoryIncidents はすべてのインシデント(過去50件)を取得する。
func GetHistoryIncidents(useLocal bool) (HistoryIncidents, error) {
	var historyIncidents HistoryIncidents

	// すべてのインシデントを取得
	if useLocal {
		if err := ReadLocalJson(SAMPLE_ALL_INCIDENTS, &historyIncidents); err != nil {
			return historyIncidents, err
		}
		return historyIncidents, nil
	}

	historyIncidentsData, err := FetchJsonApi(GITHUB_ALL_INCIDENTS)
	if err != nil {
		return historyIncidents, err
	}
	if err := json.Unmarshal(historyIncidentsData, &historyIncidents); err != nil {
		return historyIncidents, fmt.Errorf("JSON の解析に失敗しました (%v): %w", GITHUB_ALL_INCIDENTS, err)
	}

	for cnt, incident := range historyIncidents.Incidents {
		log.Printf("%v incident: %v %v\n", "[INFO]", cnt, incident.ID)
	}

	return historyIncidents, nil
}
