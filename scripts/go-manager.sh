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
readonly GO_BASE_DIR="/usr/local/go"
readonly DEFAULT_WORKSPACE="$HOME/go"
readonly DEFAULT_VERSION="1.24.1"
readonly DEFAULT_DOWNLOAD_DIR="$PWD/work/runtime/go"
readonly BACKUP_DIR="/var/backups/go"
readonly PROFILE_FILE="/etc/profile"
readonly DOWNLOAD_MIRROR="https://golang.google.cn/dl"
readonly GOPROXY_VAL="https://goproxy.io,direct"
# 获取标准安装路径
get_install_dir() {
    local version="$1"
    echo "$GO_BASE_DIR/go-$version"
}

# 打印带颜色的日志
log_info() {
    echo -e "${CYAN}[INFO]${NC} $1" >&2
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1" >&2
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1" >&2
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
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

# 显示帮助信息
show_help() {
    cat <<EOF
Go 环境管理工具 v1.2

命令:
  install    安装指定版本
  uninstall 卸载当前版本
  list      列出已安装版本
  use       切换Go版本
  current   显示当前使用版本

选项:
  -v, --version <版本>  指定Go版本 (默认: $DEFAULT_VERSION)
  -w, --workspace <路径> 设置GOPATH工作目录 (默认: $DEFAULT_WORKSPACE)
  -m, --mirror <URL>    自定义下载镜像
  -y, --yes            自动确认所有提示
  -h, --help           显示本帮助信息

示例:
  $0 install -v 1.24.1    # 安装Go 1.24.1
  $0 use 1.24.1           # 切换使用版本
  $0 list                # 查看可用版本
EOF
}

# 解析参数
parse_options() {
    local options=$(getopt -o v:w:m:yh --long version:,workspace:,mirror:,yes,help -- "$@")
    eval set -- "$options"
    
    local version=""
    local workspace=""
    local mirror=""
    local auto_confirm="false"
    
    while true; do
        case "$1" in
            -v|--version) shift; version="$1"; shift ;;
            -w|--workspace) shift; workspace="$1"; shift ;;
            -m|--mirror) shift; mirror="$1"; shift ;;
            -y|--yes) auto_confirm="true"; shift ;;
            -h|--help) show_help; exit 0 ;;
            --) shift; break ;;
            *) log_error "无效参数: $1"; show_help; exit 1 ;;
        esac
    done
    
    echo "$version,$workspace,$mirror,$auto_confirm"
}

# 获取系统架构
get_arch() {
    case $(uname -m) in
        x86_64) echo "amd64" ;;
        aarch64) echo "arm64" ;;
        *) log_error "不支持的架构: $(uname -m)"; exit 1 ;;
    esac
}

# 验证文件完整性
verify_archive() {
    local archive="$1"
    log_info "验证文件完整性: $archive"
    
    if ! tar -tzf "$archive" >/dev/null 2>&1; then
        log_warning "文件已损坏或格式不正确"
        return 1
    fi
    return 0
}

# 下载Go
download_go() {
    local version="$1"
    local mirror="${2:-$DOWNLOAD_MIRROR}"
    local arch=$(get_arch)
    local download_url="$mirror/go$version.linux-$arch.tar.gz"
    local download_dir="$DEFAULT_DOWNLOAD_DIR/$version"
    local download_file="$download_dir/go$version.linux-$arch.tar.gz"

    mkdir -p "$download_dir"
    
    # 检查是否已下载且完整
    if [ -f "$download_file" ]; then
        if verify_archive "$download_file"; then
            log_info "使用已下载的安装包: $download_file"
            echo "$download_file"
            return 0
        else
            log_warning "删除损坏的文件并重新下载"
            rm -f "$download_file"
        fi
    fi

    log_info "下载Go $version..."
    log_info "下载URL: $download_url"
    log_info "保存路径: $download_file"
    
    if ! curl -L --progress-bar -o "$download_file" "$download_url"; then
        log_error "下载失败!"
        rm -f "$download_file" 2>/dev/null
        exit 1
    fi
    
    # 验证下载文件
    if ! verify_archive "$download_file"; then
        log_error "下载文件验证失败"
        rm -f "$download_file" 2>/dev/null
        exit 1
    fi
    
    echo "$download_file"
}

