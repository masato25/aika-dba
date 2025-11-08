.PHONY: build build-cli build-web clean test run-cli run-web

# 建置所有二進位檔案
build: build-cli build-web

# 建置 CLI 工具
build-cli:
	go build -o bin/aika-dba ./cmd

# 建置 Web 服務器
build-web:
	go build -o bin/webserver ./webserver

# 清理建置檔案
clean:
	rm -rf bin/

# 運行測試
test:
	go test ./...

# 運行 CLI 工具 (需要參數)
run-cli:
	@echo "使用範例:"
	@echo "  make run-cli COMMAND=prepare"
	@echo "  make run-cli COMMAND=query QUESTION='你的問題'"
	@if [ -n "$(COMMAND)" ]; then \
		if [ "$(COMMAND)" = "query" ] && [ -n "$(QUESTION)" ]; then \
			./bin/aika-dba -command $(COMMAND) -question "$(QUESTION)"; \
		else \
			./bin/aika-dba -command $(COMMAND)"; \
		fi \
	else \
		echo "請指定 COMMAND 參數"; \
		exit 1; \
	fi

# 運行 Web 服務器
run-web:
	./bin/webserver

# 運行 Web 服務器 (熱重載模式)
dev:
	./dev.sh

# 運行 Web 服務器 (熱重載模式，使用 air 直接)
dev-air:
	air

# 安裝依賴
deps:
	go mod download

# 格式化程式碼
fmt:
	go fmt ./...

# 檢查程式碼
vet:
	go vet ./...

# 檢查所有
check: fmt vet test