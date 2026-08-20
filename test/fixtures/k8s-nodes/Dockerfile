# k8s 节点镜像：systemd 是 PID 1，sshd 由 systemd 拉起。
#
# 为什么必须是 systemd 而不是像 multinode 那样直接 exec sshd：
# somcli 装 containerd / kubelet 全靠 `systemctl daemon-reload / enable --now`，
# kubeadm 的 preflight 也要求 kubelet 是个 enabled 的服务。没有 PID 1 的 systemd，
# 安装必然停在第一个 systemctl 上 —— 那时什么集群断言都跑不到。
#
# 镜像里只装"节点本来就该有的东西"（systemd / sshd / iptables / conntrack …）。
# containerd、runc、CNI 插件、kubeadm 一律不预装 —— 装它们正是 somcli 要被验证的事，
# 预装等于把要验的东西提前替它做了。
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

# conntrack / socat / ethtool / iptables 是 kubeadm preflight 明确检查的；
# kmod 让容器内的 modprobe 能用（内核是宿主的，模块也从宿主的 /lib/modules 挂进来）。
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        systemd systemd-sysv dbus \
        openssh-server \
        ca-certificates curl tar gzip \
        iproute2 iptables ethtool socat conntrack kmod procps \
    && rm -rf /var/lib/apt/lists/*

# 容器里注定失败或毫无意义的单元先摘掉。留着的话 systemd 会一直在重试，
# 日志被刷满，真正的失败反而看不见。
RUN systemctl mask \
        systemd-udevd.service \
        systemd-udev-trigger.service \
        systemd-modules-load.service \
        systemd-journald-audit.socket \
        sys-kernel-debug.mount \
        sys-kernel-config.mount \
    && systemctl enable ssh.service

RUN mkdir -p /run/sshd

COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod 0755 /usr/local/bin/entrypoint.sh

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
