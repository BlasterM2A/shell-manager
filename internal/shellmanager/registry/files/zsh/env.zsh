# Managed by shm; changes here are overwritten by `shm update`.
# Your environment goes in ~/.zsh/env/*.zsh (loaded by every zsh, keep it quiet).
(( ${+_SHM_ENV_LOADED} )) && return
_SHM_ENV_LOADED=1
typeset -U path
for _shm_f in "$HOME"/.zsh/env/*.zsh(N); do source "$_shm_f"; done
unset _shm_f
