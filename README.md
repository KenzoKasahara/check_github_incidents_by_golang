# check_github_incidents_by_golang

[GitHub Status](https://www.githubstatus.com) の Status API から GitHub のインシデント情報を取得し、
発生中（未解決）のインシデントを Discord / Slack へ通知する Go 製ツールです。
通知内容は `./notice_message.json` にも出力します。

- 標準ライブラリのみで動作します（外部依存パッケージなし。Go 1.18 以降を想定）
- 通知先は Incoming Webhook のため、追加のトークンは不要です
- 通知済みのインシデントを記録するので、定期実行しても同じ内容は届きません
- オフラインでも確認できるよう、ローカルのサンプルファイルへ切り替えられます

```sh
go run ./cmd/check-github-incidents
```

## ドキュメント

詳細は [docs/](docs/README.md) にまとめています。

| ドキュメント | 内容 |
| --- | --- |
| [実行方法とテスト](docs/usage.md) | 実行コマンド、オプション、dry-run、テスト |
| [設定](docs/configuration.md) | `.env` の書き方、設定値の優先順位、データ取得元の切り替え |
| [通知](docs/notification.md) | Discord / Slack への通知内容、重複通知の抑止 |
| [出力ファイル](docs/output.md) | `notice_message.json` とログファイルの仕様 |
| [定期実行](docs/cron.md) | Raspberry Pi などで cron に登録して定期実行する手順 |
| [処理の流れとディレクトリ構成](docs/architecture.md) | 実行時の処理手順、ファイル構成 |
