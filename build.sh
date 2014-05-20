#!/bin/bash

printf "** Building linux/386\n"
go-linux-386 build -o bin/linux-386/latencylog github.com/zerklabs/latencylog

printf "** Building linux/amd64\n"
go-linux-amd64 build -o bin/linux-amd64/latencylog github.com/zerklabs/latencylog

printf "** Building windows/386\n"
go-windows-386 build -o bin/windows-386/latencylog.exe github.com/zerklabs/latencylog

printf "** Building windows/amd64\n"
go-windows-amd64 build -o bin/windows-amd64/latencylog.exe github.com/zerklabs/latencylog
