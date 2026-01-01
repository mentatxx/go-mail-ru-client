package mailrucloud

import (
	"io"
	"time"
)

// File тип файла на сервере
type File struct {
	CloudStructureEntryBase
	// Hash хеш файла. SHA1 + SALT
	Hash string
	// LastModifiedTimeUTC время последней модификации файла в формате UTC
	LastModifiedTimeUTC time.Time
}

// GetFileOneTimeDirectLink предоставляет одноразовую анонимную прямую ссылку для скачивания файла
func (f *File) GetFileOneTimeDirectLink() (string, error) {
	return f.client.GetFileOneTimeDirectLink(f.PublicLink)
}

// Publish публикует текущий файл
func (f *File) Publish() (*File, error) {
	result, err := f.client.Publish(f.FullPath)
	if err != nil {
		return nil, err
	}
	f.PublicLink = result.PublicLink
	return f, nil
}

// Unpublish отменяет публикацию текущего файла
func (f *File) Unpublish() (*File, error) {
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

// RestoreFileFromHistory восстанавливает файл из истории
func (f *File) RestoreFileFromHistory(historyRevision int64, rewriteExisting bool, newFileName string) (*File, error) {
	return f.client.RestoreFileFromHistory(f.FullPath, historyRevision, rewriteExisting, newFileName)
}

// GetFileHistory получает историю текущего файла
func (f *File) GetFileHistory() ([]*History, error) {
	return f.client.GetFileHistory(f.FullPath)
}

// Remove удаляет текущий файл из облака
func (f *File) Remove() error {
	return f.client.Remove(f.FullPath)
}

// Rename переименовывает текущий файл
func (f *File) Rename(newName string) (*File, error) {
	result, err := f.client.Rename(f.FullPath, newName)
	if err != nil {
		return nil, err
	}
	f.FullPath = result.FullPath
	f.Name = result.Name
	f.PublicLink = result.PublicLink
	return f, nil
}

// Copy копирует файл в другое пространство
func (f *File) Copy(destFolderPath string) (*File, error) {
	result, err := f.client.Copy(f.FullPath, destFolderPath)
	if err != nil {
		return nil, err
	}
	return &File{
		CloudStructureEntryBase: *result,
		Hash:                    f.Hash,
		LastModifiedTimeUTC:     f.LastModifiedTimeUTC,
	}, nil
}

// Move перемещает файл в другое пространство
func (f *File) Move(destFolderPath string) (*File, error) {
	result, err := f.client.Move(f.FullPath, destFolderPath)
	if err != nil {
		return nil, err
	}
	f.FullPath = result.FullPath
	f.Name = result.Name
	f.PublicLink = result.PublicLink
	return f, nil
}

// DownloadFile скачивает текущий файл из облака
func (f *File) DownloadFile(destFileName, destFolderPath string) error {
	_, _, err := f.client.DownloadFile(f.FullPath)
	return err
}

// DownloadFileToStream скачивает текущий файл из облака в поток
func (f *File) DownloadFileToStream(destStream io.Writer) error {
	stream, _, err := f.client.DownloadFile(f.FullPath)
	if err != nil {
		return err
	}
	defer stream.Close()
	_, err = io.Copy(destStream, stream)
	return err
}

// DownloadFileStream получает поток для скачивания текущего файла из облака
func (f *File) DownloadFileStream() (io.ReadCloser, int64, error) {
	return f.client.DownloadFile(f.FullPath)
}

// AbortAllAsyncTasks прерывает выполняющиеся асинхронные задачи
func (f *File) AbortAllAsyncTasks() {
	f.client.AbortAllAsyncTasks()
}
