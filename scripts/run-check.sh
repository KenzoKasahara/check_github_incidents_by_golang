#!/usr/bin/env bash
#
# cron から呼ばれる実行ラッパー。
#
# cron に check-github-incidents を直接書かずラッパーを挟むのは、
# 対話シェルと cron では環境が違い、そのままだと動かないことが多いため。
#
#   1. カレントディレクトリが $HOME になる  → cd してから実行する
#   2. PATH が /usr/bin:/bin だけになる     → バイナリを絶対パスで指す
#   3. 実行が重なることがある               → flock で多重起動を防ぐ
#   4. 標準出力がメールで飛ぶ               → logs/ に寄せてメールを止める
#
# 手動での動作確認:
#   ./scripts/run-check.sh
#   ./scripts/run-check.sh -dry-run     引数はそのままバイナリへ渡される
#   TEE=1 ./scripts/run-check.sh        画面にも出力する
#
set -euo pipefail

# --- プロジェクト直下へ移動 -------------------------------------------------
# .env / logs / notice_message.json / notified_incidents.json はいずれも
# カレントディレクトリ基準で読み書きするので、ここを外すと何も動かない。
# シンボリックリンク経由で呼ばれても正しい場所を指すよう、実体のパスを解決する
SCRIPT_PATH="$(readlink -f "${BASH_SOURCE[0]}")"
PROJECT_ROOT="$(cd "$(dirname "$SCRIPT_PATH")/.." && pwd)"
cd "$PROJECT_ROOT"

LOG_DIR="$PROJECT_ROOT/logs"
CRON_LOG="$LOG_DIR/cron.log"
LOCK_FILE="$LOG_DIR/run-check.lock"
mkdir -p "$LOG_DIR"

# 時刻はローカルタイム。アプリ側のログ (logs/log-YYYYMMDD.log) と揃えるため
log() {
	printf '[%s] [run-check] %s\n' "$(date +%Y-%m-%dT%H:%M:%S%z)" "$1"
}

# --- cron.log の切り詰め ----------------------------------------------------
# アプリ側の保持期間の削除は log-YYYYMMDD.log だけが対象なので、cron.log は
# 放っておくと伸び続ける。大きくなったら 1 世代だけ残して切り替える。
# 退避先を cron.1.log にしているのは .gitignore の /logs/*.log に含めるため
MAX_LOG_BYTES="${MAX_LOG_BYTES:-1048576}" # 1 MiB
rotate_cron_log() {
	[[ -f "$CRON_LOG" ]] || return 0
	local size
	size="$(wc -c <"$CRON_LOG")"
	if [[ "$size" -ge "$MAX_LOG_BYTES" ]]; then
		mv -f "$CRON_LOG" "$LOG_DIR/cron.1.log"
	fi
}
rotate_cron_log

# --- 実行するバイナリ -------------------------------------------------------
# cron の PATH は最小限なので、PATH に頼らず絶対パスで指す。
# 置き場所を変えたい場合は BIN=/path/to/check-github-incidents で上書きできる
BIN="${BIN:-$PROJECT_ROOT/check-github-incidents}"

main() {
	if [[ ! -x "$BIN" ]]; then
		log "ERROR: $BIN が実行できません"
		log "先に 'go build -o check-github-incidents ./cmd/check-github-incidents' を実行してください"
		return 1
	fi

	log "開始: $BIN $*"
	local status=0
	if [[ "${TEE:-0}" == "1" ]]; then
		"$BIN" "$@" || status=$?
	else
		# 標準出力の内容は logs/log-YYYYMMDD.log にも同じものが残るので捨てる。
		# 起動直後の失敗など、ログ設定前のエラーは標準エラー出力から cron.log に入る
		"$BIN" "$@" >/dev/null || status=$?
	fi
	log "終了: exit=$status"
	return "$status"
}

# --- 多重起動の防止 ---------------------------------------------------------
# 前回の実行が終わらないうちに次が始まると、notified_incidents.json の
# 読み書きが競合して通知が重複・欠落する。flock があれば必ず使う。
# -n: 待たずに即座に諦める（cron は次の回で再挑戦すればよい）
if command -v flock >/dev/null 2>&1; then
	exec 9>"$LOCK_FILE"
	if ! flock -n 9; then
		log 'SKIP: 前回の実行がまだ終わっていません' >>"$CRON_LOG"
		exit 0
	fi
fi

if [[ "${TEE:-0}" == "1" ]]; then
	main "$@" 2>&1 | tee -a "$CRON_LOG"
	exit "${PIPESTATUS[0]}"
fi
main "$@" >>"$CRON_LOG" 2>&1
