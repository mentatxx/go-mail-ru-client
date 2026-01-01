package mailrucloud

// ErrorCode определяет коды ошибок клиента облака
type ErrorCode int

const (
	// ErrorCodeNone - код ошибки не определен
	ErrorCodeNone ErrorCode = iota
	// ErrorCodePathNotExists - путь не существует
	ErrorCodePathNotExists
	// ErrorCodeUploadingSizeLimit - превышен лимит размера загрузки
	ErrorCodeUploadingSizeLimit
	// ErrorCodeDownloadingSizeLimit - превышен лимит размера скачивания
	ErrorCodeDownloadingSizeLimit
	// ErrorCodeDifferentParentPaths - элементы имеют разные родительские папки
	ErrorCodeDifferentParentPaths
	// ErrorCodeHistoryNotExists - история файла не найдена
	ErrorCodeHistoryNotExists
	// ErrorCodeNotSupportedOperation - операция не поддерживается
	ErrorCodeNotSupportedOperation
	// ErrorCodePublicLinkNotExists - публичная ссылка не существует
	ErrorCodePublicLinkNotExists
)

// CloudClientError представляет ошибку клиента облака
type CloudClientError struct {
	Message  string
	Source   string
	ErrorCode ErrorCode
}

func (e *CloudClientError) Error() string {
	if e.Source != "" {
		return e.Message + " Source: " + e.Source
	}
	return e.Message
}

// NotAuthorizedError представляет ошибку авторизации
type NotAuthorizedError struct {
	Message string
	Source  string
}

func (e *NotAuthorizedError) Error() string {
	if e.Source != "" {
		return e.Message + " Source: " + e.Source
	}
	return e.Message
}

