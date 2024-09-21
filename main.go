package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	GITHUB_COMMON_URL           string = "https://www.githubstatus.com"
	GITHUB_ALL_INCIDENTS        string = "/api/v2/incidents.json"
	GITHUB_UNRESOLVED_INCIDENTS string = "/api/v2/incidents/unresolved.json"
)

type UnresolvedIncidents struct {
	Page struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		URL      string `json:"url"`
		TimeZone string `json:"time_zone"`
		UpdateAt string `json:"updated_at"`
	} `json:"page"`
	Incidents []struct {
		CreatedAt       string   `json:"created_at"`
		ID              string   `json:"id"`
		Impact          string   `json:"impact"`
		IncidentUpdates []string `json:"incident_updates"`
		MonitoringAt    string   `json:"monitoring_at"`
		Name            string   `json:"name"`
		PageID          string   `json:"page_id"`
		ResolvedAt      bool     `json:"resolved"`
		ShortLink       string   `json:"shortlink"`
		Status          string   `json:"status"`
		UpdatedAt       string   `json:"updated_at"`
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
		CreatedAt          string   `json:"created_at"`
		ID                 string   `json:"id"`
		Impact             string   `json:"impact"`
		IncidentUpdates    []string `json:"incident_updates"`
		MonitoringAt       string   `json:"monitoring_at"`
		Name               string   `json:"name"`
		PageID             string   `json:"page_id"`
		ResolvedAt         bool     `json:"resolved"`
		ShortLink          string   `json:"shortlink"`
		Status             string   `json:"status"`
		UpdatedAt          string   `json:"updated_at"`
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
	multiLogFile := io.MultiWriter(os.Stdout, logfile)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.SetOutput(multiLogFile)
	if err != nil {
		log.Fatalln("[ERROR]", "log file open error:", err)
	}
}

func FeatchJsonApi(github_api_uri string) []byte {
	base, _ := url.Parse(GITHUB_COMMON_URL)
	reference, _ := url.Parse(github_api_uri)
	endpoint := base.ResolveReference(reference)

	log.Printf("%v %v\n", "[INFO]", "endpoint: "+endpoint.String())

	req, _ := http.NewRequest("GET", endpoint.String(), nil)
	req.Header.Add("Accept", "application/json")

	var client *http.Client = &http.Client{}
	resp, _ := client.Do(req)
	body, _ := io.ReadAll(resp.Body)
	return body
}

func CreateFolder(createFolderPath string) {
	fileInfo, err := os.Lstat("./")
	if err != nil {
		log.Fatal(err)
	}

	fileMode := fileInfo.Mode()
	unixPerms := fileMode & os.ModePerm
	if err := os.MkdirAll(createFolderPath, unixPerms); err != nil {
		log.Fatal(err)
	}
}

func GetUnResolvedIncidents() UnresolvedIncidents {
	localFlag := true // ローカルファイルを使用するかどうか

	// 未解決のインシデントを取得
	if localFlag {
		jsonFile, err := os.Open("./sample/unresolved_incidents.json")
		if err != nil {
			log.Fatal(err)
		}
		defer jsonFile.Close()

		var unResolbIncidents UnresolvedIncidents
		jsonData, err := io.ReadAll(jsonFile)
		if err != nil {
			log.Fatal(err)
		}
		json.Unmarshal(jsonData, &unResolbIncidents)

		return unResolbIncidents
	} else {
		var unResolbIncidents UnresolvedIncidents
		unResolbIncidentsData := FeatchJsonApi(GITHUB_UNRESOLVED_INCIDENTS)
		json.Unmarshal(unResolbIncidentsData, &unResolbIncidents)

		if condition := len(unResolbIncidents.Incidents); condition == 0 {
			log.Printf("%v %v", "[WARNING]", "No incidents found.")
		} else {
			for _, incident := range unResolbIncidents.Incidents {
				log.Printf("%v %v\n", "[INFO]", "ID: "+incident.ID)
			}
		}
		return unResolbIncidents
	}
}

func GetHistoryIncidents() HistoryIncidents {
	localFlag := true // ローカルファイルを使用するかどうか

	// すべてのインシデントを取得
	if localFlag {
		jsonFile, err := os.Open("./sample/all_incidents.json")
		if err != nil {
			log.Fatal(err)
		}
		defer jsonFile.Close()

		var historyIncidents HistoryIncidents
		jsonData, err := io.ReadAll(jsonFile)
		if err != nil {
			log.Fatal(err)
		}
		json.Unmarshal(jsonData, &historyIncidents)

		return historyIncidents
	} else {
		var historyIncidents HistoryIncidents
		historyIncidentsData := FeatchJsonApi(GITHUB_ALL_INCIDENTS)
		json.Unmarshal(historyIncidentsData, &historyIncidents)
		for cnt, incident := range historyIncidents.Incidents {
			fmt.Printf("incident: %v %v\n", cnt, incident.ID)
		}
		return historyIncidents
	}
}

func main() {
	repeatedStars := strings.Repeat("*", 70)

	// ログフォルダ作成
	createFolderPath := "./logs/"
	CreateFolder(createFolderPath)

	// ログファイル名を日付で作成
	time := time.Now()
	logFileName := fmt.Sprintf("%vlog-%v.log", createFolderPath, time.Format("20060102"))
	LoggingSettings(logFileName)

	// 【処理開始】
	log.Println(repeatedStars)
	log.Printf("%v %v\n", "[INFO]", "【start process】")

	// 未解決のインシデントを取得
	unResolbIncidents := GetUnResolvedIncidents()

	// 過去のインシデントを取得
	historyIncidents := GetHistoryIncidents()

	// 過去のインシデントと未解決のインシデントより、重複するインシデント情報を取得
	var noticeMessage []NoticeMessage
	fmt.Printf("noticeMessage: %T\n", noticeMessage)

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
					fmt.Println("エラー: ", err)
					return
				}

				// noticeMessageスライスに追加
				noticeMessage = append(noticeMessage, message)
			}
		}
	}

	// noticeMessageスライスをJSONにエンコード
	indentJsonData, err := json.MarshalIndent(noticeMessage, "", "    ")
	if err != nil {
		log.Fatal("エラー: ", err)
	}

	// file作成
	file, err := os.Create("./notice_message.json")
	if err != nil {
		log.Fatal("エラー", err)
	}
	defer file.Close()

	// JSONデータをファイルに書き込む
	_, err = file.Write(indentJsonData)
	if err != nil {
		log.Fatal("エラー", err)
	}
	log.Printf("%v %v\n", "[INFO]", "メッセージをファイルに書き込みました。")

	// 【処理終了】
	log.Printf("%v %v\n", "[INFO]", "【end process】")
	log.Println(repeatedStars)
}
