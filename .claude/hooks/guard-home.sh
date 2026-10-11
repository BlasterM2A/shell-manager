#!/usr/bin/env bash
# PreToolUse(Bash): protect the real $HOME from `shm setup` / `shm update`
# (they install packages and chsh; see CLAUDE.md). Reads the hook JSON on stdin,
# exits 2 (block) or 0 (allow).
#
# Rule. The command line is tokenized quote-aware and split into segments on
# newlines, `;`, `&`, `&&`, `|`, `||` and parentheses. In every segment, after
# skipping leading VAR=value assignments and wrappers (env, sudo, time, ...), the
# command word must be an invocation of shm:
#   shm | shell-manager | <any path>/shm | <any path>/shell-manager   (e.g. ./bin/shell-manager)
#   go run .|./...|*.go          task run --
# and then `setup` or `update` must appear among its arguments. Such a segment is
# blocked unless it carries its OWN `HOME=<temp dir>` assignment (an earlier
# segment does not count: `HOME=/tmp/x true; shm setup` is blocked). A temp dir
# is: /tmp/<x>, /var/tmp/<x>, $TMPDIR/<x>, ${TMPDIR}/<x>, $(mktemp ...) or
# `mktemp ...`, with no `..`. Anything else (`$H`, `$HOME`, a bare /tmp) is not
# trusted: inline the mktemp instead.
# `self-update` is blocked unconditionally: it swaps the running executable
# and never looks at $HOME, so a temp HOME does not make it safe.
# Text that merely mentions the commands (`grep 'shm update' README.md`,
# `echo shm setup`) is not an invocation and passes; so does the body of a
# heredoc (only $(...) / `...` in an unquoted one are checked, see
# strip_heredocs). `bash -c STR`, `eval STR`
# and $(...) / `...` substitutions are checked recursively.
# Best effort, not a sandbox: it stops accidents, not a determined bypass.

SEP=$'\x1f'
TOK=()

