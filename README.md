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
go run .
```

もしくは、

```sh
sh shell/go_command.sh
```

利用できるオプションは `-h` で確認できます。

```sh
go run . -h
```

- Go 1.18 以降を想定（標準ライブラリのみ使用。外部依存パッケージはありません）
- 処理中にエラーが発生した場合は、`[ERROR]` をログに出力して終了コード `1` で異常終了します

## データ取得元の切り替え

既定では実 API（GitHub Status）から取得します。
オフラインでの動作確認用に、`./testdata/` 配下のサンプルファイルへ切り替えられます。

| 取得元 | 未解決インシデント | 全インシデント |
| --- | --- | --- |
| 実 API（既定） | `https://www.githubstatus.com/api/v2/incidents/unresolved.json` | `https://www.githubstatus.com/api/v2/incidents.json` |
| ローカルサンプル | `./testdata/unresolved_incidents.json` | `./testdata/all_incidents.json` |

切り替えは、コマンドライン引数または環境変数で行います。
優先順位は **コマンドライン引数 > 環境変数 > 既定値（実 API）** です。

```sh
# 実 API から取得（既定）
go run .

# ローカルのサンプルファイルから取得
go run . -local

# 環境変数での指定（true/false, 1/0, yes/no などを許容）
USE_LOCAL_SAMPLE=true go run .

# 環境変数より引数が優先される（この場合は実 API を参照）
USE_LOCAL_SAMPLE=true go run . -local=false
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
| `components` | 影響を受けたコンポーネント名の配列 |
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
        "components": [
            "Pages",
            "Actions"
        ],
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
├── .gitignore
├── go.mod
├── main.go                        # 起動と全体の流れ
├── config.go                      # 定数・データ取得元の判定（引数 / 環境変数）
├── logger.go                      # ログ設定・フォルダ作成
├── incident.go                    # Status API の型定義・取得処理
├── notice.go                      # インシデントの突合・通知メッセージの出力
├── notice_test.go                 # 突合処理と取得元判定のテスト
├── notice_message.json            # 出力ファイル（実行時に上書き生成）
├── logs/                          # 日付別ログ（実行時に自動作成）
├── testdata/                      # サンプルレスポンス（go のビルド対象外）
│   ├── all_incidents.json         # 全インシデント
│   └── unresolved_incidents.json  # 未解決インシデント
└── shell/
    └── go_command.sh              # 実行用スクリプト
```

## テスト

```sh
go test ./...
```

インシデントの突合処理（`BuildNoticeMessages`）とデータ取得元の判定（`UseLocalSample`）を
対象にしています。ネットワークアクセスは行いません。
