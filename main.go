package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	GITHUB_COMMON_URL           string = "https://www.githubstatus.com"
	GITHUB_ALL_INCIDENTS        string = "/api/v2/incidents.json"
	GITHUB_UNRESOLVED_INCIDENTS string = "/api/v2/incidents/unresolved.json"

	SAMPLE_ALL_INCIDENTS        string = "./sample/all_incidents.json"
	SAMPLE_UNRESOLVED_INCIDENTS string = "./sample/unresolved_incidents.json"

	ENV_USE_LOCAL_SAMPLE string = "USE_LOCAL_SAMPLE"

	HTTP_TIMEOUT time.Duration = 10 * time.Second
)

type IncidentUpdate struct {
	Body       string `json:"body"`
	CreatedAt  string `json:"created_at"`
	DisplayAt  string `json:"display_at"`
	ID         string `json:"id"`
	IncidentID string `json:"incident_id"`
	Status     string `json:"status"`
	UpdatedAt  string `json:"updated_at"`
}

type UnresolvedIncidents struct {
	Page struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		URL      string `json:"url"`
		TimeZone string `json:"time_zone"`
		UpdateAt string `json:"updated_at"`
	} `json:"page"`
	Incidents []struct {
		CreatedAt       string           `json:"created_at"`
		ID              string           `json:"id"`
		Impact          string           `json:"impact"`
		IncidentUpdates []IncidentUpdate `json:"incident_updates"`
		MonitoringAt    string           `json:"monitoring_at"`
		Name            string           `json:"name"`
		PageID          string           `json:"page_id"`
		ResolvedAt      string           `json:"resolved_at"`
		ShortLink       string           `json:"shortlink"`
		Status          string           `json:"status"`
		UpdatedAt       string           `json:"updated_at"`
	}
}

type HistoryIncidents struct {
	Page struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		URL      string `json:"url"`
		TimeZone string `json:"time_zone"`
		UpdateAt string `json:"updated_at"`
	} `json:"page"`
	Incidents []struct {
		CreatedAt          string           `json:"created_at"`
		ID                 string           `json:"id"`
		Impact             string           `json:"impact"`
		IncidentUpdates    []IncidentUpdate `json:"incident_updates"`
		MonitoringAt       string           `json:"monitoring_at"`
		Name               string           `json:"name"`
		PageID             string           `json:"page_id"`
		ResolvedAt         string           `json:"resolved_at"`
		ShortLink          string           `json:"shortlink"`
		Status             string           `json:"status"`
		UpdatedAt          string           `json:"updated_at"`
		AffectedComponents []struct {
			Name string `json:"name"`
		} `json:"affected_components"`
	}
}

type NoticeMessage struct {
	IncidentID        string `json:"id"`
	IncidentImpact    string `json:"impact"`
	IncidentName      string `json:"name"`
	IncidentStatus    string `json:"status"`
	IncidentCreatedAt string `json:"created_at"`
	IncidentUpdatedAt string `json:"updated_at"`
}

func LoggingSettings(logFile string) {
	logfile, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalln("[ERROR]", "log file open error:", err)
	}

	multiLogFile := io.MultiWriter(os.Stdout, logfile)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.SetOutput(multiLogFile)
}

// UseLocalSample はローカルのサンプルファイルを使用するかどうかを判定する。
// 優先順位: コマンドライン引数 (-local) > 環境変数 (USE_LOCAL_SAMPLE) > 既定値 (false = 実 API)
func UseLocalSample() bool {
	localFlag := flag.Bool("local", false, "ローカルのサンプルファイル(./sample/*.json)を使用する")
	flag.Parse()

	// コマンドライン引数で明示的に指定された場合は、その値を優先する
	explicit := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "local" {
			explicit = true
		}
	})
	if explicit {
		return *localFlag
	}

	// 環境変数での指定 (true/false, 1/0, yes/no などを許容)
	if env, ok := os.LookupEnv(ENV_USE_LOCAL_SAMPLE); ok {
		value, err := strconv.ParseBool(env)
		if err != nil {
			log.Printf("%v %v\n", "[WARNING]", ENV_USE_LOCAL_SAMPLE+" の値が不正なため無視します: "+env)
			return false
		}
		return value
	}

	return false
}

