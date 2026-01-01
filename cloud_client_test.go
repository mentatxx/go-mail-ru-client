package mailrucloud

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// TestLogin тестовый логин (должен быть установлен в переменных окружения или изменен)
	TestLogin = ""
	// TestPassword тестовый пароль (должен быть установлен в переменных окружения или изменен)
	TestPassword = ""
	// TestFolderName имя тестовой папки в облаке
	TestFolderName = "new folder"
	// TestFolderPath путь к тестовой папке в облаке
	TestFolderPath = "/" + TestFolderName
	// TestFolderPublicLink публичная ссылка тестовой папки в облаке
	TestFolderPublicLink = "https://cloud.mail.ru/public/JWXJ/xsyPB2eZU"
	// TestFileName общее имя файла
	TestFileName = "video.mp4"
	// TestUploadFilePath путь к файлу для загрузки на локальной машине
	TestUploadFilePath = "/tmp/test_upload_file.mp4"
	// TestDownloadFilePath путь к файлу для скачивания в облаке
	TestDownloadFilePath = TestFolderPath + "/" + TestFileName
	// TestHistoryCheckingFilePath путь к файлу для проверки истории в облаке
	TestHistoryCheckingFilePath = "/Новая таблица.xlsx"
)

var (
	testAccount *Account
	testClient  *CloudClient
)

// checkAuthorization проверяет авторизацию и инициализирует тестовые объекты
func checkAuthorization(t *testing.T) {
	if testAccount == nil {
		login := TestLogin
		password := TestPassword
		if login == "" {
			login = os.Getenv("MAILRU_TEST_LOGIN")
		}
		if password == "" {
			password = os.Getenv("MAILRU_TEST_PASSWORD")
		}

		if login == "" || password == "" {
			t.Skip("Пропуск теста: не указаны учетные данные (MAILRU_TEST_LOGIN и MAILRU_TEST_PASSWORD)")
			return
		}

		testAccount = NewAccount(login, password)
		require.NoError(t, testAccount.Login())

		var err error
		testClient, err = NewCloudClient(testAccount)
		require.NoError(t, err)
	}
}

func TestOneTimeDirectLink(t *testing.T) {
	checkAuthorization(t)
	if testClient == nil {
		return
	}

	file, err := testClient.Publish(TestDownloadFilePath)
	require.NoError(t, err)
	require.NotNil(t, file)

	directLink, err := testClient.GetFileOneTimeDirectLink(file.PublicLink)
	require.NoError(t, err)
	require.NotEmpty(t, directLink)

	// Проверка доступности ссылки
	resp, err := testClient.Account.getHttpClient().Get(directLink)
	if err == nil && resp != nil {
		resp.Body.Close()
		assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 300)
	}
}

func TestPublishUnpublish(t *testing.T) {
	checkAuthorization(t)
	if testClient == nil {
		return
	}

	// Тест публикации несуществующего элемента
	_, err := testClient.Publish(TestFolderPath + "/" + "nonexistent")
	assert.Error(t, err)
	if cloudErr, ok := err.(*CloudClientError); ok {
		assert.Equal(t, ErrorCodePathNotExists, cloudErr.ErrorCode)
	}

	// Тест отмены публикации несуществующей ссылки
	_, err = testClient.Unpublish("nonexistent")
	assert.Error(t, err)
	if cloudErr, ok := err.(*CloudClientError); ok {
		assert.Equal(t, ErrorCodePublicLinkNotExists, cloudErr.ErrorCode)
	}

	// Тест публикации файла
	result, err := testClient.Publish(TestDownloadFilePath)
	require.NoError(t, err)
	assert.NotEmpty(t, result.PublicLink)
	assert.Contains(t, result.PublicLink, "https://cloud.mail.ru/public/")

	// Тест отмены публикации
	result, err = testClient.Unpublish(result.PublicLink)
	require.NoError(t, err)
	assert.Empty(t, result.PublicLink)
}

