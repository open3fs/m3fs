FROM open3fs/3fs-runtime:20250617-arm64-openeuler2203

ARG ARTIFACTORY_PREFIX=https://artifactory.open3fs.com/3fs/bin/arm64
ARG VERSION=20250617

RUN mkdir -p /opt/3fs/bin && \
    mkdir -p /opt/3fs/etc && \
    mkdir -p /var/log/3fs && \
    mkdir -p /storage

# data_placement scripts are not updated usually, so we download a fixed version.
ARG DATA_PLACEMENT_VERSION=20250410
RUN cd /opt/3fs && \
    wget https://artifactory.open3fs.com/3fs/deploy/data_placement-${DATA_PLACEMENT_VERSION}.tar.gz && \
    tar -xzf data_placement-${DATA_PLACEMENT_VERSION}.tar.gz && \
    rm data_placement-${DATA_PLACEMENT_VERSION}.tar.gz

RUN cd /opt/3fs/bin && \
    wget ${ARTIFACTORY_PREFIX}/${VERSION}/admin_cli && \
    wget ${ARTIFACTORY_PREFIX}/${VERSION}/hf3fs-admin && \
    wget ${ARTIFACTORY_PREFIX}/${VERSION}/hf3fs_fuse_main && \
    wget ${ARTIFACTORY_PREFIX}/${VERSION}/meta_main && \
    wget ${ARTIFACTORY_PREFIX}/${VERSION}/mgmtd_main && \
    wget ${ARTIFACTORY_PREFIX}/${VERSION}/migration_main && \
    wget ${ARTIFACTORY_PREFIX}/${VERSION}/monitor_collector_main && \
    wget ${ARTIFACTORY_PREFIX}/${VERSION}/simple_example_main && \
    wget ${ARTIFACTORY_PREFIX}/${VERSION}/storage_bench && \
    wget ${ARTIFACTORY_PREFIX}/${VERSION}/storage_main && \
    chmod +x /opt/3fs/bin/* && \
    mkdir -p /usr/lib64 && \
    cd /usr/lib64 && \
    wget ${ARTIFACTORY_PREFIX}/${VERSION}/hf3fs_usrbio.so && \
    wget ${ARTIFACTORY_PREFIX}/${VERSION}/libhf3fs_api_shared.so && \
    ln -s /usr/lib64/libhf3fs_api_shared.so /usr/lib/ && \
    ln -s /usr/lib64/hf3fs_usrbio.so /usr/lib/ && \
    cd /usr/bin && \
    wget ${ARTIFACTORY_PREFIX}/${VERSION}/fio && \
    chmod +x /usr/bin/fio

RUN /root/.local/bin/uv pip install --system --no-cache -r /opt/3fs/data_placement/requirements.txt

