#!/bin/bash

array=(windows.exe windows-arm.exe linux linux-arm mac mac-arm)
chmod +x upx
for i in "${array[@]}"; do
  ./upx "binaries/$i"
done
