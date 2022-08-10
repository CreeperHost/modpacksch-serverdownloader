#!/bin/bash

array=(windows.exe windows-arm.exe linux linux-arm mac mac-arm)

wget https://github.com/upx/upx/releases/download/v3.96/upx-3.96-amd64_linux.tar.xz
tar -xf upx-3.96-amd64_linux.tar.xz "upx-3.96-amd64_linux/upx" --strip-components=1

for i in "${array[@]}"; do
  ./upx "test/$i"
done
