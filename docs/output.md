# 出力ファイル

## notice_message.json

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

## logs/log-YYYYMMDD.log

実行日ごとのログファイル。処理の開始・終了、データ取得元、参照したエンドポイント、
検出したインシデント ID などを記録します（同じ内容が標準出力にも出ます）。

起動時に、保持期間（`LOG_RETENTION_DAYS`、既定 30 日）を過ぎたログを削除します。
対象は `log-YYYYMMDD.log` の形式に一致するファイルのみで、それ以外は削除しません。

Webhook URL はログに出力しません。送信エラー時も URL 部分を取り除いて記録します。
