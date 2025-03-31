#!/bin/bash
set -euo pipefail

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 默认配置
DEFAULT_VERSION="24.0.6"
DEFAULT_PATH="$PWD/work/docker/$DEFAULT_VERSION"
data_root="/var/lib/docker"
offline_mode=false
auto_confirm=false
REGISTRY_MIRRORS=(
  "https://docker.1panel.top"
  "https://docker.m.daocloud.io"
  "https://docker.1ms.run"
  "https://docker.ketches.cn"
)

# 显示步骤信息
step() {
  echo -e "${GREEN}==>${NC} ${YELLOW}$1${NC}"
}

# 显示成功信息
success() {
  echo -e "${GREEN}[✓]${NC} $1"
}

# 显示错误信息
error() {
  echo -e "${RED}[✗]${NC} $1" >&2
}

# 显示警告信息
warn() {
  echo -e "${YELLOW}[!]${NC} $1"
}

# 显示信息
info() {
  echo -e "${BLUE}[i]${NC} $1"
}

# 确认操作
confirm() {
  if $auto_confirm; then
    return 0
  fi
  local msg=${1:-"是否继续?"}
  read -p "$msg [y/N] " answer
  [[ "$answer" =~ ^[Yy]$ ]]
}

# 检查命令是否存在
command_exists() {
  command -v "$1" &>/dev/null
}

# 检查Docker是否已安装
check_docker_installed() {
  if command_exists docker; then
    echo -e "Docker版本: ${GREEN}$(docker --version 2>/dev/null | cut -d ',' -f 1)${NC}"
    if systemctl is-active docker &>/dev/null; then
      echo -e "运行状态: ${GREEN}运行中${NC}"
    else
      echo -e "运行状态: ${YELLOW}未运行${NC}"
    fi
    return 0
  else
    return 1
  fi
}

# 检查Docker环境
check_docker_environment() {
  step "检查Docker环境"
  
  if check_docker_installed; then
    info "Docker已安装"
    
    # 检查docker-compose
    if command_exists docker-compose; then
      echo -e "docker-compose版本: ${GREEN}$(docker-compose --version)${NC}"
    else
      warn "docker-compose未安装"
    fi
    
    # 检查用户组
    if groups | grep -q '\bdocker\b'; then
      info "当前用户已在docker组"
    else
      warn "当前用户不在docker组，可能需要sudo权限"
    fi
    
    # 检查镜像加速
    if [ -f /etc/docker/daemon.json ]; then
      if command_exists jq; then
        local mirrors=$(jq -r '.registry-mirrors[]?' /etc/docker/daemon.json 2>/dev/null | tr '\n' ' ')
        if [ -n "$mirrors" ]; then
          echo -e "镜像加速配置: ${GREEN}${mirrors}${NC}"
        else
          warn "未配置镜像加速"
        fi
        local data_dir=$(jq -r '.["data-root"] // empty' /etc/docker/daemon.json 2>/dev/null)
        if [ -n "$data_dir" ]; then
          echo -e "数据目录: ${GREEN}${data_dir}${NC}"
        fi
      else
        warn "找到daemon.json但未安装jq，无法验证内容"
      fi
    else
      warn "未找到daemon.json配置文件"
    fi
    
    return 0
  else
    info "Docker未安装"
    return 1
  fi
}

