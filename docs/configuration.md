# 設定

## .env

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
go run ./cmd/check-github-incidents

# ローカルのサンプルファイルから取得
go run ./cmd/check-github-incidents -local

# 環境変数での指定（true/false, 1/0, yes/no などを許容）
# ※ .env に USE_LOCAL_SAMPLE を書いている場合は .env が優先されます
USE_LOCAL_SAMPLE=true go run ./cmd/check-github-incidents

# 環境変数より引数が優先される（この場合は実 API を参照）
USE_LOCAL_SAMPLE=true go run ./cmd/check-github-incidents -local=false
```

真偽値として解釈できない値を指定した場合は、警告を出して既定値（実 API）で動作します。

サンプルを参照している間の記録は `notified_incidents.local.json` に残し、
実 API 用の `notified_incidents.json` は更新しません。
サンプルの日付は実 API とかけ離れているため、同じファイルに混ぜると
次の実 API 実行で過去のインシデントが一斉に復旧として通知されてしまうためです。

記録を分けているので、サンプルを書き換えれば発生から復旧までの流れを一通り試せます。
手順は [実行方法とテスト](usage.md#サンプルで復旧まで試す) を参照してください。

なお、API へのリクエストには 10 秒のタイムアウトを設定しています。
