# 3fs-build image

This Dockerfile is used to build **open3fs/build** image. This image serves as the development environment for building 3FS components. This image is also used as the base for the runtime image and can also be used for development and testing of 3FS components.

## Contents

The image includes:
- Ubuntu Jammy (22.04) as the base
- Essential build tools (gcc-12, g++-12, clang-14, cmake, meson)
- Development libraries (boost, gtest, glog, etc.)
- Rust toolchain
- UV package manager
- LibFUSE 3.16.1
- FoundationDB client and server packages

## Build

```
docker build -t open3fs/build:yyyymmdd -f Dockerfile .
```

## How to use

1. Pull the image: `docker pull open3fs/3fs-build`

2. Create a container and mount your 3fs source code directory:

```
docker run -it -v /path/to/your/3fs/source:/3fs open3fs/3fs-build
```

3. **Maybe you need to set git safe.directory firstly**:

```
git config --global --add safe.directory /3fs
```

4. Build 3fs within the container using the provided tools and dependencies.

```
cmake -S . -B build -DCMAKE_CXX_COMPILER=clang++-14 -DCMAKE_C_COMPILER=clang-14 -DCMAKE_BUILD_TYPE=RelWithDebInfo -DCMAKE_EXPORT_COMPILE_COMMANDS=ON
cmake --build build -j $(proc)
```
