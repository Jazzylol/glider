.PHONY: all build docker clean help test run

# 变量定义
APP_NAME := glider
DOCKER_IMAGE := glider-sxx
DOCKER_TAG := 0.0.1
DOCKER_FULL_NAME := $(DOCKER_IMAGE):$(DOCKER_TAG)
DOCKERFILE := Dockerfile-local

# Go 编译参数
GOOS := linux
GOARCH := amd64
CGO_ENABLED := 0
LDFLAGS := -s -w -extldflags "-static"

# 颜色输出
COLOR_RESET := \033[0m
COLOR_BOLD := \033[1m
COLOR_GREEN := \033[32m
COLOR_YELLOW := \033[33m
COLOR_BLUE := \033[34m

# 默认目标
all: docker

# 帮助信息
help:
	@echo "$(COLOR_BOLD)Glider-SXX Makefile$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_YELLOW)可用命令:$(COLOR_RESET)"
	@echo "  $(COLOR_GREEN)make build$(COLOR_RESET)        - 编译 Linux 二进制文件"
	@echo "  $(COLOR_GREEN)make docker$(COLOR_RESET)       - 构建 Docker 镜像 ($(DOCKER_FULL_NAME))"
	@echo "  $(COLOR_GREEN)make docker-build$(COLOR_RESET) - 构建 Docker 镜像（多阶段构建）"
	@echo "  $(COLOR_GREEN)make clean$(COLOR_RESET)        - 清理编译产物"
	@echo "  $(COLOR_GREEN)make test$(COLOR_RESET)         - 运行测试"
	@echo "  $(COLOR_GREEN)make run$(COLOR_RESET)          - 运行 Docker 容器（测试用）"
	@echo "  $(COLOR_GREEN)make push$(COLOR_RESET)         - 推送 Docker 镜像到仓库"
	@echo "  $(COLOR_GREEN)make all$(COLOR_RESET)          - 构建 Docker 镜像（默认）"
	@echo ""
	@echo "$(COLOR_YELLOW)配置:$(COLOR_RESET)"
	@echo "  Dockerfile:   $(DOCKERFILE)"
	@echo "  Image Name:   $(DOCKER_FULL_NAME)"
	@echo ""
	@echo "$(COLOR_YELLOW)示例:$(COLOR_RESET)"
	@echo "  make docker                           # 构建镜像"
	@echo "  make run                              # 运行容器测试"
	@echo "  make clean                            # 清理"

# 编译 Linux 二进制文件
build:
	@echo "$(COLOR_BLUE)>>> 编译 $(APP_NAME) for Linux/amd64...$(COLOR_RESET)"
	@GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=$(CGO_ENABLED) \
		go build -v -ldflags "$(LDFLAGS)" -o $(APP_NAME)
	@echo "$(COLOR_GREEN)✓ 编译完成: $(APP_NAME)$(COLOR_RESET)"
	@ls -lh $(APP_NAME)

# 构建 Docker 镜像（使用多阶段构建）
docker:
	@echo "$(COLOR_BLUE)>>> 构建 Docker 镜像: $(DOCKER_FULL_NAME)...$(COLOR_RESET)"
	@docker build -f $(DOCKERFILE) -t $(DOCKER_FULL_NAME) .
	@echo "$(COLOR_GREEN)✓ Docker 镜像构建完成: $(DOCKER_FULL_NAME)$(COLOR_RESET)"
	@docker images | grep $(DOCKER_IMAGE) | grep $(DOCKER_TAG)

# 仅构建 Docker 镜像（使用多阶段构建）
docker-build:
	@echo "$(COLOR_BLUE)>>> 使用多阶段构建 Docker 镜像: $(DOCKER_FULL_NAME)...$(COLOR_RESET)"
	@docker build -f $(DOCKERFILE) -t $(DOCKER_FULL_NAME) .
	@echo "$(COLOR_GREEN)✓ Docker 镜像构建完成: $(DOCKER_FULL_NAME)$(COLOR_RESET)"
	@docker images | grep $(DOCKER_IMAGE) | grep $(DOCKER_TAG)

# 运行测试
test:
	@echo "$(COLOR_BLUE)>>> 运行测试...$(COLOR_RESET)"
	@go test -v ./...
	@echo "$(COLOR_GREEN)✓ 测试完成$(COLOR_RESET)"

# 运行 Docker 容器（测试用）
run:
	@echo "$(COLOR_BLUE)>>> 运行 Docker 容器...$(COLOR_RESET)"
	@docker run --rm -it \
		-p 8080:8080 \
		-p 8443:8443 \
		$(DOCKER_FULL_NAME) \
		-listen :8443 \
		-serverPort 8080 \
		-verbose
	@echo "$(COLOR_GREEN)✓ 容器已停止$(COLOR_RESET)"

# 推送镜像到 Docker 仓库
push:
	@echo "$(COLOR_BLUE)>>> 推送 Docker 镜像到仓库...$(COLOR_RESET)"
	@docker push $(DOCKER_FULL_NAME)
	@echo "$(COLOR_GREEN)✓ 镜像推送完成$(COLOR_RESET)"

# 清理编译产物
clean:
	@echo "$(COLOR_YELLOW)>>> 清理编译产物...$(COLOR_RESET)"
	@rm -f $(APP_NAME)
	@echo "$(COLOR_GREEN)✓ 清理完成$(COLOR_RESET)"

# 清理 Docker 镜像
clean-docker:
	@echo "$(COLOR_YELLOW)>>> 清理 Docker 镜像...$(COLOR_RESET)"
	@docker rmi -f $(DOCKER_FULL_NAME) 2>/dev/null || true
	@echo "$(COLOR_GREEN)✓ Docker 镜像清理完成$(COLOR_RESET)"

# 完全清理
clean-all: clean clean-docker
	@echo "$(COLOR_GREEN)✓ 完全清理完成$(COLOR_RESET)"

# 显示版本信息
version:
	@echo "$(COLOR_BOLD)Version Information:$(COLOR_RESET)"
	@echo "  Docker Image: $(DOCKER_FULL_NAME)"
	@echo "  Go Version:   $(shell go version)"
	@echo "  GOOS:         $(GOOS)"
	@echo "  GOARCH:       $(GOARCH)"

# 检查依赖
deps:
	@echo "$(COLOR_BLUE)>>> 检查并下载依赖...$(COLOR_RESET)"
	@go mod download
	@go mod verify
	@echo "$(COLOR_GREEN)✓ 依赖检查完成$(COLOR_RESET)"

# 更新依赖
update-deps:
	@echo "$(COLOR_BLUE)>>> 更新依赖...$(COLOR_RESET)"
	@go get -u ./...
	@go mod tidy
	@echo "$(COLOR_GREEN)✓ 依赖更新完成$(COLOR_RESET)"

# 代码格式化
fmt:
	@echo "$(COLOR_BLUE)>>> 格式化代码...$(COLOR_RESET)"
	@go fmt ./...
	@echo "$(COLOR_GREEN)✓ 代码格式化完成$(COLOR_RESET)"

# 代码检查
lint:
	@echo "$(COLOR_BLUE)>>> 运行代码检查...$(COLOR_RESET)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "$(COLOR_YELLOW)警告: golangci-lint 未安装，跳过代码检查$(COLOR_RESET)"; \
	fi
	@echo "$(COLOR_GREEN)✓ 代码检查完成$(COLOR_RESET)"

