#!/bin/bash

mkdir -p $HOME/.kube
kubectl completion bash > /home/vscode/.kube/completion.bash.inc
printf "
source /usr/share/bash-completion/bash_completion
source $HOME/.kube/completion.bash.inc
complete -F __start_kubectl k
" >> $HOME/.bashrc

printf "
source <(kubectl completion zsh)
complete -F __start_kubectl k
" >> $HOME/.zshrc

make setup-kindev

cp .kind/.kind/kind-config ~/.kube/config