func TestRates(t *testing.T) {
	checkAuthorization(t)
	if testAccount == nil {
		return
	}

	for _, rate := range testAccount.ActivatedTariffs {
		assert.NotEmpty(t, rate.Name)
		assert.NotEmpty(t, rate.ID)
		if rate.ID == "ZERO" {
			assert.Nil(t, rate.Cost)
		} else {
			assert.NotNil(t, rate.Cost)
			for _, cost := range rate.Cost {
				assert.Greater(t, cost.Cost, 0.0)
				assert.Greater(t, cost.SpecialCost, 0.0)
				assert.Equal(t, "RUR", cost.Currency)
				assert.True(t, cost.Duration.DaysCount > 0 || cost.Duration.MonthsCount > 0)
				assert.True(t, cost.SpecialDuration.DaysCount > 0 || cost.SpecialDuration.MonthsCount > 0)
				assert.NotEmpty(t, cost.ID)
			}
		}
	}
}

func TestHistory(t *testing.T) {
	checkAuthorization(t)
	if testClient == nil {
		return
	}

	// Тест получения истории несуществующего файла
	_, err := testClient.GetFileHistory(TestFolderPath + "/" + "nonexistent.txt")
	assert.Error(t, err)
	if cloudErr, ok := err.(*CloudClientError); ok {
		assert.Equal(t, ErrorCodePathNotExists, cloudErr.ErrorCode)
	}

	// Тест получения истории существующего файла (если файл существует)
	historyList, err := testClient.GetFileHistory(TestHistoryCheckingFilePath)
	if err == nil && len(historyList) > 0 {
		for _, history := range historyList {
			assert.NotEmpty(t, history.FullPath)
			assert.NotEmpty(t, history.Name)
			assert.Greater(t, history.ID, int64(0))
			assert.True(t, history.LastModifiedTimeUTC.After(time.Time{}))
			assert.Greater(t, history.Size.DefaultValue, int64(0))
		}

		// Проверка текущей версии
		if len(historyList) > 0 {
			assert.True(t, historyList[0].IsCurrentVersion)
		}

		// Тест восстановления из истории
		if !testAccount.Has2GBUploadSizeLimit() {
			lastHistory := historyList[len(historyList)-1]
			if lastHistory.Revision > 0 {
				newFileName := "restored_file"
				result, err := testClient.RestoreFileFromHistory(TestHistoryCheckingFilePath, lastHistory.Revision, false, newFileName)
				if err == nil {
					extension := filepath.Ext(TestHistoryCheckingFilePath)
					assert.Equal(t, newFileName+extension, result.Name)
					assert.Equal(t, lastHistory.Size.DefaultValue, result.Size.DefaultValue)
					assert.Equal(t, lastHistory.Hash, result.Hash)
				}
			}
		}
	}
}

