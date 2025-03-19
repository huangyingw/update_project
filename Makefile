.PHONY: build clean run test

# 默认目标
all: clean build

# 编译目标
build:
	go build -o update_proj -ldflags="-s -w" -gcflags="-N -l" -trimpath

# 测试目标
test:
	go test -v ./...

# 快速编译（开发中使用）
dev:
	go build -o update_proj

# 带优化的编译
release:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o update_proj -ldflags="-s -w" -trimpath

# 清理目标
clean:
	rm -f update_proj

# 运行目标
run: build
	./update_proj 