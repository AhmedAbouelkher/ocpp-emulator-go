#!/bin/sh

mkdir -p ./bin

GOOS=windows go build -ldflags "-s -w" -o ./bin/cp_emulator.exe *.go &
    GOOS=linux go build -ldflags "-s -w" -o ./bin/cpemu_linux *.go &
        GOOS=darwin go build -ldflags "-s -w" -o ./bin/cpemu_macos *.go &
wait
echo "Build executables for windows, linux and macOS"

cp ./bin/cpemu_macos /usr/local/bin/cpemu
echo "Copied cpemu to /usr/local/bin/cpemu"