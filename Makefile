.PHONY: test build clean lint fmt vet

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

# Запуск всех проверок
check: fmt vet test

