package mailrucloud

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Folder тип папки на сервере. Объект Folder может содержать только 1 уровень подэлементов
type Folder struct {
	CloudStructureEntryBase
	// FoldersCount количество папок в этой папке в облаке
	FoldersCount int
	// FilesCount количество файлов в этой папке в облаке
	FilesCount int
	// Items список записей структуры облака
	Items []*CloudStructureEntry
	// prevDiskUsed предыдущее значение используемого облачного дискового пространства
	prevDiskUsed int64
	// lastItemsGettingTime время последнего получения элементов
	lastItemsGettingTime time.Time
}

// GetFiles получает список файлов в текущей папке
func (f *Folder) GetFiles() []*File {
	f.updateFolderInfo(false)
	if f.Items == nil {
		return []*File{}
	}

	var files []*File
	for _, item := range f.Items {
		if item.Type == "file" {
			publicLink := ""
			if item.Weblink != "" {
				publicLink = PublicLink + item.Weblink
			}
			files = append(files, &File{
				CloudStructureEntryBase: CloudStructureEntryBase{
					FullPath:   item.Home,
					Name:       item.Name,
					PublicLink: publicLink,
					Size:       NewSize(item.Size),
					account:    f.account,
					client:     f.client,
				},
				Hash:              item.Hash,
				LastModifiedTimeUTC: time.Unix(item.Mtime, 0).UTC(),
			})
		}
	}
	return files
}

// GetFolders получает список подпапок в текущей папке
func (f *Folder) GetFolders() []*Folder {
	f.updateFolderInfo(false)
	if f.Items == nil {
		return []*Folder{}
	}

	var folders []*Folder
	for _, item := range f.Items {
		if item.Type == "folder" {
			publicLink := ""
			if item.Weblink != "" {
				publicLink = PublicLink + item.Weblink
			}
			folders = append(folders, &Folder{
				CloudStructureEntryBase: CloudStructureEntryBase{
					FullPath:   item.Home,
					Name:       item.Name,
					PublicLink: publicLink,
					Size:       NewSize(item.Size),
					account:    f.account,
					client:     f.client,
				},
				FoldersCount: item.Count.Folders,
				FilesCount:   item.Count.Files,
				Items:        item.List,
			})
		}
	}
	return folders
}

// Publish публикует текущую папку
func (f *Folder) Publish() (*Folder, error) {
	result, err := f.client.Publish(f.FullPath)
	if err != nil {
		return nil, err
	}
	f.PublicLink = result.PublicLink
	return f, nil
}

// Unpublish отменяет публикацию текущей папки
func (f *Folder) Unpublish() (*Folder, error) {
	if f.PublicLink == "" {
		return f, nil
	}
	result, err := f.client.Unpublish(f.PublicLink)
	if err != nil {
		return nil, err
	}
	f.PublicLink = result.PublicLink
	return f, nil
}

// Remove удаляет текущую папку из облака
func (f *Folder) Remove() error {
	err := f.client.Remove(f.FullPath)
	if err != nil {
		return err
	}
	f.updateFolderInfo(true)
	return nil
}

// Rename переименовывает текущую папку
func (f *Folder) Rename(newName string) (*Folder, error) {
	result, err := f.client.Rename(f.FullPath, newName)
	if err != nil {
		return nil, err
	}
	f.FullPath = result.FullPath
	f.Name = result.Name
	f.PublicLink = result.PublicLink
	f.updateFolderInfo(true)
	return f, nil
}

// Copy копирует папку в другое пространство
func (f *Folder) Copy(destFolderPath string) (*Folder, error) {
	result, err := f.client.Copy(f.FullPath, destFolderPath)
	if err != nil {
		return nil, err
	}
	return &Folder{
		CloudStructureEntryBase: *result,
		FoldersCount:            f.FoldersCount,
		FilesCount:              f.FilesCount,
	}, nil
}

// Move перемещает папку в другое пространство
func (f *Folder) Move(destFolderPath string) (*Folder, error) {
	result, err := f.client.Move(f.FullPath, destFolderPath)
	if err != nil {
		return nil, err
	}
	f.FullPath = result.FullPath
	f.Name = result.Name
	f.PublicLink = result.PublicLink
	f.updateFolderInfo(true)
	return f, nil
}

