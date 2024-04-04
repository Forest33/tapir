#!/bin/sh

version=$(cat version)

#go install github.com/asticode/go-astilectron-bundler/astilectron-bundler
#brew install libpcap

mkdir -p distr

mkdir -p ../deploy/gui/resources
cp -R ../resources/* ../deploy/gui/resources
mv ../deploy/gui/bind.go ../deploy/gui/bind.go.tmp
mv ../deploy/gui/client ../deploy/gui/client.tmp

GOOS=darwin GOARCH=amd64 go build -C ../deploy/client -o ../gui/client -ldflags "-s -w" || exit
hash=$(md5 -q ../deploy/gui/client)

cd ../deploy/gui || exit
astilectron-bundler -c ../../bin/bundler-darwin-amd64.json -ldflags X:main.UseBootstrap=true -ldflags X:main.AppVersion="${version}" -ldflags X:main.ClientBinHash="${hash}" -ldflags "-s -w"

cp resources/icons/tray24.png ../../bin/distr/darwin-amd64/tapir.app/Contents/Resources/
rm -R resources
mv bind.go.tmp bind.go
mv client.tmp client
rm bind_darwin_amd64.go
