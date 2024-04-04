#!/bin/sh

version=$(cat ../../version)

cd ../../distr/linux-amd64 || exit
tar czvf ../../installer/linux/Tapir-"${version}"-linux-x86-64.tar.gz ./*

