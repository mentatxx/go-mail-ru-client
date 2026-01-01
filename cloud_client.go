package mailrucloud

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ProgressChangedEventHandler обработчик события изменения прогресса
type ProgressChangedEventHandler func(sender interface{}, e *ProgressChangedEventArgs)

// CloudClient общий коннектор с API Mail.ru
type CloudClient struct {
	// Account связанный аккаунт Mail.ru
	Account *Account
	// ProgressChangedEvent событие изменения прогресса, работает только для операций загрузки и скачивания
	ProgressChangedEvent ProgressChangedEventHandler
	// cancelToken токен отмены асинхронных задач
	cancelToken context.CancelFunc
	cancelCtx   context.Context
}

// NewCloudClient создает новый экземпляр CloudClient
func NewCloudClient(account *Account) (*CloudClient, error) {
	if account == nil {
		return nil, fmt.Errorf("account не может быть nil")
	}

	ctx, cancel := context.WithCancel(context.Background())
	client := &CloudClient{
		Account: account,
		cancelToken: cancel,
		cancelCtx:   ctx,
	}

	// Проверка авторизации
	if _, err := account.CheckAuthorization(); err != nil {
		return nil, err
	}

	return client, nil
}

// NewCloudClientWithCredentials создает новый экземпляр CloudClient с учетными данными
func NewCloudClientWithCredentials(email, password string) (*CloudClient, error) {
	account := NewAccount(email, password)
	if err := account.Login(); err != nil {
		return nil, err
	}
	return NewCloudClient(account)
}

