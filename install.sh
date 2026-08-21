#!/bin/bash
# somcli Installer
# Copyright [2023] [Structure Projects]
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# 检测系统架构
ARCH=$(uname -m)
OS=$(uname -s)

# uname -m 到 GOARCH 的映射：x86_64→amd64，aarch64/arm64→arm64。
case "$ARCH" in
  x86_64|amd64) GOARCH="amd64" ;;
  aarch64|arm64) GOARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

case "$OS" in
  Linux) BINARY="somcli-linux-${GOARCH}" ;;
  Darwin) BINARY="somcli-darwin-${GOARCH}" ;;
  *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

# 安装到系统路径
sudo cp bin/$BINARY /usr/local/bin/somcli
sudo chmod +x /usr/local/bin/somcli

echo "Installed somcli to /usr/local/bin/somcli"