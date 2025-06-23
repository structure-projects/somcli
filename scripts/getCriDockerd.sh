# !bin/bash
# 替换版本号
VERSION="v0.3.17"
CRI_DOCKERD_URL=$(curl -s https://api.github.com/repos/Mirantis/cri-dockerd/releases | grep -A 10 "tag_name.*$VERSION" | grep "browser_download_url.*amd64" | head -1 | cut -d '"' -f 4)
echo "$CRI_DOCKERD_URL"
# wget -O cri-dockerd.tgz "$CRI_DOCKERD_URL"