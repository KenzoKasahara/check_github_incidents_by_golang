# 定期実行（Raspberry Pi / cron）

Raspberry Pi などの Linux 上で cron に登録し、定期的にインシデントを確認して通知する手順です。
先に [設定](configuration.md#env) を済ませ、`.env` に Webhook URL を書いておいてください。

登録用のスクリプトを [scripts/](../scripts/) に置いてあります。

| ファイル | 役割 |
| --- | --- |
| `scripts/run-check.sh` | cron から呼ばれる実行ラッパー |
| `scripts/install-cron.sh` | crontab への登録・変更・削除 |
| `scripts/crontab.example` | 自分で crontab を書く場合の記述例 |

## 1. バイナリをビルドする

cron から `go run` を呼ぶと毎回コンパイルが走ります。cron は環境変数がほとんど無い状態で
コマンドを実行するため、Go がキャッシュ先を見つけられず失敗することもあります。
あらかじめビルドしておきます。

Raspberry Pi 上でビルドする場合:

```sh
cd ~/check_github_incidents_by_golang
go build -o check-github-incidents ./cmd/check-github-incidents
```

Go が入っていなければ `sudo apt install -y golang-go` で導入できます。
ただし `go version` が 1.18 未満だとビルドできません。
Raspberry Pi OS の bullseye 以前は 1.15 が入るため、その場合は
[go.dev](https://go.dev/dl/) から新しいものを取得してください。

手元の PC からクロスコンパイルすることもできます。
Raspberry Pi 側で `uname -m` を実行し、`aarch64` なら 64bit、`armv7l` なら 32bit です。

```sh
# 64bit (aarch64)
GOOS=linux GOARCH=arm64 go build -o check-github-incidents ./cmd/check-github-incidents

# 32bit (armv7l)
GOOS=linux GOARCH=arm GOARM=7 go build -o check-github-incidents ./cmd/check-github-incidents
```

生成したバイナリを転送し、実行権限を付けます。

```sh
scp check-github-incidents pi@raspberrypi.local:~/check_github_incidents_by_golang/
ssh pi@raspberrypi.local chmod +x ~/check_github_incidents_by_golang/check-github-incidents
```

## 2. タイムゾーンを合わせる

ログのファイル名（`log-YYYYMMDD.log`）と各行の時刻は、システムのローカルタイムに従います。
Raspberry Pi は初期状態で UTC のことがあるので、必要なら変更しておきます。

```sh
timedatectl                                  # 現在の設定を確認
sudo timedatectl set-timezone Asia/Tokyo     # 変更する
```

## 3. cron に登録する

```sh
./scripts/install-cron.sh
```

既定は 10 分ごとです。間隔を変えたいときは cron 式を渡します。

```sh
./scripts/install-cron.sh -s '*/30 * * * *'
```

書き込まれる内容を先に見たい場合は `--dry-run` を付けてください。
登録されるのは次の 2 行です。

```cron
# check_github_incidents (managed by scripts/install-cron.sh)
*/10 * * * * /home/pi/check_github_incidents_by_golang/scripts/run-check.sh
```

1 行目の目印コメントで自分の行を見分けているので、何度実行しても行が増えません。
間隔を変えるときも同じコマンドで差し替わります。ほかの cron 設定には手を触れません。

crontab を自分で書きたい場合は [scripts/crontab.example](../scripts/crontab.example) を
参考にしてください。よく使う間隔の例と、ラッパーを使わずに直接呼ぶ書き方を載せています。

## 4. 動作を確認する

cron を待つ前に、ラッパーを手で流して通ることを確かめます。

```sh
TEE=1 ./scripts/run-check.sh -dry-run
```

引数はそのままバイナリへ渡されるので、`-dry-run` なら通知は送信されません。
`TEE=1` を付けると画面にも出力されます（付けない場合は `logs/cron.log` にだけ残ります）。

問題なければ cron の実行を待ち、ログを追います。

```sh
tail -f logs/cron.log                        # 実行の開始・終了・終了コード
tail -f logs/log-$(date +%Y%m%d).log         # 取得したインシデントと通知内容
```

cron が起動したかどうかは journal で分かります。

```sh
journalctl -u cron --since "10 min ago"
```

journal が使えない環境では `grep CRON /var/log/syslog` を見てください。

## 5. 設定を確認・変更・削除する

```sh
./scripts/install-cron.sh --show        # いま入っている設定を表示
./scripts/install-cron.sh -s '*/5 * * * *'   # 間隔を変える
./scripts/install-cron.sh --uninstall   # 登録を削除する
```

`--uninstall` は目印コメントで付けた自分の行だけを消します。

## run-check.sh を挟んでいる理由

cron から直接バイナリを呼ばずラッパーを経由するのは、対話シェルと cron では環境が違い、
そのままだと動かないことが多いためです。

まずカレントディレクトリです。cron の作業ディレクトリはホームディレクトリですが、
`.env`、`logs/`、`notice_message.json`、`notified_incidents.json` はいずれも
実行時のカレントディレクトリを基準に読み書きします。ラッパーが自分の位置から
プロジェクト直下を割り出して `cd` するので、どこから呼んでも同じ場所を読み書きします。

次に多重起動です。前回の実行が終わらないうちに次が始まると
`notified_incidents.json` の読み書きが競合し、通知が重複したり抜けたりします。
`flock` で弾いているため、間隔を詰めても二重には走りません。

最後に出力です。cron は実行のたびに標準出力をメールで送ろうとします。
MTA が入っていない Raspberry Pi ではそのエラーが syslog に溜まるので、
ラッパー側で `logs/cron.log` へ寄せています。

`logs/cron.log` は日付で分かれないため、1 MiB を超えたら `cron.1.log` へ退避します。
アプリ側の保持期間による削除（`LOG_RETENTION_DAYS`）は `log-YYYYMMDD.log` だけが対象です。

## つまずきやすいところ

### バイナリが無いと言われる

```
ERROR: /home/pi/check_github_incidents_by_golang/check-github-incidents が実行できません
```

手順 1 のビルドがまだか、置き場所が違います。別の場所に置きたい場合は
`BIN=/path/to/check-github-incidents` で指定できます。

### ログが増えない、通知も来ない

crontab の行が `scripts/run-check.sh` を絶対パスで呼んでいるか確認してください。
ラッパーを使わず自分でコマンドを書いた場合は、`cd` の書き忘れが原因のことが多いです。
`cd` が無いとホームディレクトリ側に `logs/` が作られるので、`~/logs/` を覗くと痕跡が見つかります。

### 設定が反映されない

cron は `.bashrc` などの対話シェル向け設定を読み込みません。
このツールは自分で `./.env` を読むため、`cd` さえ正しければ設定は効きます。
シェルの環境変数に頼った書き方をしている場合は `.env` へ移してください。

### 同じインシデントの通知が来ない

`notified_incidents.json` に記録済みのものは再通知されません。仕様どおりの動作です。
詳しくは [通知](notification.md#重複通知の抑止) を参照してください。

### 実行間隔をどれくらいにするか

インシデントの検知が目的なので、5〜15 分あたりが扱いやすい間隔です。
1 回の実行で Status API を 2 回参照します（タイムアウトは各 10 秒）。
