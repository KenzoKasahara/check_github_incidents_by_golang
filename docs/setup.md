# セットアップ（Raspberry Pi / Linux）

Raspberry Pi などの Linux にこのツールを配置し、手動で動くところまで持っていく手順です。
Go が入っていない状態から順に進められるようにしています。

定期実行させたい場合は、この手順を終えてから [定期実行](cron.md) へ進んでください。

## 1. Go をインストールする

この機械の上でビルドする場合に必要です。
手元の PC でクロスコンパイルしてバイナリだけ転送するなら、この手順は飛ばせます
（手順 3 で触れます）。

### apt で入れる

一番手軽な方法です。

```sh
sudo apt update
sudo apt install -y golang-go
go version
```

ただし apt で入る Go はディストリビューションに固定されており、
[go.mod](../go.mod) が要求する 1.26 に届かないことがあります。

| Raspberry Pi OS | apt の Go | このツールをビルドできるか |
| --- | --- | --- |
| bookworm 以降 | 1.19 以上 | できる |
| bullseye 以前 | 1.15 | できない（次の手順で入れ直す） |

`go version` が 1.26 未満だった場合は、下の公式アーカイブから入れてください。

### 公式アーカイブから入れる

まず CPU のアーキテクチャを確認します。

```sh
uname -m
```

`aarch64` なら 64bit（`arm64`）、`armv7l` や `armv6l` なら 32bit（`armv6l`）です。
Go は 32bit ARM 向けに `armv6l` のビルドだけを配布していますが、armv7 の環境でも動きます。

バージョンとアーキテクチャを変数に入れてから展開します。
バージョンは公式が公開している最新版の番号を取ってくるので、書き換える必要はありません。

```sh
GO_VERSION="$(curl -sL 'https://go.dev/VERSION?m=text' | head -1)"   # 例: go1.27.0
GO_ARCH=arm64          # 32bit なら armv6l

curl -LO "https://go.dev/dl/${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf "${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
```

特定のバージョンを入れたい場合は、`GO_VERSION=go1.26.0` のように直接指定してください。
配布されている版は [go.dev/dl](https://go.dev/dl/) で一覧できます。

`rm -rf /usr/local/go` を先に実行するのは、古いバージョンのファイルが混ざるのを防ぐためです。
Go の公式手順でもそうなっています。

続いて PATH を通します。

```sh
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.profile
source ~/.profile
go version
```

`go version go1.27.0 linux/arm64` のように表示されれば完了です。
apt 版が残っていて古いほうが優先される場合は、`sudo apt remove golang-go` で外してください。

展開後の `/usr/local/go` は 500MB 程度あります。SD カードの空き容量に注意してください。
ダウンロードした tar.gz は `rm "${GO_VERSION}.linux-${GO_ARCH}.tar.gz"` で消して構いません。

## 2. リポジトリを取得して設定する

```sh
cd ~
git clone https://github.com/KenzoKasahara/check_github_incidents_by_golang.git
cd check_github_incidents_by_golang
```

`git` が入っていなければ `sudo apt install -y git` で導入できます。

続いて `.env` を用意し、Webhook URL を書きます。
書き方は [設定](configuration.md#env) を参照してください。

```sh
cp .env.example .env
nano .env
```

`.env` は Git の追跡対象外なので、`git pull` で上書きされることはありません。

## 3. バイナリをビルドする

```sh
go build -o check-github-incidents ./cmd/check-github-incidents
```

手元の PC からクロスコンパイルすることもできます。この場合、設置先に Go は要りません。
アーキテクチャは手順 1 と同じく `uname -m` で確認します。

```sh
# 64bit (aarch64)
GOOS=linux GOARCH=arm64 go build -o check-github-incidents ./cmd/check-github-incidents

# 32bit (armv7l / armv6l)
GOOS=linux GOARCH=arm GOARM=6 go build -o check-github-incidents ./cmd/check-github-incidents
```

生成したバイナリを転送し、実行権限を付けます。

```sh
scp check-github-incidents pi@raspberrypi.local:~/check_github_incidents_by_golang/
ssh pi@raspberrypi.local chmod +x ~/check_github_incidents_by_golang/check-github-incidents
```

## 4. タイムゾーンを合わせる

ログのファイル名（`log-YYYYMMDD.log`）と各行の時刻は、システムのローカルタイムに従います。
Raspberry Pi は初期状態で UTC のことがあるので、必要なら変更しておきます。

```sh
timedatectl                                  # 現在の設定を確認
sudo timedatectl set-timezone Asia/Tokyo     # 変更する
```

## 5. 動かして確かめる

まずネットワークに出ずに、同梱のサンプルデータで通してみます。

```sh
./check-github-incidents -dry-run -local
```

`-dry-run` なら通知は送信されません。`-local` は [testdata/](../testdata/) のサンプルを読みます。
組み立てられた通知内容がログに出れば、ビルドと `.env` の読み込みは問題ありません。

次に実 API から取得します。まだ送信はしません。

```sh
./check-github-incidents -dry-run
```

最後に実際の通知を試します。未解決インシデントが 0 件のときは何も届かないので、
届かないこと自体は異常ではありません。

```sh
./check-github-incidents
```

生成されるファイルは [出力ファイル](output.md) にまとめています。

ここまで動いたら、[定期実行](cron.md) で cron に登録してください。
