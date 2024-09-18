#!/bin/zsh
SCRIPT=$(realpath "$0")
SCRIPTPATH=$(dirname "$SCRIPT")
cd "$SCRIPTPATH"

# 运行单元测试，如果失败则退出
go test ./... || exit 1

# 运行带覆盖率的单元测试
go test ./... -cover || exit 1
