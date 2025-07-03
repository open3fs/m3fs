# 3fs-runtime image

This Dockerfile is used to build **open3fs/3fs** image. This is image will be used by 3fs-compose to deploy 3FS cluster.

Build command:

```
docker buildx build --builder ${BUILDER} --platform linux/amd64 -t open3fs/3fs-runtime:${VERSION} .
```