# 完全卸载Docker
uninstall_docker() {
  step "开始完全卸载Docker"
  
  if ! check_docker_installed; then
    info "Docker未安装，无需卸载"
    return 0
  fi
  
  if ! $auto_confirm; then
    warn "这将完全卸载Docker并删除所有数据!"
    confirm "确定要继续卸载吗?" || {
      info "卸载已取消"
      return 1
    }
  fi
  
  # 停止服务
  if systemctl is-active --quiet docker; then
    info "停止Docker服务..."
    systemctl stop docker || {
      error "停止Docker服务失败"
      return 1
    }
  fi
  
  # 禁用服务
  if systemctl is-enabled --quiet docker; then
    info "禁用Docker服务..."
    systemctl disable docker || {
      error "禁用Docker服务失败"
      return 1
    }
  fi
  
  # 删除二进制文件
  info "删除Docker文件..."
  rm -fv /usr/bin/docker* || warn "删除docker文件失败"
  rm -fv /usr/bin/containerd* || warn "删除containerd文件失败"
  rm -fv /usr/bin/runc || warn "删除runc文件失败"
  rm -fv /usr/bin/ctr || warn "删除ctr文件失败"
  
  # 删除配置和数据
  rm -rfv /etc/docker || warn "删除/etc/docker失败"
  rm -rfv $data_root || warn "删除$data_root失败"
  rm -rfv /var/run/docker.sock || warn "删除/var/run/docker.sock失败"
  
  # 删除systemd服务
  rm -fv /usr/lib/systemd/system/docker.service || warn "删除服务文件失败"
  systemctl daemon-reload || warn "systemd重载失败"
  
  # 验证卸载
  if check_docker_installed; then
    error "Docker卸载不彻底，请手动检查"
    return 1
  else
    success "Docker已完全卸载"
    return 0
  fi
}

# 参数解析
parse_args() {
  local has_args=false
  
  while [ $# -gt 0 ]; do
    case "$1" in
      -p|--path) 
        if [ -z "$2" ]; then
          error "参数 --path 需要指定路径"
          usage
          exit 1
        fi
        path="$2"
        has_args=true
        shift 2
        ;;
      -d|--download) 
        if [ -z "$2" ]; then
          error "参数 --download 需要指定URL"
          usage
          exit 1
        fi
        download_url="$2"
        has_args=true
        shift 2
        ;;
      -v|--version) 
        if [ -z "$2" ]; then
          error "参数 --version 需要指定版本号"
          usage
          exit 1
        fi
        version="$2"
        has_args=true
        shift 2
        ;;
      -data|--data)
        if [ -z "$2" ]; then
          error "参数 --data 需要指定数据目录路径"
          usage
          exit 1
        fi
        data_root="$2"
        has_args=true
        shift 2
        ;;
      -o|--offline)
        offline_mode=true
        has_args=true
        shift
        ;;
      -y|--yes)
        auto_confirm=true
        has_args=true
        shift
        ;;
      -c|--check)
        check_docker_environment
        exit $?
        ;;
      -u|--uninstall)
        uninstall_docker
        exit $?
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      -*)
        error "未知选项: $1"
        usage
        exit 1
        ;;
      *)
        # 非选项参数尝试透传给docker
        if command_exists docker; then
          exec docker "$@"
        else
          error "Docker未安装，无法执行命令"
          exit 1
        fi
        ;;
    esac
  done
  
  # 如果没有有效参数，直接进入安装流程
  if ! $has_args; then
    return
  fi
}

usage() {
  cat <<EOF
Docker 管理脚本 (v2.4)

用法: 
  $0 [选项] [docker命令]

选项:
  -v, --version VERSION   指定Docker版本 (默认: $DEFAULT_VERSION)
  -p, --path PATH         指定安装路径 (默认: $DEFAULT_PATH)
  -d, --download URL      指定下载URL
  -data, --data PATH      指定Docker数据目录 (默认: $data_root)
  -o, --offline           离线模式(不下载直接使用本地文件)
  -y, --yes               静默模式(自动确认所有提示)
  -c, --check             检查Docker环境 (简写: -c)
  -u, --uninstall         卸载Docker (简写: -u)
  -h, --help              显示帮助信息

无参数时直接进入安装流程

示例:
  $0                        # 直接安装Docker
  $0 -c                     # 检查Docker环境
  $0 -u                     # 卸载Docker
  $0 -v 24.0.6              # 安装指定版本
  $0 -data /mnt/docker      # 指定数据目录
  $0 -o                     # 离线安装模式
  $0 -y                     # 静默安装模式
  $0 ps -a                  # 透传命令到docker (相当于 docker ps -a)
EOF
  exit 0
}

# 设置默认值
set_defaults() {
  version=${version:-$DEFAULT_VERSION}
  path=${path:-"$PWD/work/runtime/docker/$version"}
  download_url=${download_url:-"https://download.docker.com/$(uname -s | tr '[:upper:]' '[:lower:]')/static/stable/$(uname -m | tr '[:upper:]' '[:lower:]')/docker-$version.tgz"}
}

