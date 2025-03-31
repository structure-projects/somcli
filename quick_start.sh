#!/bin/bash

# 定义版本和下载URL基础路径
VERSION="v1.0.0"
BASE_URL="https://github.com/structure-projects/somcli/releases/download/${VERSION}"

# 检测系统架构
ARCH=$(uname -m)
OS=$(uname -s)

# 根据系统架构确定下载文件名
if [ "$OS" = "Linux" ]; then
  if [ "$ARCH" = "x86_64" ]; then
    BINARY="somcli-linux-amd64"
  elif [ "$ARCH" = "aarch64" ]; then
    BINARY="somcli-linux-arm64"
  else
    echo "Unsupported Linux architecture: $ARCH"
    exit 1
  fi
elif [ "$OS" = "Darwin" ]; then
  if [ "$ARCH" = "arm64" ]; then
    BINARY="somcli-darwin-arm64"
  elif [ "$ARCH" = "x86_64" ]; then
    BINARY="somcli-darwin-amd64"
  else
    echo "Unsupported macOS architecture: $ARCH"
    exit 1
  fi
else
  echo "Unsupported OS: $OS"
  exit 1
fi

# 创建临时目录
TMP_DIR=$(mktemp -d)
echo "Using temporary directory: $TMP_DIR"

# 下载对应版本的二进制文件
DOWNLOAD_URL="${BASE_URL}/${BINARY}"
echo "Downloading somcli from: $DOWNLOAD_URL"

if ! curl -L -o "$TMP_DIR/somcli" "$DOWNLOAD_URL"; then
  echo "Failed to download somcli"
  rm -rf "$TMP_DIR"
  exit 1
fi

# 设置可执行权限
chmod +x "$TMP_DIR/somcli"

# 安装到系统路径
INSTALL_PATH="/usr/local/bin/somcli"
echo "Installing somcli to $INSTALL_PATH"

if ! sudo mv "$TMP_DIR/somcli" "$INSTALL_PATH"; then
  echo "Failed to install somcli"
  rm -rf "$TMP_DIR"
  exit 1
fi

# 清理临时文件
rm -rf "$TMP_DIR"

# 验证安装
if command -v somcli >/dev/null 2>&1; then
  echo "Installation successful!"
  echo "somcli version: $(somcli --version)"
else
  echo "Installation failed - somcli not found in PATH"
  exit 1
fi