#!/bin/sh
# 起 systemd 之前把节点收拾成 kubelet 能接受的样子。
#
# 这些事没法写在 Dockerfile 里：authorized_keys 依赖运行时挂进来的公钥，
# /dev/kmsg 依赖运行时的设备。
set -e

# 公钥由测试生成后挂到 /tmp/somcli-key。不直接挂到 /root/.ssh：
# 挂进来的文件属主是宿主 uid，sshd 的 StrictModes 会拒绝它 ——
# 复制一份改好权限，而不是把 StrictModes 关掉。
mkdir -p /root/.ssh
cp /tmp/somcli-key/id_rsa.pub /root/.ssh/authorized_keys
chmod 700 /root/.ssh
chmod 600 /root/.ssh/authorized_keys

# kubelet 启动时要往 /dev/kmsg 写，容器里没有这个设备就直接退出。
# 指到 /dev/console 上（kind 的做法），内容会落到容器日志里。
[ -e /dev/kmsg ] || ln -s /dev/console /dev/kmsg

exec /sbin/init
