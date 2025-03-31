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