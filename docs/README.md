# ドキュメント目次

`check_github_incidents_by_golang` の詳細ドキュメントです。
ツール全体の概要は [リポジトリルートの README](../README.md) を参照してください。

| ドキュメント | 内容 |
| --- | --- |
| [実行方法とテスト](usage.md) | 実行コマンド、オプション、dry-run、テスト |
| [設定](configuration.md) | `.env` の書き方、設定値の優先順位、データ取得元の切り替え |
| [通知](notification.md) | Discord / Slack への通知内容、重複通知の抑止 |
| [出力ファイル](output.md) | `notice_message.json` とログファイルの仕様 |
| [処理の流れとディレクトリ構成](architecture.md) | 実行時の処理手順、ファイル構成 |

## 目的から探す

- **とりあえず動かしたい** → [実行方法とテスト](usage.md)
- **Discord / Slack へ通知したい** → [設定](configuration.md#env) で Webhook URL を設定し、[通知](notification.md) を確認
- **通知が届かない / 何度も届く** → [通知](notification.md#重複通知の抑止)
- **通知せずに内容だけ確認したい** → [実行方法とテスト](usage.md)（`-dry-run`）
- **オフラインで動作確認したい** → [設定](configuration.md#データ取得元の切り替え)
- **ログを残す期間を変えたい** → [設定](configuration.md#env)（`LOG_RETENTION_DAYS`）
- **出力される JSON の項目を知りたい** → [出力ファイル](output.md#notice_messagejson)
- **コードを読みたい / 直したい** → [処理の流れとディレクトリ構成](architecture.md)
