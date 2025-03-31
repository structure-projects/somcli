#
# somcli jdk-manager
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

# 增强错误处理
set -eo pipefail

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# 定义常量
readonly DEFAULT_BASE_PATH="$PWD/work/runtime/openJdk"
readonly DEFAULT_VERSION="1.8.0"
readonly INSTALL_DIR="/usr/local/openjdk"
readonly BACKUP_DIR="/usr/local/openjdk/backups"
readonly PROFILE_DIR="/etc/profile.d"
readonly JDK_PROFILE="$PROFILE_DIR/openjdk.sh"
readonly DOWNLOAD_MIRROR="https://mirrors.huaweicloud.com/openjdk"

# 打印带颜色的日志
log_info() {
    echo -e "${CYAN}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

# 获取纯净的文件路径
get_clean_path() {
    local path="$1"
    echo "$path" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//'
}

# 初始化环境
init_environment() {
    log_info "初始化环境..."
    sudo mkdir -p "$INSTALL_DIR" "$BACKUP_DIR" "$PROFILE_DIR"
    sudo chmod 755 "$INSTALL_DIR" "$BACKUP_DIR"
    mkdir -p "$DEFAULT_BASE_PATH"
    
    [[ -f "/etc/profile" ]] && source "/etc/profile"
    [[ -d "$PROFILE_DIR" ]] && {
        for script in "$PROFILE_DIR"/*.sh; do
            source "$script" >/dev/null 2>&1 || true
        done
    }
    log_success "环境初始化完成"
}

# 显示帮助信息
show_help() {
    cat <<EOF
OpenJDK 管理工具 v3.9

命令:
  install    安装指定版本
  uninstall 卸载指定版本
  list      列出已安装版本
  use       切换当前使用版本
  current   显示当前使用版本

选项:
  -v, --version <版本>  指定JDK版本 (默认: $DEFAULT_VERSION)
  -m, --mirror <URL>    自定义下载镜像
  -y, --yes            自动确认所有提示
  -h, --help           显示本帮助信息

示例:
  $0 install -v 1.8.0    # 安装JDK 1.8.0
  $0 use 1.8.0           # 切换使用版本
  $0 list                # 查看可用版本
EOF
}

# 检查系统环境
check_environment() {
    log_info "检查系统环境..."
    local missing=()
    
    for cmd in wget tar java; do
        if ! command -v "$cmd" >/dev/null; then
            missing+=("$cmd")
        fi
    done

    if [[ ${#missing[@]} -gt 0 ]]; then
        log_error "缺少必要依赖: ${missing[*]}"
        exit 1
    fi

    local min_space=500
    local avail_space=$(df -m "$INSTALL_DIR" | awk 'NR==2 {print $4}')
    if [[ $avail_space -lt $min_space ]]; then
        log_error "磁盘空间不足 (需要至少 ${min_space}MB)"
        exit 1
    fi
    log_success "环境检查通过"
}

# 交互确认
confirm() {
    [[ "${auto_confirm:-}" == "true" ]] && return 0
    
    local msg="${1:-是否继续?}"
    local default="${2:-Y}"
    
    read -rp "$(echo -e "${BLUE}[INPUT]${NC} $msg [Y/n] ")" answer
    case "${answer:-$default}" in
        [Yy]*) return 0 ;;
        *) return 1 ;;
    esac
}

# 标准化JDK安装路径
get_standard_install_path() {
    local version="$1"
    echo "$INSTALL_DIR/jdk-$version"
}

# 获取已安装版本 (优化版)
list_installed() {
    echo -e "${CYAN}[INFO]${NC} 已安装的OpenJDK版本:"
    
    if [ -d "${INSTALL_DIR}" ]; then
        # 查找所有jdk和java开头的目录，排除备份目录
        find "${INSTALL_DIR}" -maxdepth 1 -type d \( -name "jdk*" -o -name "java*" \) ! -path "${BACKUP_DIR}*" | sort | while read -r dir; do
            version=$(basename "$dir" | sed 's/^jdk-//;s/^java-//')
            
            # 如果提取后版本号没变化，标记为unknown
            if [ "$version" == "$(basename "$dir")" ]; then
                version="unknown"
            fi
            
            # 标记当前使用的版本
            if [[ "$dir" == "$JAVA_HOME" ]]; then
                echo -e "  ${GREEN}* ${version} ($dir) [当前使用]${NC}"
            else
                echo -e "    ${BLUE}${version} ($dir)${NC}"
            fi
        done
        
        # 检查是否找到任何版本
        if [ $(find "${INSTALL_DIR}" -maxdepth 1 -type d \( -name "jdk*" -o -name "java*" \) ! -path "${BACKUP_DIR}*" | wc -l) -eq 0 ]; then
            echo -e "${YELLOW}[WARNING]${NC} 未找到任何安装版本"
        fi
    else
        echo -e "${YELLOW}[WARNING]${NC} 安装目录 ${INSTALL_DIR} 不存在"
    fi
}

# 下载JDK
download_jdk() {
    local version="$1"
    local download_url
    local download_file
    
    if [[ "$version" == "1.8.0" ]]; then
        download_url="$DOWNLOAD_MIRROR/java-jse-ri/jdk8u40/openjdk-8u40-b25-linux-x64-10_feb_2015.tar.gz"
        download_file="$DEFAULT_BASE_PATH/openjdk-8u40-b25-linux-x64-10_feb_2015.tar.gz"
    else
        download_url="$DOWNLOAD_MIRROR/$version/openjdk-${version}_linux-x64_bin.tar.gz"
        download_file="$DEFAULT_BASE_PATH/openjdk-${version}_linux-x64_bin.tar.gz"
    fi

    echo -e "${CYAN}[INFO]${NC} == 下载信息 ==" >&2
    echo -e "${CYAN}[INFO]${NC} 版本: $version" >&2
    echo -e "${CYAN}[INFO]${NC} 下载URL: $download_url" >&2
    echo -e "${CYAN}[INFO]${NC} 本地路径: $download_file" >&2
    
    if [[ -f "$download_file" ]]; then
        echo -e "${YELLOW}[WARNING]${NC} 文件已存在，跳过下载" >&2
    else
        echo -e "${CYAN}[INFO]${NC} 开始下载..." >&2
        if ! wget --progress=bar:force -O "$download_file" "$download_url"; then
            echo -e "${RED}[ERROR]${NC} 下载失败!" >&2
            exit 1
        fi
        echo -e "${GREEN}[SUCCESS]${NC} 下载完成" >&2
    fi
    
    readlink -f "$download_file" | tr -d '\n'
}

# 标准化安装目录结构
standardize_installation() {
    local src_dir="$1"
    local target_dir="$2"
    
    log_info "标准化安装目录..."
    
    if [[ "$src_dir" == "$target_dir" ]]; then
        return
    fi
    
    sudo mkdir -p "$(dirname "$target_dir")"
    if [[ -d "$target_dir" ]]; then
        local backup_dir="${target_dir}-backup-$(date +%Y%m%d%H%M%S)"
        log_warning "目标目录已存在，创建备份: $backup_dir"
        sudo mv "$target_dir" "$backup_dir"
    fi
    
    sudo mv "$src_dir" "$target_dir"
    log_success "已标准化目录结构: $target_dir"
}

# 安装JDK
install_jdk() {
    local version="$1"
    local archive="$2"
    local install_path=$(get_standard_install_path "$version")
    local backup_path="$BACKUP_DIR/jdk-$version-$(date +%Y%m%d%H%M%S)"

    archive=$(get_clean_path "$archive")
    
    echo -e "${CYAN}[INFO]${NC} == 安装信息 =="
    echo -e "${CYAN}[INFO]${NC} 版本: $version"
    echo -e "${CYAN}[INFO]${NC} 归档文件: $archive"
    echo -e "${CYAN}[INFO]${NC} 安装路径: $install_path"

    if [[ ! -f "$archive" ]]; then
        log_error "归档文件不存在"
        exit 1
    fi

    log_info "检查文件完整性..."
    if ! gzip -t "$archive" 2>/dev/null; then
        log_error "归档文件已损坏"
        exit 1
    fi
    log_success "文件完整性验证通过"

    if [[ -d "$install_path" ]]; then
        log_warning "检测到已安装版本"
        if ! confirm "是否重新安装?"; then
            return 1
        fi
        
        if confirm "是否创建备份?"; then
            log_info "创建备份到: $backup_path"
            sudo mv "$install_path" "$backup_path"
        else
            log_warning "直接覆盖安装"
            sudo rm -rf "$install_path"
        fi
    fi

    log_info "开始安装..."
    local temp_dir=$(mktemp -d)
    if ! sudo tar -xzf "$archive" -C "$temp_dir"; then
        log_error "解压安装失败!"
        sudo rm -rf "$temp_dir"
        exit 1
    fi
    
    local extracted_dir=$(find "$temp_dir" -maxdepth 1 -type d -name "jdk*" -o -name "java*" | head -n1)
    if [[ -z "$extracted_dir" ]]; then
        log_error "无法找到JDK目录"
        sudo rm -rf "$temp_dir"
        exit 1
    fi
    
    standardize_installation "$extracted_dir" "$install_path"
    sudo rm -rf "$temp_dir"
    
    sudo chown -R root:root "$install_path"
    sudo chmod -R 755 "$install_path"
    log_success "安装完成"
}

# 配置环境
setup_environment() {
    local jdk_home="$1"
    
    log_info "== 环境配置 =="
    log_info "JAVA_HOME: $jdk_home"
    
    sudo bash -c "cat > '$JDK_PROFILE'" <<EOF
export JAVA_HOME='$jdk_home'
export PATH=\$JAVA_HOME/bin:\$PATH
EOF

    source "$JDK_PROFILE"
    export JAVA_HOME="$jdk_home"
    export PATH="$JAVA_HOME/bin:$PATH"
    
    log_success "环境变量已配置到: $JDK_PROFILE"
    
    log_info "当前Java版本:"
    java -version
}

# 切换版本
switch_version() {
    local version="$1"
    local jdk_path=$(get_standard_install_path "$version")
    
    if [[ ! -d "$jdk_path" ]]; then
        log_error "JDK $version 未安装"
        list_installed
        exit 1
    fi
    
    setup_environment "$jdk_path"
    log_success "已成功切换到 JDK $version"
}

# 卸载版本
uninstall_jdk() {
    local version="$1"
    local jdk_path=$(get_standard_install_path "$version")
    
    if [[ ! -d "$jdk_path" ]]; then
        log_error "JDK $version 未安装"
        list_installed
        exit 1
    fi
    
    if ! confirm "确定要卸载 JDK $version 吗?"; then
        exit 0
    fi
    
    log_info "正在卸载 JDK $version..."
    sudo rm -rf "$jdk_path"
    
    if [[ "$JAVA_HOME" == "$jdk_path" ]]; then
        sudo rm -f "$JDK_PROFILE"
        log_warning "当前使用的JDK已被移除，请重新安装或切换其他版本"
    fi
    log_success "卸载完成"
}

# 主函数
main() {
    local command="install"
    local version="$DEFAULT_VERSION"
    local auto_confirm="false"
    
    while [[ $# -gt 0 ]]; do
        case "$1" in
            install|uninstall|use|list|current)
                command="$1"
                shift
                ;;
            -v|--version)
                version="$2"
                shift 2
                ;;
            -m|--mirror)
                DOWNLOAD_MIRROR="$2"
                shift 2
                ;;
            -y|--yes)
                auto_confirm="true"
                shift
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            *)
                log_error "无效参数: $1"
                show_help
                exit 1
                ;;
        esac
    done

    init_environment
    
    case "$command" in
        install)
            check_environment
            log_info "开始安装 JDK $version..."
            local archive
            archive=$(download_jdk "$version")
            echo -e "${CYAN}[INFO]${NC} 获取到归档文件: $archive"
            install_jdk "$version" "$archive"
            switch_version "$version"
            ;;
        use)
            switch_version "$version"
            ;;
        uninstall)
            uninstall_jdk "$version"
            ;;
        list)
            list_installed
            ;;
        current)
            if [[ -n "$JAVA_HOME" ]]; then
                log_info "当前使用版本: $(basename "$JAVA_HOME")"
                log_info "安装路径: $JAVA_HOME"
                java -version
            else
                log_warning "未检测到当前使用的JDK"
            fi
            ;;
        *)
            show_help
            exit 1
            ;;
    esac
}

main "$@"