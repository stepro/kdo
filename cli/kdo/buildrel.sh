#!/bin/bash

VERSION=$1
if [ -z "$VERSION" ]; then
  echo >&2 Error: missing version
  exit 1
fi

mkdir -p rel

echo Building Darwin binary...
GOOS=darwin GOARCH=amd64 go build -o bin/darwin/amd64/kdo -v
echo Packaging Darwin binary...
cd bin/darwin/amd64
sudo chown 0:0 kdo
tar -czvf ../../../rel/kdo-v$VERSION-darwin-amd64.tar.gz kdo
cd ../../..

echo Building Linux binary...
GOOS=linux GOARCH=amd64 go build -o bin/linux/amd64/kdo -v
echo Packaging Linux binary...
cd bin/linux/amd64
sudo chown 0:0 kdo
tar -czvf ../../../rel/kdo-v$VERSION-linux-amd64.tar.gz kdo
cd ../../..

echo Building Windows binary...
GOOS=windows GOARCH=amd64 go build -o bin/windows/amd64/kdo.exe -v
echo Packaging Windows binary...
cd bin/windows/amd64
sudo chown 0:0 kdo.exe
zip ../../../rel/kdo-v$VERSION-windows-amd64.zip kdo.exe
cd ../../..