# is_tmp_home VALUE: succeed if VALUE is a temp-dir path (see the rule above).
is_tmp_home() {
	local v=$1
	[[ $v == *..* ]] && return 1
	[[ $v =~ ^(/tmp|/var/tmp|\$TMPDIR|\$\{TMPDIR\})/[^/] ]] && return 0
	[[ $v =~ ^(\$\(mktemp|\`mktemp)([[:space:]\)\`]|$) ]] && return 0
	return 1
}

# tokenize STRING: fill TOK with shell words (quotes removed) and SEP markers.
tokenize() {
	local s=$1 i c q="" w="" have=0 depth=0 bt=0
	TOK=()
	local n=${#s}
	for ((i = 0; i < n; i++)); do
		c=${s:i:1}
		if [[ $q == "'" ]]; then
			[[ $c == "'" ]] && q="" || w+=$c
			continue
		fi
		if ((depth > 0)); then # inside an unquoted $( ... )
			w+=$c
			[[ $c == "(" ]] && depth=$((depth + 1))
			[[ $c == ")" ]] && depth=$((depth - 1))
			continue
		fi
		if ((bt)); then
			w+=$c
			[[ $c == '`' ]] && bt=0
			continue
		fi
		if [[ $c == '\' ]]; then
			w+=${s:i+1:1}
			have=1
			i=$((i + 1))
			continue
		fi
		if [[ $c == '$' && ${s:i+1:1} == "(" ]]; then
			w+='$('
			have=1
			depth=1
			i=$((i + 1))
			continue
		fi
		if [[ $c == '`' ]]; then
			w+=$c
			have=1
			bt=1
			continue
		fi
		if [[ $q == '"' ]]; then
			[[ $c == '"' ]] && q="" || w+=$c
			continue
		fi
		case $c in
		"'" | '"')
			q=$c
			have=1
			;;
		'#')
			if ((have)); then
				w+=$c
			else # comment: skip to end of line
				while ((i < n)) && [[ ${s:i:1} != $'\n' ]]; do i=$((i + 1)); done
				i=$((i - 1))
			fi
			;;
		';' | '&' | '|' | $'\n' | '(' | ')')
			((have)) && TOK+=("$w")
			w=""
			have=0
			if ((${#TOK[@]} > 0)) && [[ ${TOK[${#TOK[@]} - 1]} != "$SEP" ]]; then TOK+=("$SEP"); fi
			;;
		' ' | $'\t')
			((have)) && TOK+=("$w")
			w=""
			have=0
			;;
		*)
			w+=$c
			have=1
			;;
		esac
	done
	((have)) && TOK+=("$w")
	return 0
}

# strip_heredocs STRING: print STRING with heredoc bodies taken out. The body
# of a heredoc is data, not commands: with a quoted delimiter (<<'EOF', <<"EOF",
# <<\EOF) it is dropped; with an unquoted one the shell still expands $(...) and
# `...` in it, so the body is kept as the argument of a harmless `:` command
# (the tokenizer then recurses into the substitutions only). `<<<` is not a
# heredoc. An unterminated heredoc swallows the rest of the text, like bash.
strip_heredocs() {
	local line cmp rest out="" body=""
	local -a pd=() pq=() pdash=()
	local re='(^|[^<])<<(-?)[[:space:]]*(\\?)(["'"'"']?)([A-Za-z_][A-Za-z0-9_]*)'
	flush() { # emit the finished unquoted body
		[[ ${pq[0]} == 0 && -n $body ]] && out+=": \"${body//\"/\\\"}\""$'\n'
		body=""
		pd=("${pd[@]:1}") pq=("${pq[@]:1}") pdash=("${pdash[@]:1}")
	}
	while IFS= read -r line || [[ -n $line ]]; do
		if ((${#pd[@]} > 0)); then
			cmp=$line
			[[ ${pdash[0]} == 1 ]] && cmp=${line#"${line%%[!$'\t']*}"}
			if [[ $cmp == "${pd[0]}" ]]; then
				flush
			else
				body+=$line$'\n'
			fi
			continue
		fi
		out+=$line$'\n'
		rest=$line
		while [[ $rest =~ $re ]]; do
			pd+=("${BASH_REMATCH[5]}")
			[[ -n ${BASH_REMATCH[2]} ]] && pdash+=(1) || pdash+=(0)
			[[ -n ${BASH_REMATCH[3]}${BASH_REMATCH[4]} ]] && pq+=(1) || pq+=(0)
			rest=${rest#*"${BASH_REMATCH[0]}"}
		done
	done <<<"$1"
	while ((${#pd[@]} > 0)); do flush; done
	printf '%s' "$out"
}

# check STRING HOME_OK: print a reason and return 1 if STRING invokes a guarded
# command. HOME_OK=1 when an enclosing command already set a temp HOME.
check() {
	local s=$1 inherited=${2:-0}
	local -a toks seg=()
	local t
	s=$(strip_heredocs "$s")
	tokenize "$s"
	toks=("${TOK[@]}")
	toks+=("$SEP")
	for t in "${toks[@]}"; do
		if [[ $t == "$SEP" ]]; then
			if ((${#seg[@]} > 0)); then
				check_segment "$inherited" "${seg[@]}" || return 1
			fi
			seg=()
		else
			seg+=("$t")
		fi
	done
	return 0
}

check_segment() {
	local home_ok=$1
	shift
	local -a w=("$@")
	local i=0 x inner

	# Recurse into $(...) and `...` found in any word.
	for x in "${w[@]}"; do
		if [[ $x == *'$('* ]]; then
			check "${x#*\$(}" "$home_ok" || return 1
		elif [[ $x == *'`'* ]]; then
			inner=${x#*\`}
			check "${inner%\`*}" "$home_ok" || return 1
		fi
	done

	# Prefix: assignments, wrappers and their flags.
	while ((i < ${#w[@]})); do
		x=${w[i]}
		if [[ $x =~ ^[A-Za-z_][A-Za-z0-9_]*= ]]; then
			if [[ $x == HOME=* ]]; then
				if is_tmp_home "${x#HOME=}"; then home_ok=1; else home_ok=0; fi
			fi
		elif [[ $x =~ ^(env|command|exec|time|nohup|nice|sudo|\{|\!|if|then|else|elif|do|while|until)$ ]] ||
			[[ $x == -* ]]; then
			:
		else
			break
		fi
		i=$((i + 1))
	done
	((i < ${#w[@]})) || return 0

	local cmd=${w[i]} base args=("${w[@]:i+1}")
	base=${cmd##*/}
	case $base in
	shm | shell-manager) ;;
	task)
		[[ ${args[0]:-} == run ]] || return 0
		local j found=0
		for j in "${!args[@]}"; do
			if [[ ${args[j]} == -- ]]; then
				args=("${args[@]:j+1}")
				found=1
				break
			fi
		done
		((found)) || return 0
		;;
	go)
		[[ ${args[0]:-} == run && ${args[1]:-} =~ ^(\.|\./.*|.*\.go)$ ]] || return 0
		args=("${args[@]:2}")
		;;
	bash | sh | zsh | dash)
		local k
		for ((k = 0; k < ${#args[@]} - 1; k++)); do
			if [[ ${args[k]} =~ ^-[a-zA-Z]*c$ ]]; then
				check "${args[k + 1]}" "$home_ok" || return 1
				break
			fi
		done
		return 0
		;;
	eval)
		check "${args[*]}" "$home_ok" || return 1
		return 0
		;;
	*) return 0 ;;
	esac

	local a
	for a in "${args[@]}"; do
		if [[ $a == self-update ]]; then
			echo "Blocked: 'self-update' replaces the running executable regardless of \$HOME (see CLAUDE.md)." >&2
			return 1
		fi
		if [[ $a == setup || $a == update ]] && ((home_ok == 0)); then
			echo "Blocked: 'shm $a' against the real \$HOME (see CLAUDE.md). Prefix that same command with HOME=\$(mktemp -d)." >&2
			return 1
		fi
	done
	return 0
}

main() {
	local cmd
	cmd=$(jq -r '.tool_input.command // empty') || exit 0
	[[ -n $cmd ]] || exit 0
	check "$cmd" 0 || exit 2
	exit 0
}

# Run only when executed; sourcing (not used today) just defines the functions.
[[ ${BASH_SOURCE[0]} == "$0" ]] && main
