# m3fs
m3fs(Make 3FS) is the toolset designed to manage [3FS](https://github.com/deepseek-ai/3FS) cluster.

## Environment Requirements

### Machine Types

You can test 3FS with m3fs on many different environments.

For research and function test purpose, using low-cost virtual machines with directory storage type are enough. For example:

1. Public cloud virtual machines with ethernet nic and at least 16GB RAM. E.g. Google Cloud **N2** instance.
1. Aliyun virtual machines with eRDMA nic and at least 16GB RAM. E.g. Aliyun g8i instance.

For performance testing, recommendation configuration are Aliyun eRDMA instances equipped with NVMe disks. E.g. i4 instances.

### OS and Docker

Currently, only support Ubuntu 22.04 LTS. The docker shipped with it (version 26.1) works fine.

Install docker in Ubuntu:

```
apt install docker.io
```

For running 3FS with virtual machine only has ethernet nic, you need to install **linux-modules-extra** package.

## Quick Start

### Download m3fs

```
mkdir m3fs
cd m3fs
curl -sfL https://artifactory.open3fs.com/m3fs/getm3fs | sh
```

### Install From Cloud Storage

> If you can not visit  Docker Hub directly.

```
./m3fs config create
```

Then, edit **cluster.yml**:

- Fill nodes IP address and SSH account
- Choose **diskType**:

```
name: "open3fs"
workDir: "/opt/3fs"
# networkType configure the network type of the cluster, can be one of the following:
# - RDMA: use RDMA network protocol
# - ERDMA: use aliyun ERDMA as RDMA network protocol
# - RXE: use linux rxe kernel module to mock RDMA network protocol
networkType: "ERDMA"
nodes:
  - name: node1
    host: "10.0.0.201"
    username: "root"
    password: "password"
  - name: node2
    host: "10.0.0.202"
    username: "root"
    password: "password"
services:
  client:
    nodes:
      - node1
    hostMountpoint: /mnt/3fs
  storage:
    nodes:
      - node1
      - node2
    # diskType configure the disk type of the storage node to use, can be one of the following:
    # - nvme: NVMe SSD
    # - dir: use a directory on the filesystem
    diskType: "dir"

# More lines goes after
```

> Delete password line if you want to use key-based authentication.

Download docker images:

```
./m3fs a download  -c cluster.yml  -o ./pkg
```

Prepare environment:

```
./m3fs cluster prepare -c cluster.yml -a ./pkg
```

Create cluster:

```
./m3fs cluster create -c ./cluster.yml
```

Check mount point:

```
# mount | grep 3fs
hf3fs.open3fs on /mnt/3fs type fuse.hf3fs (rw,nosuid,nodev,relatime,user_id=0,group_id=0,default_permissions,allow_other,max_read=1048576)
```

Now, you can use the mount point.

You can use **admin_cli.sh** to interact with mgmtd service:

```
$ /opt/3fs/admin_cli.sh list-nodes
Id     Type     Status               Hostname   Pid  Tags  LastHeartbeatTime    ConfigVersion  ReleaseVersion
1      MGMTD    PRIMARY_MGMTD        open3fs-1  1    []    N/A                  1(UPTODATE)    250228-dev-1-999999-cd564a23
100    META     HEARTBEAT_CONNECTED  open3fs-1  1    []    2025-03-19 14:39:19  1(UPTODATE)    250228-dev-1-999999-cd564a23
10001  STORAGE  HEARTBEAT_CONNECTED  open3fs-1  1    []    2025-03-19 14:39:19  1(UPTODATE)    250228-dev-1-999999-cd564a23
10002  STORAGE  HEARTBEAT_CONNECTED  open3fs-2  1    []    2025-03-19 14:39:20  1(UPTODATE)    250228-dev-1-999999-cd564a23
```

Destroy the cluster:

```
./m3fs cluster destroy -c ./cluster.yml
```

### Install From Docker Hub

This method pulling images from docker hub.

```
./m3fs config create
```

Then, edit **cluster.yml** as mentioned above.

Prepare environment:

```
./m3fs cluster prepare -c cluster.yml
```

Create cluster:

```
./m3fs cluster create -c ./cluster.yml
```

### Install From Private Registry

This method pulling images from your private registry.

Firstly, download following images from docker hub, and upload them to your registry:

- open3f3/foundationdb:7.3.63
- open3fs/clickhouse:25.1-jammy
- open3fs/3fs:YYYYMMDD

Then, generating cluster config with `--registry` argument:

```
./m3fs config create --registry harbor.yourname.com
```

You can also use private registry, by creating the cluster with `--registry` argument:

```
./m3fs cluster create -c cluster.yml --registry harbor.yourname.com
```

The relative path of the images in your registry are written in the images section of *cluster.yml*:

```
images:
  3fs:
    repo: "open3fs/3fs"
    tag: "20250327"
  fdb: 
    repo: "open3fs/foundationdb"
    tag: "7.3.63"
  clickhouse:
    repo: "open3fs/clickhouse"
    tag: "25.1-jammy"
```

### Install For Large-Scale Cluster

For large-scale deployments, m3fs supports using the **nodeGroups** property in *cluster.yml* instead of individually listing each node in the **nodes** property.

For example, to deploy a 20-node cluster, you can organize nodes into functional groups:

- A management and metadata group (3 nodes)
- A storage services group (10 nodes)
- A client group (7 nodes)

This approach simplifies configuration and management of large clusters by defining IP ranges for each node group.

```
...
# Above content are same as before
nodeGroups:
  - name: meta
    username: "root"
    ipBegin: "192.168.1.1"
    ipEnd: "192.168.1.3"
  - name: storage
    username: "root"
    ipBegin: "192.168.1.11"
    ipEnd: "192.168.1.20"
  - name: client
    username: "root"
    ipBegin: "192.168.1.31"
    ipEnd: "192.168.1.37"
services:
  client:
    containerName: 3fs-client
    nodeGroups:
      - group1
    hostMountpoint: /mnt/3fs
  storage:
    containerName: 3fs-storage
    nodeGroups:
      - storage
    diskType: "nvme"
  mgmtd:
    containerName: 3fs-mgmtd
    nodeGroups:
      - meta
  meta:
    containerName: 3fs-meta
    nodeGroups:
      - meta
  monitor:
    containerName: 3fs-monitor
    nodeGroups:
      - meta
  fdb:
    containerName: 3fs-fdb
    nodeGroups:
      - meta
  clickhouse:
    containerName: 3fs-clickhouse
    nodeGroups:
      - meta

# Following content are same as before
...
```
