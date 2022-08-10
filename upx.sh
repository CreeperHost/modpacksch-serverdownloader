#!/bin/bash

array=(windows.exe windows-arm.exe linux linux-arm mac mac-arm)
#array=(windows.exe)
for i in "${array[@]}"; do
  ./upx "test/$i"
done
