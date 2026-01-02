# go-mail-ru-client

Неофициальная библиотека-клиент для работы с облаком Mail.ru на языке Go.

(!) Не работает. Используйте WebDAV https://help.mail.ru/cloud_web/app/webdav/

Это порт библиотеки [MailRuCloudClientDotNETCore](https://github.com/erastmorgan/MailRuCloudClientDotNETCore) на язык Go.

[![Go Reference](https://pkg.go.dev/badge/github.com/mentatxx/go-mail-ru-client.svg)](https://pkg.go.dev/github.com/mentatxx/go-mail-ru-client)
[![Go Report Card](https://goreportcard.com/badge/github.com/mentatxx/go-mail-ru-client)](https://goreportcard.com/report/github.com/mentatxx/go-mail-ru-client)

## Установка

```bash
go get github.com/mentatxx/go-mail-ru-client
```

## Использование

Примеры использования можно найти в тестах: `cloud_client_test.go`

## Основные компоненты

- **Account** - управление аккаунтом и авторизацией
- **CloudClient** - основной клиент для работы с API облака Mail.ru
- **File** - работа с файлами
- **Folder** - работа с папками

## Примеры

### Авторизация

```go
account := NewAccount("email@mail.ru", "password")
err := account.Login()
if err != nil {
    log.Fatal(err)
}
```

### Получение информации о диске

```go
diskUsage, err := account.GetDiskUsage()
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Total: %d, Used: %d, Free: %d\n", 
    diskUsage.Total.DefaultValue, 
    diskUsage.Used.DefaultValue, 
    diskUsage.Free.DefaultValue)
```

### Загрузка файла

```go
client := NewCloudClient(account)
file, err := client.UploadFile("test.txt", fileContent, "/")
if err != nil {
    log.Fatal(err)
}
```

### Скачивание файла

```go
stream, length, err := client.DownloadFile("/test.txt")
if err != nil {
    log.Fatal(err)
}
defer stream.Close()
```

## Разработка

### Установка Git Hooks

Для автоматической проверки форматирования перед коммитом:

```bash
make install-hooks
```

Или вручную:

```bash
cp .githooks/* .git/hooks/
chmod +x .git/hooks/*
```

### Запуск тестов

```bash
# Установите переменные окружения для тестов
export MAILRU_TEST_LOGIN="your_email@mail.ru"
export MAILRU_TEST_PASSWORD="your_password"

# Запустите тесты
go test -v ./...
```

### Форматирование кода

```bash
# Автоматическое форматирование
go fmt ./...
# или
gofmt -w .
```


## Лицензия

MIT

## Ссылки

- [Исходная .NET библиотека](https://github.com/erastmorgan/MailRuCloudClientDotNETCore)
- [Документация Go](https://pkg.go.dev/github.com/mentatxx/go-mail-ru-client)
- [Репозиторий](https://github.com/mentatxx/go-mail-ru-client)

