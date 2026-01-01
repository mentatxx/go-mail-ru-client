package mailrucloud

const (
	// BaseMailRuCloud базовый адрес облака
	BaseMailRuCloud = "https://cloud.mail.ru"
	// BaseMailRuAuth базовый адрес авторизации Mail.ru
	BaseMailRuAuth = "https://auth.mail.ru"
	// Auth URL авторизации
	Auth = "/cgi-bin/auth"
	// EnsureSdc адрес для обеспечения SDC cookies
	EnsureSdc = "/sdc?from=https://cloud.mail.ru/home"
	// AuthTokenURL URL получения токена авторизации
	AuthTokenURL = "/api/v2/tokens/csrf"
	// DiskSpace информация о дисковом пространстве
	DiskSpace = "/api/v2/user/space?api=2&email=%s&token=%s"
	// ItemsList список элементов облака
	ItemsList = "/api/v2/folder?token=%s&home=%s"
	// PublicLink начало публичной ссылки
	PublicLink = "https://cloud.mail.ru/public/"
	// Dispatcher информация о шардах
	Dispatcher = "/api/v2/dispatcher?token=%s"
	// UploadFile ссылка загрузки файла
	UploadFile = "%s?cloud_domain=2&x-email=%s"
	// CreateFileOrFolder создание записи файла или папки в структуре облака
	CreateFileOrFolder = "/api/v2/%s/add"
	// CreateZipArchive подготовка ZIP архива для скачивания
	CreateZipArchive = "/api/v2/zip"
	// FileRequest начало любого запроса файла
	FileRequest = "/api/v2/file/"
	// Rename переименование файла или папки
	Rename = "/api/v2/file/rename"
	// Remove удаление файла или папки
	Remove = "/api/v2/file/remove"
	// HistoryURL URL истории файла
	HistoryURL = "/api/v2/file/history?home=%s&api=2&email=%s&x-email=%s&token=%s"
	// RatesURL URL тарифов
	RatesURL = "/api/v2/billing/rates?api=2&email=%s&x-email=%s&token=%s"
	// DownloadTokenURL URL токена для одноразового скачивания
	DownloadTokenURL = "/api/v2/tokens/download"
	// UserAgent User-Agent для запросов
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/67.0.3396.87 Safari/537.36"
)

