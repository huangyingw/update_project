#!/bin/zsh
SCRIPT=$(realpath "$0")
SCRIPTPATH=$(dirname "$SCRIPT")
cd "$SCRIPTPATH"

# 运行单元测试，如果失败则退出
go test ./... || exit 1

# 编译Go程序
echo "正在编译Go程序..."
make release

# 运行编译后的程序
echo "正在运行程序..."
./update_proj
