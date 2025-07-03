#!/bin/sh

# Check OS
OS_COMPATABILE=false
SUPPORTED_OS="ubuntu:22.04 openEuler:22.03 openEuler:24.03"
if [ -f /etc/os-release ]; then
  . /etc/os-release
  for os_version in $SUPPORTED_OS; do
    os_name="${os_version%%:*}"
    os_ver="${os_version##*:}"
    if [ "$ID" = "$os_name" ] && [ "$VERSION_ID" = "$os_ver" ]; then
      OS_COMPATABILE=true
      break
    fi
  done
fi

if [ "$OS_COMPATABILE" = false ]; then
  echo ""
  echo "Warning: m3fs is only supported on the following systems:"
  for os_version in $SUPPORTED_OS; do
    echo "  - $os_version" | sed 's/:/ /g'
  done
  echo "After downloading, please copy the tar.gz file to a supported system."
  echo ""
fi

if [ -z "${ARCH}" ]; then
  case "$(uname -m)" in
  x86_64)
    ARCH=amd64
    ;;
  aarch64|arm64)
    ARCH=arm64
    ;;
  *)
    echo "${ARCH}, isn't supported"
    exit 1
    ;;
  esac
fi

# # Fetch latest version
# if [ "m${VERSION}" = "m" ]; then
#   VERSION="$(curl -sL https://api.github.com/repos/open3fs/m3fs/releases |
#     grep -o 'download/v[0-9]*.[0-9]*.[0-9]*/' |
#     sort --version-sort |
#     tail -1 | awk -F'/' '{ print $2}')"
#   VERSION="${VERSION##*/}"
# fi

# if [ "m${VERSION}" = "m" ]; then
#   echo "Unable to get latest version of m3fs. Set VERSION env var and re-run. For example: export VERSION=v1.0.0"
#   echo ""
#   exit
# fi

VERSION="v0.2.0"
DOWNLOAD_URL="https://artifactory.open3fs.com/m3fs/m3fs_${VERSION}_${ARCH}.tar.gz"

echo ""
echo "Downloading m3fs ${VERSION} from ${DOWNLOAD_URL} ..."
echo ""

curl -fsLO "$DOWNLOAD_URL"
if [ $? -ne 0 ]; then
  echo ""
  echo "Failed to download m3fs ${VERSION} !"
  echo ""
  echo "Please verify the version you are trying to download."
  echo ""
  exit
fi

if [ ${OS_COMPATABILE} = true ]; then
  filename="m3fs_${VERSION}_${ARCH}.tar.gz"
  ret='0'
  command -v tar >/dev/null 2>&1 || { ret='1'; }
  if [ "$ret" -eq 0 ]; then
    tar -xzf "${filename}"
  else
    echo "m3fs ${VERSION} Download Complete!"
    echo ""
    echo "Try to unpack the ${filename} failed."
    echo "tar: command not found, please unpack the ${filename} manually."
    exit
  fi
fi

echo ""
echo "m3fs ${VERSION} Download Complete!"
echo ""
