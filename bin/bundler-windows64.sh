#!/bin/sh

version=$(cat version)

#go install github.com/asticode/go-astilectron-bundler/astilectron-bundler

mkdir -p ../deploy/gui/resources
cp -R ../resources/* ../deploy/gui/resources
mv ../deploy/gui/bind.go ../deploy/gui/bind.go.tmp
mv ../deploy/gui/client ../deploy/gui/client.tmp

GOOS=windows GOARCH=amd64 go build -C ../deploy/client -o ../gui/client -ldflags "-s -w"
hash=$(md5sum -z ../deploy/gui/client | awk '{print $1}')

cd ../deploy/gui || exit
astilectron-bundler -c ../../bin/bundler-windows64.json -ldflags X:main.UseBootstrap=true -ldflags X:main.AppVersion="${version}" -ldflags X:main.ClientBinHash="${hash}" -ldflags "-s -w"

rm -R resources
mv bind.go.tmp bind.go
mv client.tmp client
rm bind_windows_amd64.go
rm windows.syso
