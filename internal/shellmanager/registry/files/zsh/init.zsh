# Managed by shm; changes here are overwritten by `shm update`.
# Your config goes in ~/.zsh/{env,functions,completions,aliases,local}/
[[ -o interactive ]] || return
[[ -r "$HOME/.zsh/env.zsh" ]] && source "$HOME/.zsh/env.zsh"
fpath=("$HOME/.zsh/completions" "$HOME/.zsh/functions" $fpath)
() { (( $# )) && autoload -Uz "$@" } "$HOME"/.zsh/functions/*(N-.:t)
for _shm_f in "$HOME"/.zsh/{tools,aliases,local}/*.zsh(N); do source "$_shm_f"; done
unset _shm_f
