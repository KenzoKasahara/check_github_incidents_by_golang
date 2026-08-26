# check_github_incidents_by_golang

[GitHub Status](https://www.githubstatus.com) の Status API から GitHub のインシデント情報を取得し、
発生中（未解決）のインシデントを Discord / Slack へ通知する Go 製ツールです。
通知内容は `./notice_message.json` にも出力します。

## 処理の流れ

1. `./logs/` フォルダを作成し、`log-YYYYMMDD.log` へログを出力（標準出力にも同時出力）
2. `.env` を読み込み、環境変数として設定
3. 保持期間を過ぎたログファイルを削除
4. データ取得元（実 API / ローカルサンプル）を判定
5. 未解決インシデント情報を取得（`/api/v2/incidents/unresolved.json`）
6. 全インシデント情報（過去 50 件）を取得（`/api/v2/incidents.json`）
7. インシデント ID でそれぞれのデータを比較し、重複するインシデントを抽出
8. 抽出したインシデント情報を `./notice_message.json` に整形して出力
9. 前回通知した内容と突合し、未通知・更新されたインシデントのみ Discord / Slack へ通知

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

送信内容を確認したいだけの場合は `-dry-run` を使います。通知は送信されず、
組み立てたペイロードがログに出力されるだけです。通知済みの記録も更新しません。

```sh
go run . -dry-run
```

- Go 1.18 以降を想定（標準ライブラリのみ使用。外部依存パッケージはありません）
- 処理中にエラーが発生した場合は、`[ERROR]` をログに出力して終了コード `1` で異常終了します

## 設定（.env）

Webhook URL などの設定は、リポジトリ直下の `.env` に記述します。
`.env.example` をコピーして使ってください。`.env` は Git の追跡対象外です。

```sh
cp .env.example .env
```

| キー | 内容 | 既定値 |
| --- | --- | --- |
| `DISCORD_WEBHOOK_URL` | Discord の Incoming Webhook URL。未設定なら Discord へ通知しない | 未設定 |
| `SLACK_WEBHOOK_URL` | Slack の Incoming Webhook URL。未設定なら Slack へ通知しない | 未設定 |
| `LOG_RETENTION_DAYS` | ログの保持日数（当日を含む）。`0` で削除しない | `30` |
| `USE_LOCAL_SAMPLE` | ローカルサンプルを使用するか | `false` |

優先順位は **コマンドライン引数 > `.env` > シェルの環境変数 > 既定値** です。

`.env` の値はシェルの環境変数を**上書きします**。シェルに残った古い設定が意図せず
使われるのを防ぐためで、`godotenv` などの一般的な挙動とは逆になっています。
一時的に切り替えたい場合は、`.env` に書かずコマンドライン引数を使ってください。

対応する記法は `KEY=VALUE`、`export KEY=VALUE`、`#` で始まるコメント行、空行です。
値全体を `"` または `'` で囲んだ場合はクォートを取り除きます。
行の途中の `#` はコメントとして扱いません（URL に含まれる場合があるため）。

`.env` が存在しない場合はエラーとせず、シェルの環境変数と既定値で動作します。

## データ取得元の切り替え

既定では実 API（GitHub Status）から取得します。
オフラインでの動作確認用に、`./testdata/` 配下のサンプルファイルへ切り替えられます。

| 取得元 | 未解決インシデント | 全インシデント |
| --- | --- | --- |
| 実 API（既定） | `https://www.githubstatus.com/api/v2/incidents/unresolved.json` | `https://www.githubstatus.com/api/v2/incidents.json` |
| ローカルサンプル | `./testdata/unresolved_incidents.json` | `./testdata/all_incidents.json` |

切り替えは、コマンドライン引数または環境変数で行います。

```sh
# 実 API から取得（既定）
go run .

# ローカルのサンプルファイルから取得
go run . -local

# 環境変数での指定（true/false, 1/0, yes/no などを許容）
# ※ .env に USE_LOCAL_SAMPLE を書いている場合は .env が優先されます
USE_LOCAL_SAMPLE=true go run .

# 環境変数より引数が優先される（この場合は実 API を参照）
USE_LOCAL_SAMPLE=true go run . -local=false
```

真偽値として解釈できない値を指定した場合は、警告を出して既定値（実 API）で動作します。

なお、API へのリクエストには 10 秒のタイムアウトを設定しています。

## 通知

`.env` に Webhook URL を設定すると、未解決インシデントを Discord / Slack へ通知します。
どちらも Incoming Webhook を使うため、追加のパッケージやトークンは不要です。

- `impact` に応じて色分けします（`critical` = 赤、`major` = 橙、`minor` = 黄）
- インシデント名から `shortlink` へリンクします
- 1 回の通知に含めるのは 10 件までです（Discord の embeds 上限に合わせています）。
  超過分は見出しに「他 N 件」と表示します
- 未解決インシデントが 0 件のときは通知しません
- 片方の送信に失敗しても、もう片方の送信は継続します

### 重複通知の抑止

定期実行しても同じインシデントを繰り返し通知しないよう、通知済みのインシデントを
`./notified_incidents.json` に記録しています。

通知するのは、**未記録のインシデント**と、**記録時から `updated_at` が変化したインシデント**だけです。
インシデントの状況が更新されたときには改めて通知が届きます。

解決済みとなり未解決一覧から消えたインシデントの記録は引き継がないため、
ファイルが際限なく増えることはありません。
すべて通知し直したい場合は、このファイルを削除してください。

なお、送信に失敗した場合は記録を更新しないため、次回の実行で再送されます。

## 出力ファイル

### `notice_message.json`

未解決インシデントごとに、以下の項目を配列で出力します。

| キー | 内容 |
| --- | --- |
| `id` | インシデント ID |
| `impact` | 影響度（`none` / `minor` / `major` / `critical`） |
| `name` | インシデント名 |
| `status` | ステータス（`investigating` / `identified` / `monitoring` など） |
| `shortlink` | インシデントの詳細ページ URL |
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
        "shortlink": "http://stspg.co:5000/Q0E",
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

起動時に、保持期間（`LOG_RETENTION_DAYS`、既定 30 日）を過ぎたログを削除します。
対象は `log-YYYYMMDD.log` の形式に一致するファイルのみで、それ以外は削除しません。

Webhook URL はログに出力しません。送信エラー時も URL 部分を取り除いて記録します。

## ディレクトリ構成

```
.
├── .gitignore
├── .env.example                   # .env のひな形（.env は追跡対象外）
├── go.mod
├── main.go                        # 起動と全体の流れ
├── config.go                      # 定数・コマンドライン引数・設定値の判定
├── dotenv.go                      # .env の読み込み
├── logger.go                      # ログ設定・フォルダ作成・古いログの削除
├── incident.go                    # Status API の型定義・取得処理
├── notice.go                      # インシデントの突合・通知メッセージの出力
├── notify.go                      # 通知の共通処理（Webhook 送信 / dry-run）
├── discord.go                     # Discord 用ペイロード
├── slack.go                       # Slack 用ペイロード
├── state.go                       # 通知済みインシデントの記録
├── *_test.go                      # 各処理のテスト
├── notice_message.json            # 出力ファイル（実行時に上書き生成）
├── notified_incidents.json        # 通知済みの記録（実行時に生成 / 追跡対象外）
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

インシデントの突合、通知ペイロードの組み立て、`.env` の解析、ログの削除、
通知済み状態の管理などを対象にしています。
Webhook の送信は `httptest` のローカルサーバーを使うため、外部へのアクセスは発生しません。
