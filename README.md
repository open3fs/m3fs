# m3fs
m3fs(Make 3FS) is the toolset designed to manage 3FS cluster.

# Quick Start

## Aliyun eRDMA instances

Download m3fs:

```
mkdir m3fs
cd m3fs
curl -sfL https://artifactory.open3fs.com/m3fs/getm3fs | sh -
```

### For Offline Installation

```
./bin/m3fs config create
```

Edit *cluster.yml*:

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
    password: "R00tasd$"
  - name: node2
    host: "10.0.0.202"
    username: "root"
    password: "R00tasd$"

```
./bin/m3fs config create
```

Edit *cluster.yml* as follow (fill your servers IP and password):

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
# More lines...
```

Download docker images:

```
./m3fs artifact export  -c cluster.yml  -o ./pkg
```

Create cluster:

```
./m3fs cluster create -c ./cluster.yml -a ./pkg
```

Check mount point:

```
mount | grep 3fs
```

Finally, using your mount point.
