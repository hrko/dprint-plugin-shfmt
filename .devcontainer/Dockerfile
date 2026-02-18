FROM mcr.microsoft.com/devcontainers/base:ubuntu-24.04

# Install Fish shell
RUN export DEBIAN_FRONTEND=noninteractive && \
    sudo apt-get update && \
    sudo apt-get install -y software-properties-common && \
    sudo add-apt-repository -y ppa:fish-shell/release-4 && \
    sudo apt-get update && \
    sudo apt-get install -y fish && \
    sudo apt-get clean && \
    sudo rm -rf /var/lib/apt/lists/*
