#!/bin/bash

# 检测系统架构
ARCH=$(uname -m)
OS=$(uname -s)

if [ "$OS" = "Linux" ]; then
  BINARY="somcli-linux-amd64"
elif [ "$OS" = "Darwin" ]; then
  if [ "$ARCH" = "arm64" ]; then
    BINARY="somcli-darwin-arm64"
  else
    BINARY="somcli-darwin-amd64"
  fi
else
  echo "Unsupported OS: $OS"
  exit 1
fi

# 安装到系统路径
sudo cp bin/$BINARY /usr/local/bin/somcli
sudo chmod +x /usr/local/bin/somcli

echo "Installed somcli to /usr/local/bin/somcli"