# 処理の流れとディレクトリ構成

## 処理の流れ

1. `./logs/` フォルダを作成し、`log-YYYYMMDD.log` へログを出力（標準出力にも同時出力）
2. `.env` を読み込み、環境変数として設定
3. 保持期間を過ぎたログファイルを削除
4. データ取得元（実 API / ローカルサンプル）を判定
5. 未解決インシデント情報を取得（`/api/v2/incidents/unresolved.json`）
6. 全インシデント情報（過去 50 件）を取得（`/api/v2/incidents.json`）
7. 未解決インシデント（＝発生中）を抽出し、影響を受けたコンポーネントを全インシデント情報から補完
8. 抽出したインシデント情報を `./notice_message.json` に整形して出力
9. 全インシデント情報から解決済み（`resolved` / `postmortem`）のものを抽出
10. 前回通知した内容と突合し、新規・更新・復旧したインシデントのみ Discord / Slack へ通知

## ディレクトリ構成

```text
.
├── .gitignore
├── .env.example                   # .env のひな形（.env は追跡対象外）
├── go.mod
├── cmd/
│   └── check-github-incidents/    # フォルダ名がビルドされるバイナリ名になる
│       ├── main.go                # 起動と全体の流れ
│       ├── config.go              # コマンドライン引数・設定値の判定
│       ├── dotenv.go              # .env の読み込み
│       ├── logger.go              # ログ設定・フォルダ作成・古いログの削除
│       ├── incident.go            # Status API の型定義・取得処理
│       ├── notice.go              # インシデントの突合・通知メッセージの出力
│       ├── notify.go              # 通知の共通処理（Webhook 送信 / dry-run）
│       ├── testnotify.go          # テスト通知（-test-notify）のサンプルと送信
│       ├── discord.go             # Discord 用ペイロード
│       ├── slack.go               # Slack 用ペイロード
│       ├── state.go               # 通知済みインシデントの記録・前回との差分検知
│       └── *_test.go              # 各処理のテスト
├── docs/                          # ドキュメント（README.md が目次）
├── scripts/                       # 定期実行まわり（詳細は docs/cron.md）
│   ├── run-check.sh               # cron から呼ばれる実行ラッパー
│   ├── install-cron.sh            # crontab への登録・変更・削除
│   └── crontab.example            # crontab の記述例
├── check-github-incidents         # ビルド生成物（追跡対象外）
├── notice_message.json            # 出力ファイル（実行時に上書き生成）
├── notified_incidents.json        # 通知済みの記録（実行時に生成 / 追跡対象外）
├── notified_incidents.local.json  # -local 用の通知済みの記録（同上）
├── logs/                          # 日付別ログと cron のログ（実行時に自動作成）
└── testdata/                      # サンプルレスポンス（go のビルド対象外）
    ├── all_incidents.json         # 全インシデント
    └── unresolved_incidents.json  # 未解決インシデント
```
