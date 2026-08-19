# check_github_incidents_by_golang

[GitHub Status](https://www.githubstatus.com) の Status API から GitHub のインシデント情報を取得し、
発生中（未解決）のインシデントを通知用の JSON として出力する Go 製ツールです。

## 処理の流れ

1. データ取得元（実 API / ローカルサンプル）を判定
2. `./logs/` フォルダを作成し、`log-YYYYMMDD.log` へログを出力（標準出力にも同時出力）
3. 未解決インシデント情報を取得（`/api/v2/incidents/unresolved.json`）
4. 全インシデント情報（過去 50 件）を取得（`/api/v2/incidents.json`）
5. インシデント ID でそれぞれのデータを比較し、重複するインシデントを抽出
6. 抽出したインシデント情報を `./notice_message.json` に整形して出力

## 実行方法

```sh
go run main.go
```

もしくは、

```sh
sh shell/go_command.sh
```

利用できるオプションは `-h` で確認できます。

```sh
go run main.go -h
```

- Go 1.18 以降を想定（標準ライブラリのみ使用。外部依存パッケージはありません）
- 処理中にエラーが発生した場合は、`[ERROR]` をログに出力して終了コード `1` で異常終了します

## データ取得元の切り替え

既定では実 API（GitHub Status）から取得します。
オフラインでの動作確認用に、`./sample/` 配下のサンプルファイルへ切り替えられます。

| 取得元 | 未解決インシデント | 全インシデント |
| --- | --- | --- |
| 実 API（既定） | `https://www.githubstatus.com/api/v2/incidents/unresolved.json` | `https://www.githubstatus.com/api/v2/incidents.json` |
| ローカルサンプル | `./sample/unresolved_incidents.json` | `./sample/all_incidents.json` |

切り替えは、コマンドライン引数または環境変数で行います。
優先順位は **コマンドライン引数 > 環境変数 > 既定値（実 API）** です。

```sh
# 実 API から取得（既定）
go run main.go

# ローカルのサンプルファイルから取得
go run main.go -local

# 環境変数での指定（true/false, 1/0, yes/no などを許容）
USE_LOCAL_SAMPLE=true go run main.go

# 環境変数より引数が優先される（この場合は実 API を参照）
USE_LOCAL_SAMPLE=true go run main.go -local=false
```

環境変数に真偽値として解釈できない値を指定した場合は、警告を出して既定値（実 API）で動作します。

なお、API へのリクエストには 10 秒のタイムアウトを設定しています。

## 出力ファイル

### `notice_message.json`

未解決インシデントごとに、以下の項目を配列で出力します。

| キー | 内容 |
| --- | --- |
| `id` | インシデント ID |
| `impact` | 影響度（`none` / `minor` / `major` / `critical`） |
| `name` | インシデント名 |
| `status` | ステータス（`investigating` / `identified` / `monitoring` など） |
| `created_at` | 発生日時 |
| `updated_at` | 最終更新日時 |

出力例:

```json
[
    {
        "id": "cp306tmzcl0y",
        "impact": "critical",
        "name": "Unplanned Database Outage",
        "status": "identified",
        "created_at": "2014-05-14T14:22:39.441-06:00",
        "updated_at": "2014-05-14T14:35:21.711-06:00"
    }
]
```

未解決インシデントが 1 件も無い場合は、空配列 `[]` を出力します。

### `logs/log-YYYYMMDD.log`

実行日ごとのログファイル。処理の開始・終了、データ取得元、参照したエンドポイント、
検出したインシデント ID などを記録します（同じ内容が標準出力にも出ます）。

## ディレクトリ構成

```
.
├── main.go                        # 本体
├── notice_message.json            # 出力ファイル（実行時に上書き生成）
├── logs/                          # 日付別ログ（実行時に自動作成）
├── sample/
│   ├── all_incidents.json         # 全インシデントのサンプルレスポンス
│   └── unresolved_incidents.json  # 未解決インシデントのサンプルレスポンス
└── shell/
    └── go_command.sh              # 実行用スクリプト
```

## 既知の課題 / TODO

- 全インシデント情報から取得したコンポーネント情報（`affected_components`）は、
  通知メッセージ組み立て時に文字列化しているものの `NoticeMessage` 構造体に
  対応するフィールドがないため、`notice_message.json` には出力されません。
  加えて、参照先が `historyIncidents.Incidents[0]` 固定になっており、
  一致したインシデントのコンポーネントを参照していません。
- 通知メッセージを文字列連結で JSON 組み立てしてから `json.Unmarshal` しているため、
  インシデント名にダブルクォートが含まれる場合に解析エラーとなります。
  構造体へ直接代入する方式への変更が望ましいです。