# 下载Docker (兼容旧版wget)
download_docker() {
  step "下载Docker二进制包"
  
  local package_path="$path/docker-$version.tgz"
  
  # 离线模式处理
  if $offline_mode; then
    if [ -f "$package_path" ]; then
      info "离线模式: 使用本地文件 $package_path"
      return 0
    else
      error "离线模式启用但未找到本地文件: $package_path"
      if ! confirm "是否要下载文件?"; then
        return 1
      fi
      offline_mode=false
    fi
  fi
  
  echo -e "下载地址: ${YELLOW}$download_url${NC}"
  echo -e "保存路径: ${YELLOW}$package_path${NC}"
  
  mkdir -p "$path"
  
  # 下载参数配置
  local wget_params=("-c" "-T" "300" "-O" "$package_path")
  
  # 如果支持--show-progress则添加
  if wget --help | grep -q -- --show-progress; then
    wget_params+=("--show-progress")
  fi
  
  # 重试机制
  local max_retries=3
  local retry_count=0
  
  while [ $retry_count -lt $max_retries ]; do
    if wget "${wget_params[@]}" "$download_url"; then
      success "Docker下载完成"
      return 0
    fi
    
    retry_count=$((retry_count+1))
    error "下载失败，正在重试 ($retry_count/$max_retries)..."
    sleep 5
  done
  
  error "Docker下载失败（已达最大重试次数）"
  return 1
}

