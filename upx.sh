#!/bin/bash

array=(windows.exe windows-arm.exe linux linux-arm mac mac-arm)

for i in "${array[@]}"; do
  ./upx "test/$i"
done