// GetFileOneTimeDirectLink предоставляет одноразовую анонимную прямую ссылку для скачивания файла
func (c *CloudClient) GetFileOneTimeDirectLink(publicLink string) (string, error) {
	if publicLink == "" || !strings.HasPrefix(publicLink, PublicLink) {
		return "", &CloudClientError{
			Message:   "Некорректная публичная ссылка",
			ErrorCode: ErrorCodePathNotExists,
		}
	}

	if err := c.checkAuthorization(); err != nil {
		return "", err
	}

	// Получение токена скачивания
	values := c.getDefaultFormDataFields()
	delete(values, "conflict")

	formData := url.Values{}
	for k, v := range values {
		formData.Set(k, fmt.Sprintf("%v", v))
	}

	req, err := http.NewRequest("POST", BaseMailRuCloud+DownloadTokenURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", UserAgent)

	resp, err := c.Account.getHttpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var tokenResp AuthToken
	if err := deserializeJSON(body, &tokenResp); err != nil {
		return "", err
	}

	shards, err := c.getShardsInfo()
	if err != nil {
		return "", err
	}

	if len(shards.WeblinkGet) == 0 {
		return "", fmt.Errorf("шарды WeblinkGet не найдены")
	}

	shardURL := shards.WeblinkGet[0].URL
	filePath := strings.Replace(publicLink, PublicLink, "", 1)
	return fmt.Sprintf("%s/%s?key=%s", shardURL, filePath, tokenResp.Token), nil
}

// Publish публикует файл или папку
func (c *CloudClient) Publish(sourceFullPath string) (*CloudStructureEntryBase, error) {
	return c.publishUnpublishInternal(sourceFullPath, true)
}

// Unpublish отменяет публикацию файла или папки
func (c *CloudClient) Unpublish(publicLink string) (*CloudStructureEntryBase, error) {
	return c.publishUnpublishInternal(publicLink, false)
}

// RestoreFileFromHistory восстанавливает файл из истории
func (c *CloudClient) RestoreFileFromHistory(sourceFullPath string, historyRevision int64, rewriteExisting bool, newFileName string) (*File, error) {
	if historyRevision <= 0 {
		return nil, &CloudClientError{
			Message:   "Ревизия должна быть больше 0",
			ErrorCode: ErrorCodeHistoryNotExists,
		}
	}

	if c.Account.Has2GBUploadSizeLimit() {
		return nil, &CloudClientError{
			Message:   "Текущая операция не поддерживается для вашего аккаунта. Пожалуйста, обновите тарифный план",
			ErrorCode: ErrorCodeNotSupportedOperation,
		}
	}

	histories, err := c.GetFileHistory(sourceFullPath)
	if err != nil {
		return nil, err
	}

	var history *History
	for _, h := range histories {
		if h.Revision == historyRevision {
			history = h
			break
		}
	}

	if history == nil {
		return nil, &CloudClientError{
			Message:   "История не существует по указанному номеру ревизии",
			Source:    "historyRevision",
			ErrorCode: ErrorCodeHistoryNotExists,
		}
	}

	originalFileName := filepath.Base(sourceFullPath)
	extension := filepath.Ext(originalFileName)
	if newFileName == "" {
		newFileName = originalFileName
	} else if !strings.HasSuffix(strings.ToLower(newFileName), strings.ToLower(extension)) {
		newFileName += extension
	}

	newFullPath := sourceFullPath
	if !rewriteExisting {
		parentPath := c.getParentCloudPath(sourceFullPath)
		newFullPath = parentPath + newFileName
	}

	created, err := c.createFileOrFolder(true, newFullPath, history.Hash, history.SizeBytes, rewriteExisting)
	if err != nil {
		return nil, err
	}

	return &File{
		CloudStructureEntryBase: CloudStructureEntryBase{
			FullPath: created.NewPath,
			Name:     created.NewName,
			Size:     history.Size,
		},
		Hash:              history.Hash,
		LastModifiedTimeUTC: history.LastModifiedTimeUTC,
	}, nil
}

// GetFileHistory получает историю файла
func (c *CloudClient) GetFileHistory(sourceFullPath string) ([]*History, error) {
	if sourceFullPath == "" {
		return nil, &CloudClientError{
			Message:   "Путь не может быть пустым",
			ErrorCode: ErrorCodePathNotExists,
		}
	}

	if err := c.checkAuthorization(); err != nil {
		return nil, err
	}

	sourceFullPath = c.getPathStartEndSlash(sourceFullPath, true, false)
	values := c.getDefaultFormDataFields(sourceFullPath)
	delete(values, "conflict")

	formData := url.Values{}
	for k, v := range values {
		formData.Set(k, fmt.Sprintf("%v", v))
	}

	historyURL := fmt.Sprintf(BaseMailRuCloud+HistoryURL, sourceFullPath, c.Account.Email, c.Account.Email, c.Account.getAuthToken())
	req, err := http.NewRequest("POST", historyURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", UserAgent)

	resp, err := c.Account.getHttpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, &CloudClientError{
			Message:   "Файл по указанному пути не существует",
			Source:    "sourceFullPath",
			ErrorCode: ErrorCodePathNotExists,
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var historyList []*History
	if err := deserializeJSON(body, &historyList); err != nil {
		return nil, err
	}

	if len(historyList) > 0 {
		historyList[0].IsCurrentVersion = true
		for i := range historyList {
			historyList[i].Size = NewSize(historyList[i].SizeBytes)
			historyList[i].LastModifiedTimeUTC = time.Unix(historyList[i].LastModifiedTimeUnix, 0).UTC()
		}
	}

	return historyList, nil
}

// Remove удаляет файл или папку
func (c *CloudClient) Remove(sourceFullPath string) error {
	if sourceFullPath == "" {
		return &CloudClientError{
			Message:   "Путь не может быть пустым",
			ErrorCode: ErrorCodePathNotExists,
		}
	}

	if err := c.checkAuthorization(); err != nil {
		return err
	}

	sourceFullPath = c.getPathStartEndSlash(sourceFullPath, true, false)
	values := c.getDefaultFormDataFields(sourceFullPath)

	formData := url.Values{}
	for k, v := range values {
		formData.Set(k, fmt.Sprintf("%v", v))
	}

	req, err := http.NewRequest("POST", BaseMailRuCloud+Remove, strings.NewReader(formData.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", UserAgent)

	resp, err := c.Account.getHttpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// Rename переименовывает элемент структуры облака
func (c *CloudClient) Rename(sourceFullPath, name string) (*CloudStructureEntryBase, error) {
	if sourceFullPath == "" {
		return nil, &CloudClientError{
			Message:   "Путь не может быть пустым",
			ErrorCode: ErrorCodePathNotExists,
		}
	}

	if name == "" {
		return nil, &CloudClientError{
			Message:   "Имя не может быть пустым",
			ErrorCode: ErrorCodePathNotExists,
		}
	}

	if err := c.checkAuthorization(); err != nil {
		return nil, err
	}

	sourceFullPath = c.getPathStartEndSlash(sourceFullPath, true, false)
	item, err := c.checkUnknownItemExisting(sourceFullPath)
	if err != nil {
		return nil, err
	}

	extension := filepath.Ext(item.Name)
	if extension != "" && !strings.HasSuffix(strings.ToLower(name), strings.ToLower(extension)) {
		name += extension
	}

	values := c.getDefaultFormDataFields(sourceFullPath)
	values["name"] = name

	formData := url.Values{}
	for k, v := range values {
		formData.Set(k, fmt.Sprintf("%v", v))
	}

	req, err := http.NewRequest("POST", BaseMailRuCloud+Rename, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", UserAgent)

	resp, err := c.Account.getHttpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var newPath string
	if err := deserializeJSON(body, &newPath); err != nil {
		return nil, err
	}

	newName := filepath.Base(newPath)
	item.PublicLink = ""
	item.FullPath = newPath
	item.Name = newName

	return item, nil
}

// Copy копирует элемент структуры облака
func (c *CloudClient) Copy(sourceFullPath, destFolderPath string) (*CloudStructureEntryBase, error) {
	return c.moveOrCopyInternal(sourceFullPath, destFolderPath, false)
}

// Move перемещает элемент структуры облака
func (c *CloudClient) Move(sourceFullPath, destFolderPath string) (*CloudStructureEntryBase, error) {
	return c.moveOrCopyInternal(sourceFullPath, destFolderPath, true)
}

// CreateFolder создает все директории и поддиректории по указанному пути, если они еще не существуют
func (c *CloudClient) CreateFolder(fullFolderPath string) (*Folder, error) {
	if fullFolderPath == "" {
		return nil, &CloudClientError{
			Message:   "Путь не может быть пустым",
			ErrorCode: ErrorCodePathNotExists,
		}
	}

	if err := c.checkAuthorization(); err != nil {
		return nil, err
	}

	fullFolderPath = c.getPathStartEndSlash(fullFolderPath, true, true)
	createdFolder, err := c.createFileOrFolder(false, fullFolderPath, "", 0, false)
	if err != nil {
		return nil, err
	}

	return &Folder{
		CloudStructureEntryBase: CloudStructureEntryBase{
			Name:     createdFolder.NewName,
			FullPath: createdFolder.NewPath,
			account:  c.Account,
			client:   c,
		},
	}, nil
}

// GetFolder получает информацию о корневой папке, включая список файлов и папок
func (c *CloudClient) GetFolder(fullPath ...string) (*Folder, error) {
	if err := c.checkAuthorization(); err != nil {
		return nil, err
	}

	path := ""
	if len(fullPath) > 0 {
		path = fullPath[0]
	}

	path = c.getPathStartEndSlash(path, true, true)
	itemsListURL := fmt.Sprintf(BaseMailRuCloud+ItemsList, c.Account.getAuthToken(), path)

	req, err := http.NewRequest("GET", itemsListURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := c.Account.getHttpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var deserialized CloudStructureEntry
	if err := deserializeJSON(body, &deserialized); err != nil {
		return nil, err
	}

	publicLink := ""
	if deserialized.Weblink != "" {
		publicLink = PublicLink + deserialized.Weblink
	}

	return &Folder{
		CloudStructureEntryBase: CloudStructureEntryBase{
			FilesCount:   deserialized.Count.Files,
			FoldersCount: deserialized.Count.Folders,
			FullPath:     deserialized.Home,
			Name:         deserialized.Name,
			PublicLink:   publicLink,
			Size:         NewSize(deserialized.Size),
			account:      c.Account,
			client:       c,
		},
		Items: deserialized.List,
	}, nil
}

// checkAuthorization проверяет авторизацию
func (c *CloudClient) checkAuthorization() error {
	_, err := c.Account.CheckAuthorization()
	return err
}

// getShardsInfo получает информацию о шардах
func (c *CloudClient) getShardsInfo() (*ShardsList, error) {
	if err := c.checkAuthorization(); err != nil {
		return nil, err
	}

	dispatcherURL := fmt.Sprintf(BaseMailRuCloud+Dispatcher, c.Account.getAuthToken())
	req, err := http.NewRequest("GET", dispatcherURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := c.Account.getHttpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var shardsList ShardsList
	if err := deserializeJSON(body, &shardsList); err != nil {
		return nil, err
	}

	return &shardsList, nil
}

// getDefaultFormDataFields получает поля формы данных по умолчанию
func (c *CloudClient) getDefaultFormDataFields(sourceFullPath ...string) map[string]interface{} {
	result := map[string]interface{}{
		"conflict": "rename",
		"api":      2,
		"token":    c.Account.getAuthToken(),
		"email":    c.Account.Email,
		"x-email":  c.Account.Email,
	}

	if len(sourceFullPath) > 0 && sourceFullPath[0] != "" {
		result["home"] = sourceFullPath[0]
	}

	return result
}

// getPathStartEndSlash получает и устанавливает слэш в начале и конце пути
func (c *CloudClient) getPathStartEndSlash(path string, setAtStart, setAtEnd bool) string {
	if path == "" {
		path = ""
	}

	if setAtStart {
		path = "/" + path
	}

	if setAtEnd {
		path = path + "/"
	}

	// Замена множественных слэшей и обратных слэшей на один прямой
	re := regexp.MustCompile(`[/\\]+`)
	path = re.ReplaceAllString(path, "/")

	return path
}

// getParentCloudPath получает родительский путь облака
func (c *CloudClient) getParentCloudPath(path string) string {
	path = strings.TrimSuffix(path, "/")
	lastIndex := strings.LastIndex(path, "/")
	if lastIndex == -1 {
		return "/"
	}
	return path[:lastIndex+1]
}

// createFileOrFolder создает новый файл или папку в облаке
func (c *CloudClient) createFileOrFolder(addFile bool, path, hash string, size int64, rewriteExisting bool) (*struct {
	NewName string
	NewPath string
}, error) {
	if err := c.checkAuthorization(); err != nil {
		return nil, err
	}

	values := c.getDefaultFormDataFields(path)
	if rewriteExisting {
		values["conflict"] = "rewrite"
	} else {
		values["conflict"] = "rename"
	}

	if addFile && hash != "" && size != 0 {
		values["hash"] = hash
		values["size"] = size
	}

	operationType := "folder"
	if addFile {
		operationType = "file"
	}

	createURL := fmt.Sprintf(BaseMailRuCloud+CreateFileOrFolder, operationType)
	formData := url.Values{}
	for k, v := range values {
		formData.Set(k, fmt.Sprintf("%v", v))
	}

	req, err := http.NewRequest("POST", createURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", UserAgent)

	resp, err := c.Account.getHttpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var newPath string
	if err := deserializeJSON(body, &newPath); err != nil {
		return nil, err
	}

	newName := filepath.Base(newPath)
	return &struct {
		NewName string
		NewPath string
	}{
		NewName: newName,
		NewPath: newPath,
	}, nil
}

// moveOrCopyInternal перемещает или копирует элемент структуры облака
func (c *CloudClient) moveOrCopyInternal(sourceFullPath, destFolderPath string, move bool) (*CloudStructureEntryBase, error) {
	if sourceFullPath == "" {
		return nil, &CloudClientError{
			Message:   "Путь не может быть пустым",
			ErrorCode: ErrorCodePathNotExists,
		}
	}

	if destFolderPath == "" {
		return nil, &CloudClientError{
			Message:   "Путь назначения не может быть пустым",
			ErrorCode: ErrorCodePathNotExists,
		}
	}

	if err := c.checkAuthorization(); err != nil {
		return nil, err
	}

	sourceFullPath = c.getPathStartEndSlash(sourceFullPath, true, false)
	destFolderPath = c.getPathStartEndSlash(destFolderPath, true, false)

	item, err := c.checkUnknownItemExisting(sourceFullPath)
	if err != nil {
		return nil, err
	}

	_, err = c.GetFolder(destFolderPath)
	if err != nil {
		return nil, &CloudClientError{
			Message:   "Папка назначения не существует в облаке",
			Source:    "destFolderPath",
			ErrorCode: ErrorCodePathNotExists,
		}
	}

	values := c.getDefaultFormDataFields(sourceFullPath)
	values["folder"] = destFolderPath

	formData := url.Values{}
	for k, v := range values {
		formData.Set(k, fmt.Sprintf("%v", v))
	}

	operation := "copy"
	if move {
		operation = "move"
	}

	req, err := http.NewRequest("POST", BaseMailRuCloud+FileRequest+operation, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", UserAgent)

	resp, err := c.Account.getHttpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var newPath string
	if err := deserializeJSON(body, &newPath); err != nil {
		return nil, err
	}

	newName := filepath.Base(newPath)
	item.PublicLink = ""
	item.FullPath = newPath
	item.Name = newName

	return item, nil
}

// checkUnknownItemExisting проверяет существование неизвестного элемента структуры облака
func (c *CloudClient) checkUnknownItemExisting(sourceFullPath string) (*CloudStructureEntryBase, error) {
	parentPath := c.getParentCloudPath(sourceFullPath)
	itemName := strings.TrimSuffix(sourceFullPath, "/")
	itemName = filepath.Base(itemName)

	parentFolder, err := c.GetFolder(parentPath)
	if err != nil {
		return nil, err
	}

	// Проверка файлов
	for _, file := range parentFolder.GetFiles() {
		if file.Name == itemName {
			return &file.CloudStructureEntryBase, nil
		}
	}

	// Проверка папок
	for _, folder := range parentFolder.GetFolders() {
		if folder.Name == itemName {
			return &folder.CloudStructureEntryBase, nil
		}
	}

	return nil, &CloudClientError{
		Message:   "Исходный элемент не существует в облаке",
		Source:    "sourceFullPath",
		ErrorCode: ErrorCodePathNotExists,
	}
}

// publishUnpublishInternal публикует или отменяет публикацию файла или папки
func (c *CloudClient) publishUnpublishInternal(link string, publish bool) (*CloudStructureEntryBase, error) {
	if link == "" {
		return nil, &CloudClientError{
			Message:   "Ссылка не может быть пустой",
			ErrorCode: ErrorCodePathNotExists,
		}
	}

	if err := c.checkAuthorization(); err != nil {
		return nil, err
	}

	var item *CloudStructureEntryBase
	if publish {
		link = c.getPathStartEndSlash(link, true, false)
		var err error
		item, err = c.checkUnknownItemExisting(link)
		if err != nil {
			return nil, err
		}
	} else {
		link = strings.Replace(link, PublicLink, "", 1)
	}

	values := c.getDefaultFormDataFields(link)
	delete(values, "conflict")

	if !publish {
		delete(values, "home")
		values["weblink"] = link
	}

	formData := url.Values{}
	for k, v := range values {
		formData.Set(k, fmt.Sprintf("%v", v))
	}

	operation := "unpublish"
	if publish {
		operation = "publish"
	}

	req, err := http.NewRequest("POST", BaseMailRuCloud+FileRequest+operation, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", UserAgent)

	resp, err := c.Account.getHttpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
		errorCode := ErrorCodePathNotExists
		if !publish {
			errorCode = ErrorCodePublicLinkNotExists
		}
		return nil, &CloudClientError{
			Message:   fmt.Sprintf("Элемент по введенному %s не существует", map[bool]string{true: "пути", false: "публичной ссылке"}[publish]),
			Source:    "link",
			ErrorCode: errorCode,
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result string
	if err := deserializeJSON(body, &result); err != nil {
		return nil, err
	}

	if !publish {
		return c.checkUnknownItemExisting(result)
	}

	item.PublicLink = PublicLink + result
	return item, nil
}

// AbortAllAsyncTasks прерывает выполняющиеся асинхронные задачи
func (c *CloudClient) AbortAllAsyncTasks() {
	if c.cancelToken != nil {
		c.cancelToken()
	}
}

// UploadFile загружает файл в облако. Лимит загрузки 4GB
func (c *CloudClient) UploadFile(destFileName, sourceFilePath, destFolderPath string) (*File, error) {
	if sourceFilePath == "" {
		return nil, &CloudClientError{
			Message:   "Путь к исходному файлу не может быть пустым",
			ErrorCode: ErrorCodePathNotExists,
		}
	}

	// Открытие файла
	file, err := os.Open(sourceFilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	originalFileName := filepath.Base(sourceFilePath)
	extension := filepath.Ext(originalFileName)
	if destFileName == "" {
		destFileName = originalFileName
	} else if extension != "" && !strings.HasSuffix(strings.ToLower(destFileName), strings.ToLower(extension)) {
		destFileName += extension
	}

	return c.UploadFileFromStream(destFileName, file, destFolderPath)
}

// UploadFileFromStream загружает файл в облако из потока
func (c *CloudClient) UploadFileFromStream(destFileName string, content io.Reader, destFolderPath string) (*File, error) {
	if err := c.checkAuthorization(); err != nil {
		return nil, err
	}

	destFolderPath = c.getPathStartEndSlash(destFolderPath, true, true)

	if destFileName == "" {
		return nil, &CloudClientError{
			Message:   "Имя файла не может быть пустым",
			ErrorCode: ErrorCodePathNotExists,
		}
	}

	// Чтение содержимого в память для определения размера
	contentBytes, err := io.ReadAll(content)
	if err != nil {
		return nil, err
	}

	if len(contentBytes) == 0 {
		return nil, &CloudClientError{
			Message:   "Содержимое не может быть пустым",
			ErrorCode: ErrorCodePathNotExists,
		}
	}

	if destFolderPath == "" {
		return nil, &CloudClientError{
			Message:   "Путь к папке назначения не может быть пустым",
			ErrorCode: ErrorCodePathNotExists,
		}
	}

	_, err = c.GetFolder(destFolderPath)
	if err != nil {
		return nil, &CloudClientError{
			Message:   "Путь не существует",
			Source:    "destFolderPath",
			ErrorCode: ErrorCodePathNotExists,
		}
	}

	fileSize := int64(len(contentBytes))
	sizeLimit := int64(2048 * 1024 * 1024) // 2GB
	if !c.Account.Has2GBUploadSizeLimit() {
		sizeLimit = int64(32768 * 1024 * 1024) // 32GB
	}

	if fileSize > sizeLimit {
		return nil, &CloudClientError{
			Message:   fmt.Sprintf("Максимальный лимит размера загрузки составляет %dGB", sizeLimit/(1024*1024*1024)),
			Source:    "content",
			ErrorCode: ErrorCodeUploadingSizeLimit,
		}
	}

	shards, err := c.getShardsInfo()
	if err != nil {
		return nil, err
	}

	if len(shards.Upload) == 0 {
		return nil, fmt.Errorf("шарды Upload не найдены")
	}

	shardURL := shards.Upload[0].URL
	uploadURL := fmt.Sprintf(UploadFile, shardURL, c.Account.Email)

	req, err := http.NewRequestWithContext(c.cancelCtx, "PUT", uploadURL, bytes.NewReader(contentBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.ContentLength = fileSize

	// Отслеживание прогресса загрузки
	if c.ProgressChangedEvent != nil {
		// Простая реализация прогресса - можно улучшить
		go func() {
			c.ProgressChangedEvent(c, &ProgressChangedEventArgs{
				ProgressPercentage: 0,
				State: &ProgressChangeTaskState{
					TotalBytes:      NewSize(fileSize),
					BytesInProgress: NewSize(0),
				},
			})
		}()
	}

	resp, err := c.Account.getHttpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var hash string
	if err := deserializeJSON(body, &hash); err != nil {
		return nil, err
	}

	createdFile, err := c.createFileOrFolder(true, destFolderPath+destFileName, hash, fileSize, false)
	if err != nil {
		return nil, err
	}

	if c.ProgressChangedEvent != nil {
		c.ProgressChangedEvent(c, &ProgressChangedEventArgs{
			ProgressPercentage: 100,
			State: &ProgressChangeTaskState{
				TotalBytes:      NewSize(fileSize),
				BytesInProgress: NewSize(fileSize),
			},
		})
	}

	return &File{
		CloudStructureEntryBase: CloudStructureEntryBase{
			FullPath: createdFile.NewPath,
			Name:     createdFile.NewName,
			Size:     NewSize(fileSize),
			account:  c.Account,
			client:   c,
		},
		Hash:              hash,
		LastModifiedTimeUTC: time.Now().UTC(),
	}, nil
}

// DownloadFile скачивает файл из облака
func (c *CloudClient) DownloadFile(sourceFilePath string) (io.ReadCloser, int64, error) {
	if sourceFilePath == "" {
		return nil, 0, &CloudClientError{
			Message:   "Путь к файлу не может быть пустым",
			ErrorCode: ErrorCodePathNotExists,
		}
	}

	sourceFilePath = strings.TrimPrefix(sourceFilePath, "/")
	if err := c.checkAuthorization(); err != nil {
		return nil, 0, err
	}

	shards, err := c.getShardsInfo()
	if err != nil {
		return nil, 0, err
	}

	if len(shards.Get) == 0 {
		return nil, 0, fmt.Errorf("шарды Get не найдены")
	}

	shardURL := shards.Get[0].URL
	req, err := http.NewRequestWithContext(c.cancelCtx, "GET", shardURL+sourceFilePath, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := c.Account.getHttpClient().Do(req)
	if err != nil {
		return nil, 0, err
	}

	if resp.StatusCode == 422 {
		resp.Body.Close()
		return nil, 0, &CloudClientError{
			Message:   "Максимальный лимит размера скачивания составляет 4GB",
			Source:    "sourceFilePath",
			ErrorCode: ErrorCodeDownloadingSizeLimit,
		}
	}

	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, 0, &CloudClientError{
			Message:   "Файл не существует в облаке",
			Source:    "sourceFilePath",
			ErrorCode: ErrorCodePathNotExists,
		}
	}

	contentLength := resp.ContentLength
	if contentLength < 0 {
		contentLength = 0
	}

	return resp.Body, contentLength, nil
}

// DownloadItemsAsZIPArchive скачивает файлы и папки в ZIP архив по выбранным путям
func (c *CloudClient) DownloadItemsAsZIPArchive(filesAndFoldersPaths []string) (io.ReadCloser, int64, error) {
	if err := c.checkAuthorization(); err != nil {
		return nil, 0, err
	}

	link, err := c.GetDirectLinkZIPArchive(filesAndFoldersPaths, "")
	if err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequestWithContext(c.cancelCtx, "GET", link, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := c.Account.getHttpClient().Do(req)
	if err != nil {
		return nil, 0, err
	}

	// Вычисление примерного размера
	var contentLength int64
	if len(filesAndFoldersPaths) > 0 {
		parentPath := filesAndFoldersPaths[0]
		parentFolder, err := c.GetFolder(parentPath)
		if err == nil && parentFolder != nil {
			files := parentFolder.GetFiles()
			folders := parentFolder.GetFolders()
			for _, path := range filesAndFoldersPaths {
				for _, file := range files {
					if file.FullPath == path {
						contentLength += file.Size.DefaultValue
					}
				}
				for _, folder := range folders {
					if folder.FullPath == path {
						contentLength += folder.Size.DefaultValue
					}
				}
			}
		}
	}

	return resp.Body, contentLength, nil
}

// DownloadItemsAsZIPArchiveToStream скачивает файлы и папки в ZIP архив в поток
func (c *CloudClient) DownloadItemsAsZIPArchiveToStream(filesAndFoldersPaths []string, destStream io.Writer) error {
	stream, _, err := c.DownloadItemsAsZIPArchive(filesAndFoldersPaths)
	if err != nil {
		return err
	}
	defer stream.Close()

	_, err = io.Copy(destStream, stream)
	return err
}

// GetDirectLinkZIPArchive предоставляет анонимную прямую ссылку для скачивания ZIP архива выбранных файлов и папок
func (c *CloudClient) GetDirectLinkZIPArchive(filesAndFoldersPaths []string, destZipArchiveName string) (string, error) {
	if filesAndFoldersPaths == nil || len(filesAndFoldersPaths) == 0 {
		return "", &CloudClientError{
			Message:   "Список путей не может быть пустым",
			ErrorCode: ErrorCodePathNotExists,
		}
	}

	for _, path := range filesAndFoldersPaths {
		if path == "" || path == "/" {
			return "", &CloudClientError{
				Message:   "Один из путей пуст или указывает на домашнюю директорию",
				ErrorCode: ErrorCodePathNotExists,
			}
		}
	}

	if err := c.checkAuthorization(); err != nil {
		return "", err
	}

	if destZipArchiveName == "" {
		destZipArchiveName = fmt.Sprintf("%d", time.Now().Unix())
	}

	if !strings.HasSuffix(strings.ToLower(destZipArchiveName), ".zip") {
		destZipArchiveName += ".zip"
	}

	// Проверка общего пути
	var commonPath string
	allHasCommonPath := true
	processedPaths := make([]string, len(filesAndFoldersPaths))

	for i, path := range filesAndFoldersPaths {
		parentPath := c.getParentCloudPath(path)
		if commonPath == "" {
			commonPath = parentPath
		}
		allHasCommonPath = allHasCommonPath && (commonPath == parentPath)
		processedPaths[i] = fmt.Sprintf(`"%s"`, c.getPathStartEndSlash(path, true, false))
	}

	if !allHasCommonPath {
		return "", &CloudClientError{
			Message:   "Некоторые файлы или папки имеют разные общие пути. Все элементы должны иметь общую родительскую папку",
			Source:    "filesAndFoldersPaths",
			ErrorCode: ErrorCodeDifferentParentPaths,
		}
	}

	pathsStr := fmt.Sprintf("[%s]", strings.Join(processedPaths, ","))
	values := map[string]interface{}{
		"home_list": pathsStr,
		"name":      destZipArchiveName,
		"api":       2,
		"token":     c.Account.getAuthToken(),
		"email":     c.Account.Email,
	}

	formData := url.Values{}
	for k, v := range values {
		formData.Set(k, fmt.Sprintf("%v", v))
	}

	req, err := http.NewRequest("POST", BaseMailRuCloud+CreateZipArchive, strings.NewReader(formData.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", UserAgent)

	resp, err := c.Account.getHttpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 422 {
		return "", &CloudClientError{
			Message:   "Максимальный лимит размера скачивания составляет 4GB",
			ErrorCode: ErrorCodeDownloadingSizeLimit,
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var directLink string
	if err := deserializeJSON(body, &directLink); err != nil {
		return "", err
	}

	return directLink, nil
}

