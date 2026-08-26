#!/usr/bin/env bash
#
# crontab へ定期実行の設定を登録／更新／削除する。
#
# crontab は「全文を読み込んで全文を書き戻す」インターフェースしか無いので、
# 目印コメント（MARKER）で自分の行だけを見分けて差し替える。
# 何度実行しても行が増えない（= 冪等）ようにしてある。
#
#   ./scripts/install-cron.sh                       10 分ごとに実行する設定を入れる
#   ./scripts/install-cron.sh -s '*/30 * * * *'     30 分ごとに変更する
#   ./scripts/install-cron.sh --show                いま入っている設定を表示する
#   ./scripts/install-cron.sh --dry-run             書き換え後の crontab を表示するだけ
#   ./scripts/install-cron.sh --uninstall           設定を削除する
#
set -euo pipefail

SCRIPT_PATH="$(readlink -f "${BASH_SOURCE[0]}")"
PROJECT_ROOT="$(cd "$(dirname "$SCRIPT_PATH")/.." && pwd)"
RUNNER="$PROJECT_ROOT/scripts/run-check.sh"
BIN="$PROJECT_ROOT/check-github-incidents"

# この文字列を含む行が「このツールが管理している行」。書き換え・削除の目印にする
MARKER='# check_github_incidents (managed by scripts/install-cron.sh)'

# 既定は 10 分ごと。Status API は公開エンドポイントなので、
# これ以上短くしても得られる情報は増えにくい（相手側への負荷だけ増える）
SCHEDULE='*/10 * * * *'
ACTION='install'

# ファイル先頭のコメントをそのまま使い方として出す（説明を 2 か所に書かないため）。
# 1 行目の shebang は飛ばし、コメントでなくなった時点で止める
usage() {
	awk 'NR > 1 { if ($0 !~ /^#/) exit; sub(/^# ?/, ""); print }' "$SCRIPT_PATH"
	exit "${1:-0}"
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	-s | --schedule)
		[[ $# -ge 2 ]] || {
			echo "エラー: $1 には cron 式が必要です（例: '*/10 * * * *'）" >&2
			exit 2
		}
		SCHEDULE="$2"
		shift 2
		;;
	--uninstall) ACTION='uninstall'; shift ;;
	--show) ACTION='show'; shift ;;
	--dry-run) ACTION='dry-run'; shift ;;
	-h | --help) usage 0 ;;
	*)
		echo "エラー: 不明な引数: $1" >&2
		usage 2
		;;
	esac
done

command -v crontab >/dev/null 2>&1 || {
	echo "エラー: crontab コマンドが見つかりません（sudo apt install cron）" >&2
	exit 1
}

# crontab が空だと `crontab -l` は終了コード 1 を返す。エラー扱いにしない
current_crontab() {
	crontab -l 2>/dev/null || true
}

# 管理対象の行（MARKER 行と、そのすぐ下の run-check.sh を呼ぶ行）を取り除いて返す。
# 該当行しか無いと grep は「1 件も出力しなかった」で終了コード 1 を返す。
# set -o pipefail と組み合わさるとスクリプトごと止まってしまうので || true で受ける
without_managed_lines() {
	current_crontab | grep -vF -e "$MARKER" -e "$RUNNER" || true
}

if [[ "$ACTION" == 'show' ]]; then
	found="$(current_crontab | grep -F -A1 "$MARKER" || true)"
	if [[ -z "$found" ]]; then
		echo '登録されていません'
	else
		echo "$found"
	fi
	exit 0
fi

if [[ "$ACTION" == 'uninstall' ]]; then
	# `crontab -l | ... | crontab -` と一気に繋ぐと、読み出しと書き込みが同時に走って
	# crontab を空にしてしまうことがある。必ず一度変数へ受けてから書き戻す
	remaining="$(without_managed_lines)"
	before="$(current_crontab | wc -l)"
	printf '%s\n' "$remaining" | crontab -
	after="$(current_crontab | wc -l)"
	if [[ "$before" -eq "$after" ]]; then
		echo '登録されていませんでした（変更なし）'
	else
		echo "削除しました: $RUNNER"
	fi
	exit 0
fi

[[ -x "$RUNNER" ]] || {
	echo "エラー: $RUNNER が実行できません（chmod +x scripts/run-check.sh）" >&2
	exit 1
}

# バイナリは後から用意しても構わないので、ここでは止めずに知らせるだけにする
[[ -x "$BIN" ]] || {
	echo "警告: $BIN がまだありません" >&2
	echo "      go build -o check-github-incidents ./cmd/check-github-incidents" >&2
}

# cron 式は「分 時 日 月 曜日」の 5 フィールド。ここで弾かないと
# crontab - が丸ごと失敗し、既存の設定まで消えかねない
field_count="$(printf '%s\n' "$SCHEDULE" | tr -s ' ' | wc -w)"
[[ "$field_count" -eq 5 ]] || {
	echo "エラー: cron 式は 5 フィールド必要です: '$SCHEDULE'" >&2
	exit 2
}

# ここでも読み書きを同時に走らせない。先に全文を組み立ててから 1 回だけ書き戻す
new_crontab="$(
	without_managed_lines
	echo "$MARKER"
	echo "$SCHEDULE $RUNNER"
)"

if [[ "$ACTION" == 'dry-run' ]]; then
	echo '--- 書き込まれる crontab ---'
	printf '%s\n' "$new_crontab"
	exit 0
fi

printf '%s\n' "$new_crontab" | crontab -
echo "登録しました: $SCHEDULE $RUNNER"
echo "確認: crontab -l / ログ: $PROJECT_ROOT/logs/cron.log"
