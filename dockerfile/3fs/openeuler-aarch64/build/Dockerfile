FROM openeuler/openeuler:22.03-lts-sp4

RUN dnf install -y \
    autoconf \
    boost-devel \
    cmake \
    double-conversion-devel \
    gcc \
    gcc-c++ \
    gflags-devel \
    git \
    glibc-devel \
    glog-devel \
    gmock-devel \
    gperftools-devel \
    gtest-devel \
    libaio-devel \
    libatomic \
    libdwarf-devel \
    libevent-devel \
    libibverbs-devel \
    libunwind-devel \
    libuv-devel \
    lld \
    lz4-devel \
    make \
    meson \
    numactl-devel \
    openssl-devel \
    python3-devel \
    wget \
    xz-devel && \
    dnf clean all && \
    rm -rf /var/cache/dnf

RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
RUN curl -LsSf https://astral.sh/uv/install.sh | sh

ARG LIBFUSE_VERSION=3.16.1
ARG FDB_VERSION=7.3.63
ARG FDB_VERFILE=${FDB_VERSION}-1
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
        wget -q https://github.com/apple/foundationdb/releases/download/${FDB_VERSION}/foundationdb-clients-${FDB_VERFILE}.el9.aarch64.rpm && \
        wget -q https://github.com/apple/foundationdb/releases/download/${FDB_VERSION}/foundationdb-server-${FDB_VERFILE}.el9.aarch64.rpm && \
        rpm -ivh foundationdb-* && \
        rm -rf /tmp/3fs

ARG BISHENG_VERSION=4.2.0
RUN mkdir /opt/bisheng && \
        cd /opt/bisheng && \
        wget -q https://mirrors.huaweicloud.com/kunpeng/archive/compiler/bisheng_compiler/BiShengCompiler-${BISHENG_VERSION}-aarch64-linux.tar.gz && \
        tar zxf BiShengCompiler-${BISHENG_VERSION}-aarch64-linux.tar.gz && \
        rm -f BiShengCompiler-${BISHENG_VERSION}-aarch64-linux.tar.gz

ENV PATH="/opt/bisheng/BiShengCompiler-${BISHENG_VERSION}-aarch64-linux/bin:${PATH}"
ENV LD_LIBRARY_PATH="/opt/bisheng/BiShengCompiler-${BISHENG_VERSION}-aarch64-linux/lib64:/opt/bisheng/BiShengCompiler-${BISHENG_VERSION}-aarch64-linux/lib:/opt/bisheng/BiShengCompiler-${BISHENG_VERSION}-aarch64-linux/lib/aarch64-unknown-linux-gnu:${LD_LIBRARY_PATH}"

RUN mkdir /3fs
WORKDIR /3fs