// CreateFolder создает новую папку в текущей папке
func (f *Folder) CreateFolder(folderName string) (*Folder, error) {
	if strings.Contains(folderName, "/") {
		return nil, &CloudClientError{
			Message:   "Вложенные поддиректории не разрешены. Используйте CloudClient.CreateFolder вместо этого",
			ErrorCode: ErrorCodePathNotExists,
		}
	}

	result, err := f.client.CreateFolder(f.FullPath + "/" + folderName)
	if err != nil {
		return nil, err
	}
	f.updateFolderInfo(true)
	return result, nil
}

// UploadFile загружает файл в облако
func (f *Folder) UploadFile(sourceFilePath string) (*File, error) {
	result, err := f.client.UploadFile("", sourceFilePath, f.FullPath)
	if err != nil {
		return nil, err
	}
	f.updateFolderInfo(true)
	return result, nil
}

// UploadFileFromStream загружает файл в облако из потока
func (f *Folder) UploadFileFromStream(fileName string, content io.Reader) (*File, error) {
	result, err := f.client.UploadFileFromStream(fileName, content, f.FullPath)
	if err != nil {
		return nil, err
	}
	f.updateFolderInfo(false)
	return result, nil
}

// DownloadItemsAsZIPArchive скачивает файлы и папки из текущей папки в ZIP архив
func (f *Folder) DownloadItemsAsZIPArchive(fileAndFolderNames []string, destZipArchiveName, destFolderPath string) error {
	for i := range fileAndFolderNames {
		fileAndFolderNames[i] = f.FullPath + "/" + fileAndFolderNames[i]
	}
	// Создание временного файла для сохранения архива
	tmpFile, err := os.CreateTemp(destFolderPath, destZipArchiveName)
	if err != nil {
		return err
	}
	defer tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	err = f.client.DownloadItemsAsZIPArchiveToStream(fileAndFolderNames, tmpFile)
	if err != nil {
		return err
	}

	// Перемещение временного файла в нужное место
	destPath := filepath.Join(destFolderPath, destZipArchiveName)
	return os.Rename(tmpFile.Name(), destPath)
}

// DownloadFolderAsZIP скачивает текущую папку из облака как ZIP архив
func (f *Folder) DownloadFolderAsZIP(destStream io.Writer) error {
	return f.client.DownloadItemsAsZIPArchiveToStream([]string{f.FullPath}, destStream)
}

// DownloadFolderAsZIPStream получает поток для скачивания текущей папки из облака как ZIP архив
func (f *Folder) DownloadFolderAsZIPStream() (io.ReadCloser, int64, error) {
	return f.client.DownloadItemsAsZIPArchive([]string{f.FullPath})
}

// AbortAllAsyncTasks прерывает выполняющиеся асинхронные задачи
func (f *Folder) AbortAllAsyncTasks() {
	f.client.AbortAllAsyncTasks()
}

// updateFolderInfo обновляет информацию о папке, если требуется
func (f *Folder) updateFolderInfo(forceUpdate bool) {
	if f.lastItemsGettingTime.IsZero() {
		f.lastItemsGettingTime = time.Now()
	}

	diffTime := time.Since(f.lastItemsGettingTime).Seconds()
	var currentDiskSpace *DiskUsage
	var err error

	if f.Items == nil || (diffTime > 1.0 && func() bool {
		currentDiskSpace, err = f.account.GetDiskUsage()
		return err == nil && currentDiskSpace.Used.DefaultValue != f.prevDiskUsed
	}()) || forceUpdate {
		folder, err := f.client.GetFolder(f.FullPath)
		if err == nil && folder != nil {
			f.Items = folder.Items
			f.Size = folder.Size
			f.PublicLink = folder.PublicLink
			f.FilesCount = folder.FilesCount
			f.FoldersCount = folder.FoldersCount
			f.lastItemsGettingTime = time.Now()
		}
	}

	if currentDiskSpace != nil {
		f.prevDiskUsed = currentDiskSpace.Used.DefaultValue
	}
}

