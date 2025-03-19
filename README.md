# m3fs
m3fs(Make 3FS) is the toolset designed to manage 3FS cluster.

## Environment Requirements

### Machine Types

You can test 3FS with m3fs on many different environments.

For research and function test purpose, using low-cost virtual machines with directory storage type are enough. For example:

1. Public cloud virtual machines with ethernet nic. E.g. Google Cloud **N2** instance.
1. Aliyun virtual machines with eRDMA nic. E.g. Aliyun g8i instance.

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
curl -sfL https://artifactory.open3fs.com/m3fs/getm3fs | sh -
```

### Offline Installation

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

Destroy the cluster:

```
./m3fs cluster destroy -c ./cluster.yml
```

### Online Installation

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

### Online Installation With Private Registry

This method pulling images from your private registry.

Firstly, download following images from docker hub, and upload them to your registry:

- open3f3/foundationdb:7.3.63
- open3fs/clickhouse:25.1-jammy
- open3fs/3fs:20250315

Then, create the cluster with `--registry` argument:

```
./m3fs cluster create -c cluster.yml --registry harbor.yourname.com
```

The relative path of the images in your registry are written in the images section of *cluster.yml*:

```
images:
  3fs:
    repo: "open3fs/3fs"
    tag: "20250315"
  fdb: 
    repo: "open3fs/foundationdb"
    tag: "7.3.63"
  clickhouse:
    repo: "open3fs/clickhouse"
    tag: "25.1-jammy"
```