func FeatchJsonApi(github_api_uri string) ([]byte, error) {
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

func CreateFolder(createFolderPath string) error {
	fileInfo, err := os.Lstat("./")
	if err != nil {
		return fmt.Errorf("カレントディレクトリの情報取得に失敗しました: %w", err)
	}

	fileMode := fileInfo.Mode()
	unixPerms := fileMode & os.ModePerm
	if err := os.MkdirAll(createFolderPath, unixPerms); err != nil {
		return fmt.Errorf("フォルダの作成に失敗しました (%v): %w", createFolderPath, err)
	}

	return nil
}

// GetUnResolvedIncidents は未解決のインシデントを取得する。
func GetUnResolvedIncidents(useLocal bool) (UnresolvedIncidents, error) {
	var unResolbIncidents UnresolvedIncidents

	// 未解決のインシデントを取得
	if useLocal {
		if err := ReadLocalJson(SAMPLE_UNRESOLVED_INCIDENTS, &unResolbIncidents); err != nil {
			return unResolbIncidents, err
		}
		return unResolbIncidents, nil
	}

	unResolbIncidentsData, err := FeatchJsonApi(GITHUB_UNRESOLVED_INCIDENTS)
	if err != nil {
		return unResolbIncidents, err
	}
	if err := json.Unmarshal(unResolbIncidentsData, &unResolbIncidents); err != nil {
		return unResolbIncidents, fmt.Errorf("JSON の解析に失敗しました (%v): %w", GITHUB_UNRESOLVED_INCIDENTS, err)
	}

	if condition := len(unResolbIncidents.Incidents); condition == 0 {
		log.Printf("%v %v", "[WARNING]", "No incidents found.")
	} else {
		for _, incident := range unResolbIncidents.Incidents {
			log.Printf("%v %v\n", "[INFO]", "ID: "+incident.ID)
		}
	}

	return unResolbIncidents, nil
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

	historyIncidentsData, err := FeatchJsonApi(GITHUB_ALL_INCIDENTS)
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

func main() {
	repeatedStars := strings.Repeat("*", 70)

	// データの取得元を判定 (コマンドライン引数 > 環境変数 > 既定値)
	useLocal := UseLocalSample()

	// ログフォルダ作成
	createFolderPath := "./logs/"
	if err := CreateFolder(createFolderPath); err != nil {
		log.Fatalln("[ERROR]", err)
	}

	// ログファイル名を日付で作成
	now := time.Now()
	logFileName := fmt.Sprintf("%vlog-%v.log", createFolderPath, now.Format("20060102"))
	LoggingSettings(logFileName)

	// 【処理開始】
	log.Println(repeatedStars)
	log.Printf("%v %v\n", "[INFO]", "【start process】")

	if useLocal {
		log.Printf("%v %v\n", "[INFO]", "data source: local sample files")
	} else {
		log.Printf("%v %v\n", "[INFO]", "data source: "+GITHUB_COMMON_URL)
	}

	// 未解決のインシデントを取得
	unResolbIncidents, err := GetUnResolvedIncidents(useLocal)
	if err != nil {
		log.Fatalln("[ERROR]", err)
	}

	// 過去のインシデントを取得
	historyIncidents, err := GetHistoryIncidents(useLocal)
	if err != nil {
		log.Fatalln("[ERROR]", err)
	}

	// 過去のインシデントと未解決のインシデントより、重複するインシデント情報を取得
	// 0 件でも null ではなく空配列 ([]) として出力されるように初期化する
	noticeMessage := []NoticeMessage{}

	for _, historyIncident := range historyIncidents.Incidents {
		for _, unResolbIncident := range unResolbIncidents.Incidents {
			if historyIncident.ID == unResolbIncident.ID {
				jsonData := `{"id": "` + unResolbIncident.ID + `",` +
					`"name": "` + unResolbIncident.Name + `",` +
					`"impact": "` + unResolbIncident.Impact + `",` +
					`"status": "` + unResolbIncident.Status + `",` +
					`"components": "` + fmt.Sprint(historyIncidents.Incidents[0].AffectedComponents) + `",` +
					`"created_at": "` + unResolbIncident.CreatedAt + `",` +
					`"updated_at": "` + unResolbIncident.UpdatedAt + `"` +
					`}`

				var message NoticeMessage
				if err := json.Unmarshal([]byte(jsonData), &message); err != nil {
					log.Fatalln("[ERROR]", fmt.Errorf("通知メッセージの JSON 解析に失敗しました (id: %v): %w", unResolbIncident.ID, err))
				}

				// noticeMessageスライスに追加
				noticeMessage = append(noticeMessage, message)
			}
		}
	}

	// noticeMessageスライスをJSONにエンコード
	indentJsonData, err := json.MarshalIndent(noticeMessage, "", "    ")
	if err != nil {
		log.Fatalln("[ERROR]", fmt.Errorf("通知メッセージの JSON エンコードに失敗しました: %w", err))
	}

	// file作成
	noticeFilePath := "./notice_message.json"
	file, err := os.Create(noticeFilePath)
	if err != nil {
		log.Fatalln("[ERROR]", fmt.Errorf("ファイルの作成に失敗しました (%v): %w", noticeFilePath, err))
	}
	defer file.Close()

	// JSONデータをファイルに書き込む
	if _, err := file.Write(indentJsonData); err != nil {
		log.Fatalln("[ERROR]", fmt.Errorf("ファイルの書き込みに失敗しました (%v): %w", noticeFilePath, err))
	}
	log.Printf("%v %v\n", "[INFO]", "メッセージをファイルに書き込みました。")

	// 【処理終了】
	log.Printf("%v %v\n", "[INFO]", "【end process】")
	log.Println(repeatedStars)
}
