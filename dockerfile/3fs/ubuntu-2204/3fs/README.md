# 3fs image

This image contains 3FS binaries and scripts. It is used to deploy a 3FS cluster.

Build command:

```
docker buildx build --builder ${BUILDER} --platform linux/amd64 -t open3fs/3fs:${VERSION} .
```