func TestRemove(t *testing.T) {
	checkAuthorization(t)
	if testClient == nil {
		return
	}

	// Создание тестовой папки
	folder, err := testClient.CreateFolder(TestFolderName + "/" + "test_remove_folder")
	require.NoError(t, err)

	// Удаление папки
	err = testClient.Remove(folder.FullPath)
	require.NoError(t, err)

	// Проверка, что папка удалена
	result, err := testClient.GetFolder(folder.FullPath)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestRename(t *testing.T) {
	checkAuthorization(t)
	if testClient == nil {
		return
	}

	// Создание тестового файла
	_, err := os.Stat(TestUploadFilePath)
	if err != nil {
		t.Skip("Пропуск теста: файл для загрузки не найден")
		return
	}

	file, err := testClient.UploadFile("", TestUploadFilePath, TestFolderPath)
	require.NoError(t, err)

	// Создание тестовой папки
	folder, err := testClient.CreateFolder(TestFolderName + "/" + "test_rename_folder")
	require.NoError(t, err)

	newFileName := "renamed_file"
	newFolderName := "renamed_folder"

	// Тест переименования несуществующего элемента
	_, err = testClient.Rename(TestFolderPath+"/nonexistent", newFolderName)
	assert.Error(t, err)

	// Переименование файла
	renamedFile, err := file.Rename(newFileName)
	require.NoError(t, err)
	extension := filepath.Ext(file.Name)
	assert.Equal(t, newFileName+extension, renamedFile.Name)

	// Переименование папки
	renamedFolder, err := folder.Rename(newFolderName)
	require.NoError(t, err)
	assert.Equal(t, newFolderName, renamedFolder.Name)
}

func TestMoveCopy(t *testing.T) {
	checkAuthorization(t)
	if testClient == nil {
		return
	}

	moveCopyFolderName := "test_move_copy_folder"
	moveCopyFolderPath := TestFolderPath + "/" + moveCopyFolderName
	_, err := testClient.CreateFolder(moveCopyFolderPath)
	require.NoError(t, err)

	// Создание тестового файла
	_, err = os.Stat(TestUploadFilePath)
	if err != nil {
		t.Skip("Пропуск теста: файл для загрузки не найден")
		return
	}

	moveCopyFileName := "test_move_copy_file"
	moveCopyFile, err := testClient.UploadFile(moveCopyFileName, TestUploadFilePath, TestFolderPath)
	require.NoError(t, err)

	moveCopyToFolderPath := TestFolderPath + "/" + "test_dest_folder"
	_, err = testClient.CreateFolder(moveCopyToFolderPath)
	require.NoError(t, err)

	// Тест копирования несуществующего элемента
	_, err = testClient.Copy(TestFolderPath+"/nonexistent", moveCopyToFolderPath)
	assert.Error(t, err)

	// Копирование папки
	copiedFolder, err := testClient.Copy(moveCopyFolderPath, moveCopyToFolderPath)
	require.NoError(t, err)
	assert.Empty(t, copiedFolder.PublicLink)
	assert.Contains(t, copiedFolder.FullPath, moveCopyFolderName)

	// Перемещение папки
	movedFolder, err := testClient.Move(moveCopyFolderPath, moveCopyToFolderPath)
	require.NoError(t, err)
	assert.Empty(t, movedFolder.PublicLink)
	assert.Contains(t, movedFolder.FullPath, moveCopyFolderName)

	// Копирование файла
	extension := filepath.Ext(moveCopyFile.Name)
	copiedFile, err := testClient.Copy(moveCopyFile.FullPath, moveCopyToFolderPath)
	require.NoError(t, err)
	assert.Empty(t, copiedFile.PublicLink)
	assert.Contains(t, copiedFile.FullPath, moveCopyFileName+extension)

	// Перемещение файла
	movedFile, err := testClient.Move(moveCopyFile.FullPath, moveCopyToFolderPath)
	require.NoError(t, err)
	assert.Empty(t, movedFile.PublicLink)
	assert.Contains(t, movedFile.FullPath, moveCopyFileName)
}

func TestDownloadMultipleItemsAsZIP(t *testing.T) {
	checkAuthorization(t)
	if testClient == nil {
		return
	}

	directLink, err := testClient.GetDirectLinkZIPArchive([]string{TestDownloadFilePath}, "")
	require.NoError(t, err)
	assert.NotEmpty(t, directLink)

	// Тест с разными родительскими путями
	_, err = testClient.GetDirectLinkZIPArchive([]string{TestDownloadFilePath, TestFolderPath + "/nonexistent/nonexistent"}, "")
	assert.Error(t, err)
	if cloudErr, ok := err.(*CloudClientError); ok {
		assert.Equal(t, ErrorCodeDifferentParentPaths, cloudErr.ErrorCode)
	}
}

func TestDownloadFile(t *testing.T) {
	checkAuthorization(t)
	if testClient == nil {
		return
	}

	// Тест скачивания несуществующего файла
	_, _, err := testClient.DownloadFile(TestFolderPath + "/" + "nonexistent.txt")
	assert.Error(t, err)
	if cloudErr, ok := err.(*CloudClientError); ok {
		assert.Equal(t, ErrorCodePathNotExists, cloudErr.ErrorCode)
	}

	// Тест скачивания существующего файла
	stream, length, err := testClient.DownloadFile(TestDownloadFilePath)
	if err == nil {
		defer stream.Close()
		assert.NotNil(t, stream)
		assert.Greater(t, length, int64(0))
	}
}

func TestCreateFolder(t *testing.T) {
	checkAuthorization(t)
	if testClient == nil {
		return
	}

	newFolderName := "test_new_folder"
	result, err := testClient.CreateFolder(TestFolderPath + "/new folders test/" + newFolderName)
	require.NoError(t, err)
	assert.Equal(t, newFolderName, filepath.Base(result.FullPath))
	assert.Contains(t, result.FullPath, TestFolderPath+"/new folders test")
}

func TestUploadFile(t *testing.T) {
	checkAuthorization(t)
	if testClient == nil {
		return
	}

	// Тест загрузки в несуществующую папку
	fileInfo, err := os.Stat(TestUploadFilePath)
	if err != nil {
		t.Skip("Пропуск теста: файл для загрузки не найден")
		return
	}

	_, err = testClient.UploadFile("", TestUploadFilePath, TestFolderName+"nonexistent")
	assert.Error(t, err)
	if cloudErr, ok := err.(*CloudClientError); ok {
		assert.Equal(t, ErrorCodePathNotExists, cloudErr.ErrorCode)
	}

	// Загрузка файла
	result, err := testClient.UploadFile("", TestUploadFilePath, TestFolderPath)
	require.NoError(t, err)
	assert.Equal(t, fileInfo.Size(), result.Size.DefaultValue)
	assert.Contains(t, result.Name, filepath.Base(TestFileName))
	assert.NotEmpty(t, result.Hash)
	assert.True(t, result.LastModifiedTimeUTC.Before(time.Now().UTC()) || result.LastModifiedTimeUTC.Equal(time.Now().UTC().Truncate(time.Second)))
	assert.Empty(t, result.PublicLink)
}

func TestDiskUsage(t *testing.T) {
	checkAuthorization(t)
	if testAccount == nil {
		return
	}

	result, err := testAccount.GetDiskUsage()
	require.NoError(t, err)
	assert.Greater(t, result.Free.DefaultValue, int64(0))
	assert.Greater(t, result.Total.DefaultValue, int64(0))
	assert.Greater(t, result.Used.DefaultValue, int64(0))
	assert.True(t, result.Used.DefaultValue < result.Total.DefaultValue)
	assert.True(t, result.Free.DefaultValue < result.Total.DefaultValue)
}

func TestGetItems(t *testing.T) {
	checkAuthorization(t)
	if testClient == nil {
		return
	}

	result, err := testClient.GetFolder(TestFolderPath)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Greater(t, result.FilesCount, 0)
	assert.Greater(t, result.FoldersCount, 0)
	assert.Equal(t, TestFolderPath, result.FullPath)
	assert.Equal(t, TestFolderName, result.Name)
	assert.Equal(t, TestFolderPublicLink, result.PublicLink)
	assert.Greater(t, result.Size.DefaultValue, int64(0))
	assert.Equal(t, result.FilesCount, len(result.GetFiles()))
	assert.Equal(t, result.FoldersCount, len(result.GetFolders()))

	for _, file := range result.GetFiles() {
		assert.NotEmpty(t, file.FullPath)
		assert.NotEmpty(t, file.Hash)
		assert.True(t, file.LastModifiedTimeUTC.After(time.Time{}))
		assert.NotEmpty(t, file.Name)
		if file.PublicLink != "" {
			assert.Contains(t, file.PublicLink, "https://cloud.mail.ru/public/")
		}
	}

	for _, folder := range result.GetFolders() {
		assert.NotEmpty(t, folder.FullPath)
		assert.NotEmpty(t, folder.Name)
		if folder.PublicLink != "" {
			assert.Contains(t, folder.PublicLink, "https://cloud.mail.ru/public/")
		}
	}

	// Тест получения несуществующей папки
	result, err = testClient.GetFolder(TestFolderPath + "/nonexistent")
	require.NoError(t, err)
	assert.Nil(t, result)
}
