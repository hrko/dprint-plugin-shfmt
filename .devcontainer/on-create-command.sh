#!/bin/bash

mkdir -p ~/.local/bin
mkdir -p ~/.config/fish

curl https://mise.run | sh
echo 'eval "$(~/.local/bin/mise activate bash)"' >> ~/.bashrc
echo 'eval "$(~/.local/bin/mise activate zsh)"' >> ~/.zshrc
echo '~/.local/bin/mise activate fish | source' >> ~/.config/fish/config.fish

curl -sS https://starship.rs/install.sh | env BIN_DIR=~/.local/bin FORCE=1 sh
echo 'eval "$(starship init bash)"' >> ~/.bashrc
echo 'eval "$(starship init zsh)"' >> ~/.zshrc
echo 'starship init fish | source' >> ~/.config/fish/config.fish

curl --proto '=https' --tlsv1.2 -LsSf https://setup.atuin.sh | env ATUIN_INSTALL_DIR=~/.local/bin sh
# Atuin will automatically update the shell configuration files, so no additional setup is needed here.