# 安装Docker
install_docker() {
  step "安装Docker到系统目录"
  echo -e "解压路径: ${YELLOW}$path${NC}"
  
  if ! tar -C "$path" -zxvf "$path/docker-$version.tgz"; then
    error "Docker解压失败"
    return 1
  fi
  
  if ! cp -v "$path"/docker/* /usr/bin/; then
    error "Docker二进制文件复制失败"
    return 1
  fi
  success "Docker安装完成"
  return 0
}

# 配置systemd服务
configure_service() {
  step "配置systemd服务"
  local service_file="/usr/lib/systemd/system/docker.service"
  
  # 确保目录存在
  mkdir -p "$(dirname "$service_file")"
  
  # 直接写入服务文件，不使用临时文件
  cat > "$service_file" <<'EOF'
[Unit]
Description=Docker Application Container Engine
Documentation=https://docs.docker.com
After=network-online.target firewalld.service
Wants=network-online.target

[Service]
Type=notify
ExecStart=/usr/bin/dockerd --selinux-enabled=false
ExecReload=/bin/kill -s HUP $MAINPID
LimitNOFILE=infinity
LimitNPROC=infinity
LimitCORE=infinity
TimeoutStartSec=0
Delegate=yes
KillMode=process
Restart=on-failure
StartLimitBurst=3
StartLimitInterval=60s

[Install]
WantedBy=multi-user.target
EOF

  # 验证服务文件
  if ! systemd-analyze verify "$service_file" &>/dev/null; then
    error "服务文件验证失败"
    # 显示错误详情
    systemd-analyze verify "$service_file" || true
    return 1
  fi
  
  chmod 644 "$service_file"
  success "Docker服务配置完成"
  return 0
}

# 配置镜像加速
configure_mirrors() {
  step "配置Docker镜像加速"
  
  # 确保目录存在
  mkdir -p /etc/docker
  
  # 检查jq是否安装
  if ! command_exists jq; then
    warn "未找到jq命令，跳过daemon.json格式验证"
    # 直接写入不验证
    cat > /etc/docker/daemon.json <<EOF
{
  "data-root": "$data_root",
  "registry-mirrors": [
    "https://docker.1panel.top",
    "https://docker.m.daocloud.io",
    "https://docker.1ms.run",
    "https://docker.ketches.cn"
  ]
}
EOF
  else
    # 使用jq验证
    cat > /etc/docker/daemon.json <<EOF
{
  "data-root": "$data_root",
  "registry-mirrors": [
$(printf '    "%s",\n' "${REGISTRY_MIRRORS[@]}" | sed '$s/,$//')
  ]
}
EOF

    if ! jq empty /etc/docker/daemon.json &>/dev/null; then
      error "daemon.json格式验证失败"
      jq empty /etc/docker/daemon.json || true
      return 1
    fi
  fi
  
  echo -e "数据目录: ${GREEN}$data_root${NC}"
  echo -e "已配置镜像加速器:"
  printf '  - %s\n' "${REGISTRY_MIRRORS[@]}"
  success "镜像加速配置完成"
  return 0
}

# 启动服务
start_service() {
  step "启动Docker服务"
  
  if ! systemctl daemon-reload; then
    error "systemd重载失败"
    return 1
  fi
  
  if ! systemctl enable docker.service; then
    error "设置开机启动失败"
    return 1
  fi
  
  if ! systemctl start docker; then
    error "Docker服务启动失败"
    journalctl -xe --no-pager | tail -n 20
    return 1
  fi
  
  echo -e "运行状态: ${GREEN}$(systemctl is-active docker)${NC}"
  echo -e "开机启动: ${GREEN}$(systemctl is-enabled docker)${NC}"
  success "Docker服务启动成功"
  return 0
}

# 验证安装
verify_installation() {
  step "验证Docker安装"
  if ! command_exists docker; then
    error "Docker验证失败: docker命令未找到"
    return 1
  fi
  
  if ! docker --version; then
    error "Docker验证失败: 无法获取版本"
    return 1
  fi
  
  success "Docker验证成功"
  return 0
}

# 处理安装失败
handle_install_failure() {
  local failed_step="$1"
  
  error "安装失败在步骤: $failed_step"
  echo
  warn "可能的原因:"
  case "$failed_step" in
    "download_docker")
      if $offline_mode; then
        echo "- 离线模式启用但未找到本地文件"
      else
        echo "- 网络连接问题"
        echo "- 下载URL不可用"
        echo "- 磁盘空间不足"
      fi
      ;;
    "install_docker")
      echo "- 权限不足"
      echo "- 文件系统只读"
      ;;
    "configure_service")
      echo "- systemd配置错误"
      echo "- 临时文件创建失败"
      ;;
    "configure_mirrors")
      echo "- jq命令未安装"
      echo "- JSON格式错误"
      ;;
    "start_service")
      echo "- Docker配置错误"
      echo "- 端口冲突"
      ;;
  esac
  
  exit 1
}

# 主安装流程
install_docker_main() {
  local steps=(
    "set_defaults"
    "download_docker"
    "install_docker"
    "configure_service"
    "configure_mirrors"
    "start_service"
    "verify_installation"
  )
  
  for step_func in "${steps[@]}"; do
    if ! $step_func; then
      handle_install_failure "$step_func"
    fi
  done
}

# 主流程
main() {
  echo -e "\n${GREEN}=== Docker管理脚本 v2.4 ===${NC}\n"
  
  # 解析参数（如果没有参数会直接返回）
  parse_args "$@"
  
  # 检查是否已安装
  if check_docker_installed; then
    if ! $auto_confirm; then
      warn "Docker已经安装，如需重新安装请先卸载 (使用 -u 选项)"
      exit 1
    else
      warn "检测到Docker已安装，将在静默模式下继续安装流程"
    fi
  fi
  
  # 进入主安装流程
  install_docker_main
  
  # 安装后提示
  echo -e "\n${GREEN}=== 安装完成 ===${NC}"
  echo -e "Docker版本: ${GREEN}$(docker --version)${NC}"
  echo -e "运行状态: ${GREEN}$(systemctl is-active docker)${NC}"
  echo -e "数据目录: ${GREEN}$data_root${NC}"
  
  # 显示镜像加速配置
  if [ -f /etc/docker/daemon.json ]; then
    if command_exists jq; then
      echo -e "镜像加速: ${GREEN}$(jq -r '.registry-mirrors[]?' /etc/docker/daemon.json | tr '\n' ' ')${NC}"
    else
      echo -e "镜像加速: ${GREEN}已配置(使用cat /etc/docker/daemon.json查看)${NC}"
    fi
  fi
  
  # 提示用户组设置
  if ! groups | grep -q '\bdocker\b'; then
    echo -e "\n${YELLOW}提示: 请将用户添加到docker组以不使用sudo运行docker:${NC}"
    echo -e "  sudo usermod -aG docker \$USER"
    echo -e "  然后退出当前会话重新登录"
  fi
}

main "$@"