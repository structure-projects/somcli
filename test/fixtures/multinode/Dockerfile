# 三个 sshd 节点的镜像。用意是让"远程"真的是远程：独立的文件系统、独立的进程空间，
# 这样"文件到了目标节点、没到操作机"才断言得出来。
FROM debian:12-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends openssh-server \
    && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /run/sshd

# authorized_keys 由测试在启动前生成并挂到 /tmp/somcli-key。
# 不直接挂到 /root/.ssh：挂进来的文件属主是宿主机 uid，sshd 的 StrictModes 会拒绝它。
# 所以在启动时复制一份并改好权限，而不是关掉 StrictModes。
CMD ["sh", "-c", "mkdir -p /root/.ssh \
    && cp /tmp/somcli-key/id_rsa.pub /root/.ssh/authorized_keys \
    && chmod 700 /root/.ssh && chmod 600 /root/.ssh/authorized_keys \
    && exec /usr/sbin/sshd -D -e"]
