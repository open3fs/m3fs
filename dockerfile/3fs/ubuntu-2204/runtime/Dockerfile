FROM ubuntu:jammy-20250126

RUN apt update && \
        apt install -y \
        gdb \
        google-perftools \
        iproute2 \
        libaio1 \
        libboost-all-dev \
        libdouble-conversion3 \
        libdwarf1 \
        libgflags2.2 \
        libgoogle-glog0v5 \
        libgoogle-perftools4 \
        liblz4-1 \
        liblzma5 \
        libprocps8 \
        libssl3 \
        libunwind8 \
        libuv1 \
        libjemalloc2 \
        lld-14 \
        strace \
        wget && \
        apt-get clean && \
        rm -rf /var/lib/apt/lists/*

RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
RUN curl -LsSf https://astral.sh/uv/install.sh | sh

ARG LIBFUSE_VERSION=3.16.1
COPY ./libfuse-${LIBFUSE_VERSION}/x86_64-linux-gnu/* /usr/local/lib/

# ibdev2netdev is used by many program to get netdev from ib device
# This one is copied from Mellanox OFED driver.
COPY ibdev2netdev /usr/sbin/ibdev2netdev
COPY root_profile /root/.profile

ARG FDB_VERSION=7.3.63
ARG FDB_VERFILE=${FDB_VERSION}-1
RUN mkdir /tmp/3fs && \
        cd /tmp/3fs && \
        wget -q https://github.com/apple/foundationdb/releases/download/${FDB_VERSION}/foundationdb-clients_${FDB_VERFILE}_amd64.deb && \
        dpkg -i foundationdb-clients_${FDB_VERFILE}_amd64.deb && \
        rm -rf /tmp/3fs
