# go-mail-ru-client

Неофициальная библиотека-клиент для работы с облаком Mail.ru на языке Go.

Это порт библиотеки [MailRuCloudClientDotNETCore](https://github.com/erastmorgan/MailRuCloudClientDotNETCore) на язык Go.

## Установка

```bash
go get github.com/cloudru/go-mail-ru-client
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

## Лицензия

MIT