# 安装Go
install_go() {
    local version="$1"
    local archive="$2"
    local install_dir=$(get_install_dir "$version")
    local backup_dir="$BACKUP_DIR/$(date +%Y%m%d%H%M%S)-go-$version"

    log_info "安装Go $version到 $install_dir..."
    
    # 创建备份目录并设置权限
    sudo mkdir -p "$BACKUP_DIR" "$GO_BASE_DIR"
    sudo chmod 755 "$BACKUP_DIR" "$GO_BASE_DIR"
    sudo chown $(id -u):$(id -g) "$BACKUP_DIR"

    # 备份现有安装
    if [ -d "$install_dir" ]; then
        log_warning "检测到已安装版本"
        if [ "$auto_confirm" != "true" ] && ! confirm "是否重新安装?"; then
            return 1
        fi
        
        log_info "创建备份到: $backup_dir"
        sudo mv "$install_dir" "$backup_dir" || {
            log_error "备份失败"
            exit 1
        }
    fi

    # 解压安装
    log_info "正在解压安装包..."
    if ! sudo tar -C "$GO_BASE_DIR" -xzf "$archive"; then
        log_error "解压安装失败!"
        exit 1
    fi
    
    # 重命名解压后的目录
    if [ -d "$GO_BASE_DIR/go" ]; then
        sudo mv "$GO_BASE_DIR/go" "$install_dir"
    else
        log_error "安装目录结构不符合预期"
        exit 1
    fi
    
    # 验证安装
    if [ ! -f "$install_dir/bin/go" ]; then
        log_error "安装验证失败: 未找到go可执行文件"
        exit 1
    fi
    
    log_success "安装完成"
}

