#
# somcli build
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

# 版本信息配置
VERSION="v1.0.0"
COMMIT=$(git rev-parse HEAD)
DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ')

# 清理并创建输出目录
OUTPUT_DIR="bin"
rm -rf ${OUTPUT_DIR}
mkdir -p ${OUTPUT_DIR}

# 构建 Linux amd64 版本
echo "构建 Linux amd64 版本..."
GOOS=linux GOARCH=amd64 go build \
  -ldflags "-X github.com/structure-projects/somcli/cmd.Version=$VERSION \
            -X github.com/structure-projects/somcli/cmd.GitCommit=$COMMIT \
            -X github.com/structure-projects/somcli/cmd.BuildDate=$DATE" \
  -o ${OUTPUT_DIR}/somcli-linux-amd64

# 构建 macOS Intel 版本
echo "构建 macOS amd64 版本..."
GOOS=darwin GOARCH=amd64 go build \
  -ldflags "-X github.com/structure-projects/somcli/cmd.Version=$VERSION \
            -X github.com/structure-projects/somcli/cmd.GitCommit=$COMMIT \
            -X github.com/structure-projects/somcli/cmd.BuildDate=$DATE" \
  -o ${OUTPUT_DIR}/somcli-darwin-amd64

# 构建 macOS ARM 版本
echo "构建 macOS arm64 版本..."
GOOS=darwin GOARCH=arm64 go build \
  -ldflags "-X github.com/structure-projects/somcli/cmd.Version=$VERSION \
            -X github.com/structure-projects/somcli/cmd.GitCommit=$COMMIT \
            -X github.com/structure-projects/somcli/cmd.BuildDate=$DATE" \
  -o ${OUTPUT_DIR}/somcli-darwin-arm64

# 设置可执行权限
chmod +x ${OUTPUT_DIR}/*

echo "构建完成！"
echo "输出文件:"
ls -lh ${OUTPUT_DIR}/