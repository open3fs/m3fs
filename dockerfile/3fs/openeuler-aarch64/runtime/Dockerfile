FROM openeuler/openeuler:22.03-lts-sp4

RUN dnf install -y \
    meson \
    gdb \
    libatomic \
    libibverbs \
    libaio \
    libunwind \
    boost \
    double-conversion \
    gflags \
    glog \
    gperftools-libs \
    jemalloc \
    strace \
    wget \
    numactl \
    util-linux && \
    dnf clean all && \
    rm -rf /var/cache/dnf

RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
RUN curl -LsSf https://astral.sh/uv/install.sh | sh

ARG LIBFUSE_VERSION=3.16.1
RUN mkdir /tmp/3fs && \
        cd /tmp/3fs && \
        wget -q https://github.com/libfuse/libfuse/releases/download/fuse-${LIBFUSE_VERSION}/fuse-${LIBFUSE_VERSION}.tar.gz && \
        tar zxf fuse-${LIBFUSE_VERSION}.tar.gz && \
        cd fuse-${LIBFUSE_VERSION}/ && \
        mkdir build && \
        cd build && \
        meson setup .. && \
        ninja && \
        ninja install && \
        rm -rf /tmp/3fs

# ibdev2netdev is used by many program to get netdev from ib device
# This one is copied from Mellanox OFED driver.
COPY ibdev2netdev /usr/sbin/ibdev2netdev
COPY root_profile /root/.profile

ARG FDB_VERSION=7.3.63
ARG FDB_VERFILE=${FDB_VERSION}-1
RUN mkdir /tmp/3fs && \
        cd /tmp/3fs && \
        wget -q https://github.com/apple/foundationdb/releases/download/${FDB_VERSION}/foundationdb-clients-${FDB_VERFILE}.el9.aarch64.rpm && \
        rpm -ivh foundationdb-* && \
        rm -rf /tmp/3fs
