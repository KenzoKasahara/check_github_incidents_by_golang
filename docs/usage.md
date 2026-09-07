# 実行方法

```sh
go run ./cmd/check-github-incidents
```

利用できるオプションは `-h` で確認できます。

```sh
go run ./cmd/check-github-incidents -h
```

送信内容を確認したいだけの場合は `-dry-run` を使います。通知は送信されず、
組み立てたペイロードがログに出力されるだけです。通知済みの記録も更新しません。

```sh
go run ./cmd/check-github-incidents -dry-run
```

## テスト通知

通知先の疎通や見た目を確かめたい場合は `-test-notify` を使います。
Status API は参照せず、サンプルのインシデントを新規・更新・復旧の 3 件まとめて送ります。

```sh
go run ./cmd/check-github-incidents -test-notify
```

- 記録ファイルと `notice_message.json` には触れないため、本番の記録を壊しません
- インシデント名は `[テスト通知] Example Incident` となり、本物と取り違えることはありません
- Webhook URL が未設定のときは `[WARNING]` を出して何も送りません
- `-dry-run` と併用すると、送信せずに内容だけを確認できます

```sh
go run ./cmd/check-github-incidents -test-notify -dry-run
```

## サンプルで復旧まで試す

`-local` は [testdata/](../testdata/) のサンプルを読みます。記録は実 API 用とは別の
`notified_incidents.local.json` に残るため、本番の記録を壊さずに
発生から復旧までの流れを一通り確認できます。

```sh
# 1. 発生を通知する（testdata の未解決インシデントが新規として飛ぶ）
go run ./cmd/check-github-incidents -local

# 2. testdata/unresolved_incidents.json から該当のインシデントを取り除き、
#    testdata/all_incidents.json の同じ ID を "status": "resolved" にして
#    "resolved_at" を入れる

# 3. 復旧を通知する
go run ./cmd/check-github-incidents -local

# 4. 試し終わったら記録を消す
rm notified_incidents.local.json
```

記録が更新されるのは実際に送信した実行だけです。`-dry-run` を付けたり
Webhook URL が未設定のままだと記録が残らないため、2 回目も「新規」として通知されます。

ログや出力ファイルは**実行時のカレントディレクトリ**を基準に作成されます。
上記のとおり、リポジトリのルートから実行してください。

- Go 1.26 以降を想定（標準ライブラリのみ使用。外部依存パッケージはありません）
- 処理中にエラーが発生した場合は、`[ERROR]` をログに出力して終了コード `1` で異常終了します

## テスト

```sh
go test ./...
```

インシデントの突合、通知ペイロードの組み立て、`.env` の解析、ログの削除、
通知済み状態の管理などを対象にしています。
Webhook の送信は `httptest` のローカルサーバーを使うため、外部へのアクセスは発生しません。
