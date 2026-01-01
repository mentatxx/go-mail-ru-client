.PHONY: test build clean lint fmt vet install-hooks

# Запуск тестов
test:
	go test -v ./...

# Сборка проекта
build:
	go build ./...

# Очистка
clean:
	go clean

# Линтинг
lint:
	golangci-lint run

# Проверка цикломатической сложности
cyclo:
	@if command -v gocyclo >/dev/null 2>&1; then \
		echo "Проверка цикломатической сложности (порог: 15):"; \
		gocyclo -over 15 . || true; \
		echo ""; \
		echo "Полный отчет:"; \
		gocyclo -avg .; \
	else \
		echo "gocyclo не установлен. Установите: go install github.com/fzipp/gocyclo/cmd/gocyclo@latest"; \
	fi

# Форматирование кода
fmt:
	go fmt ./...

# Проверка кода
vet:
	go vet ./...

# Установка зависимостей
deps:
	go mod download
	go mod tidy

# Установка git hooks
install-hooks:
	@if [ -d .git ]; then \
		cp .githooks/* .git/hooks/ 2>/dev/null || true; \
		chmod +x .git/hooks/*; \
		echo "Git hooks установлены"; \
	else \
		echo "Ошибка: это не git репозиторий"; \
		exit 1; \
	fi

# Запуск всех проверок
check: fmt vet cyclo test

