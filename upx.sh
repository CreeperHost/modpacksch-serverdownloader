#!/bin/bash

array=(windows.exe linux mac)
chmod +x upx
for i in "${array[@]}"; do
  ./upx "binaries/$i"
done
