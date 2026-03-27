.PHONY: build clean run test install uninstall

# 获取当前系统类型
UNAME := $(shell uname | tr '[:upper:]' '[:lower:]')

# 安装路径配置
PREFIX ?= /usr/local
BINDIR = $(PREFIX)/bin
BINARY_NAME = update_proj

# 默认目标
all: clean build

# 编译目标
build:
	CGO_ENABLED=0 go build -o update_proj -ldflags="-s -w" -trimpath

# 测试目标
test:
	go test -v ./...

# 快速编译（开发中使用）
dev:
	go build -o update_proj

# 带优化的编译
release:
	CGO_ENABLED=0 GOOS=$(UNAME) GOARCH=amd64 go build -o update_proj -ldflags="-s -w" -trimpath

# 清理目标
clean:
	rm -f update_proj

# 运行目标
run: build
	./update_proj 

# 安装目标
install: build
	@echo "安装 $(BINARY_NAME) 到 $(BINDIR)"
	@mkdir -p $(BINDIR)
	@install -m 755 $(BINARY_NAME) $(BINDIR)/$(BINARY_NAME)
	@echo "安装完成: $(BINDIR)/$(BINARY_NAME)"

# 卸载目标
uninstall:
	@echo "卸载 $(BINDIR)/$(BINARY_NAME)"
	@rm -f $(BINDIR)/$(BINARY_NAME)
	@echo "卸载完成"