# 配置环境变量
setup_environment() {
    local version="$1"
    local workspace="${2:-$DEFAULT_WORKSPACE}"
    local install_dir=$(get_install_dir "$version")
    
    log_info "配置环境变量..."
    
    # 定义标记
    local start_marker="# BEGIN GO PATH"
    local end_marker="# END GO PATH"

    # 清理旧配置
    sudo sed -i "/$start_marker/,/$end_marker/d" "$PROFILE_FILE" 2>/dev/null
    
    # 添加新配置
    sudo tee -a "$PROFILE_FILE" > /dev/null <<EOF
$start_marker
export GOROOT="$install_dir"
export GOPATH="$workspace"
export GOPROXY="$GOPROXY_VAL"
export GOPRIVATE=""
export GOSUMDB="sum.golang.org"
export PATH=\$GOROOT/bin:\${GOPATH//://bin:}/bin:\$PATH
$end_marker
EOF
    
    # 立即生效
    export GOROOT="$install_dir"
    export GOPATH="$workspace"
    export GOPROXY="$GOPROXY_VAL"
    export PATH="$GOROOT/bin:${GOPATH//://bin:}/bin:$PATH"
    
    log_success "环境变量已配置到 $PROFILE_FILE"
    source $PROFILE_FILE
}

# 获取已安装版本
list_installed() {
    echo -e "${CYAN}[INFO]${NC} 已安装的Go版本:"
    
    if [ -d "$GO_BASE_DIR" ]; then
        find "$GO_BASE_DIR" -maxdepth 1 -type d -name "go-*" | sort | while read -r dir; do
            version=$(basename "$dir" | sed 's/^go-//')
            
            # 标记当前使用的版本
            if [[ "$dir" == "${GOROOT:-}" ]]; then
                echo -e "  ${GREEN}* ${version} ($dir) [当前使用]${NC}"
            else
                echo -e "    ${BLUE}${version} ($dir)${NC}"
            fi
        done
        
        if [ $(find "$GO_BASE_DIR" -maxdepth 1 -type d -name "go-*" | wc -l) -eq 0 ]; then
            echo -e "${YELLOW}[WARNING]${NC} 未找到任何安装版本"
        fi
    else
        echo -e "${YELLOW}[WARNING]${NC} Go安装目录 $GO_BASE_DIR 不存在"
    fi
}

# 卸载Go
uninstall_go() {
    local version="$1"
    local install_dir=$(get_install_dir "$version")
    
    if [ ! -d "$install_dir" ]; then
        log_error "Go $version 未安装"
        exit 1
    fi
    
    if [ "$auto_confirm" != "true" ] && ! confirm "确定要卸载Go $version吗?"; then
        exit 0
    fi
    
    local backup_dir="$BACKUP_DIR/$(date +%Y%m%d%H%M%S)-go-$version"
    
    log_info "正在备份并卸载Go $version..."
    sudo mkdir -p "$BACKUP_DIR"
    sudo mv "$install_dir" "$backup_dir"
    
    # 如果卸载的是当前版本，清理环境变量
    if [ "$GOROOT" = "$install_dir" ] || [ "$(command -v go 2>/dev/null)" = "$install_dir/bin/go" ]; then
        local start_marker="# BEGIN GO PATH"
        local end_marker="# END GO PATH"
        sudo sed -i "/$start_marker/,/$end_marker/d" "$PROFILE_FILE" 2>/dev/null
        log_warning "已移除当前使用的Go版本，请手动执行以下命令或重新登录:"
        echo "  source $PROFILE_FILE"
    fi
    
    log_success "已卸载Go $version (备份在 $backup_dir)"
}

# 主函数
main() {
    local command="${1:-install}"  # 默认执行install命令
    shift
    
    # 解析选项
    IFS=',' read -r version workspace mirror auto_confirm <<< "$(parse_options "$@")"
    
    case "$command" in
        install)
            version="${version:-$DEFAULT_VERSION}"
            workspace="${workspace:-$DEFAULT_WORKSPACE}"
            
            log_info "准备安装Go $version"
            log_info "工作目录: $workspace"
            
            local archive=$(download_go "$version" "$mirror")
            install_go "$version" "$archive"
            setup_environment "$version" "$workspace"
            
            log_info "当前Go版本:"
            go version
            ;;
        uninstall)
            if [ -z "$version" ]; then
                # 如果没有指定版本，尝试卸载当前版本
                if command -v go >/dev/null; then
                    version=$(go version | awk '{print $3}' | sed 's/go//')
                    install_dir=$(get_install_dir "$version")
                    if [ ! -d "$install_dir" ]; then
                        log_error "当前Go版本不是通过本工具安装的"
                        exit 1
                    fi
                else
                    log_error "请指定要卸载的版本或先设置当前Go版本"
                    exit 1
                fi
            fi
            uninstall_go "$version"
            ;;
        list)
            list_installed
            ;;
        use)
            if [ -z "$version" ]; then
                log_error "请指定要使用的版本"
                list_installed
                exit 1
            fi
            
            local install_dir=$(get_install_dir "$version")
            if [ ! -d "$install_dir" ]; then
                log_error "Go $version 未安装"
                list_installed
                exit 1
            fi
            
            setup_environment "$version" "${workspace:-$DEFAULT_WORKSPACE}"
            log_success "已切换到Go $version"
            log_info "当前Go版本:"
            go version
            ;;
        current)
            if command -v go >/dev/null; then
                log_info "当前Go版本:"
                go version
                log_info "GOROOT: ${GOROOT:-未设置}"
                log_info "GOPATH: ${GOPATH:-未设置}"
                log_info "GOPROXY: ${GOPROXY:-未设置}"
            else
                log_warning "Go未安装或未配置"
            fi
            ;;
        *)
            show_help
            exit 1
            ;;
    esac
}

# 启动脚本
main "$@"