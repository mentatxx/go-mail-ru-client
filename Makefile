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
check: fmt vet test

