FROM ubuntu:jammy-20250126

RUN apt update && \
        apt install -y \
        build-essential \
        clang-14 \
        clang-format-14 \
        clang-tidy-14 \
        cmake \
        g++-12 \
        gcc-12 \
        git \
        meson \
        strace \
        google-perftools \
        libaio-dev \
        libboost-all-dev \
        libdouble-conversion-dev \
        libdwarf-dev \
        libgflags-dev \
        libgmock-dev \
        libgoogle-glog-dev \
        libgoogle-perftools-dev \
        libgtest-dev \
        liblz4-dev \
        liblzma-dev \
        libprocps-dev \
        libssl-dev \
        libunwind-dev \
        libuv1-dev \
        libjemalloc-dev \
        lld-14 \
        wget && \
        apt-get clean && \
        rm -rf /var/lib/apt/lists/*

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
        wget -q https://github.com/apple/foundationdb/releases/download/${FDB_VERSION}/foundationdb-clients_${FDB_VERFILE}_amd64.deb && \
        wget -q https://github.com/apple/foundationdb/releases/download/${FDB_VERSION}/foundationdb-server_${FDB_VERFILE}_amd64.deb && \
        dpkg -i foundationdb-clients_${FDB_VERFILE}_amd64.deb && \
        dpkg -i foundationdb-server_${FDB_VERFILE}_amd64.deb && \
        rm -rf /tmp/3fs

RUN mkdir /3fs
WORKDIR /3fs
