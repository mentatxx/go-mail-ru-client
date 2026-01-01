package mailrucloud

import (
	"time"
)

// StorageUnit определяет единицы измерения размера диска
type StorageUnit int

const (
	// StorageUnitByte байты
	StorageUnitByte StorageUnit = iota
	// StorageUnitKB килобайты
	StorageUnitKB
	// StorageUnitMB мегабайты
	StorageUnitMB
	// StorageUnitGB гигабайты
	StorageUnitGB
	// StorageUnitTB терабайты
	StorageUnitTB
)

// Size определяет размер элемента в облаке
type Size struct {
	// DefaultValue значение по умолчанию в байтах
	DefaultValue int64
	// NormalizedValue нормализованное автоматически определенное значение
	NormalizedValue float64
	// NormalizedType нормализованная автоматически определенная единица измерения
	NormalizedType StorageUnit
}

// NewSize создает новый объект Size
func NewSize(sourceValue int64) *Size {
	size := &Size{
		DefaultValue: sourceValue,
	}
	size.setNormalizedValue()
	return size
}

func (s *Size) setNormalizedValue() {
	if s.DefaultValue < 1024 {
		s.NormalizedType = StorageUnitByte
		s.NormalizedValue = float64(s.DefaultValue)
	} else if s.DefaultValue >= 1024 && s.DefaultValue < 1048576 {
		s.NormalizedType = StorageUnitKB
		s.NormalizedValue = float64(s.DefaultValue) / 1024.0
	} else if s.DefaultValue >= 1048576 && s.DefaultValue < 1073741824 {
		s.NormalizedType = StorageUnitMB
		s.NormalizedValue = float64(s.DefaultValue) / 1024.0 / 1024.0
	} else if s.DefaultValue >= 1073741824 && s.DefaultValue < 1099511627776 {
		s.NormalizedType = StorageUnitGB
		s.NormalizedValue = float64(s.DefaultValue) / 1024.0 / 1024.0 / 1024.0
	} else {
		s.NormalizedType = StorageUnitTB
		s.NormalizedValue = float64(s.DefaultValue) / 1024.0 / 1024.0 / 1024.0 / 1024.0
	}
	s.NormalizedValue = float64(int(s.NormalizedValue*100)) / 100.0
}

// DiskUsage использование диска на текущем аккаунте
type DiskUsage struct {
	// Total общий размер диска
	Total *Size
	// Used используемый размер диска
	Used *Size
	// Free свободный размер диска
	Free *Size
}

// CloudStructureEntryBase базовый класс элемента структуры облака
type CloudStructureEntryBase struct {
	// Name имя элемента
	Name string
	// Size размер элемента
	Size *Size
	// FullPath полный путь элемента в облаке
	FullPath string
	// PublicLink публичная ссылка для общего доступа без аутентификации
	PublicLink string
	// FilesCount количество файлов (для папок)
	FilesCount int
	// FoldersCount количество папок (для папок)
	FoldersCount int
	// account аккаунт Mail.ru
	account *Account
	// client клиент облака
	client *CloudClient
}

// History определяет историю модификации файла
type History struct {
	// ID уникальный ID текущей истории
	ID int64 `json:"uid"`
	// LastModifiedTimeUTC время последней модификации файла в UTC
	LastModifiedTimeUTC time.Time
	// Name имя файла
	Name string `json:"name"`
	// FullPath полный путь файла в облаке
	FullPath string `json:"path"`
	// Size размер файла
	Size *Size
	// IsCurrentVersion указывает, является ли файл текущей версией истории
	IsCurrentVersion bool
	// Revision ревизия
	Revision int64 `json:"rev"`
	// Hash хеш файла для текущей модификации файла
	Hash string `json:"hash"`
	// LastModifiedTimeUnix время последней модификации файла в формате UNIX
	LastModifiedTimeUnix int64 `json:"time"`
	// SizeBytes размер файла в байтах
	SizeBytes int64 `json:"size"`
}

// ProgressChangedEventArgs аргументы события изменения прогресса
type ProgressChangedEventArgs struct {
	// ProgressPercentage процент прогресса
	ProgressPercentage int
	// State состояние прогресса
	State *ProgressChangeTaskState
}

// ProgressChangeTaskState состояние задачи, используемое для события изменения прогресса
type ProgressChangeTaskState struct {
	// TotalBytes общие байты операции
	TotalBytes *Size
	// BytesInProgress байты в процессе для текущей операции
	BytesInProgress *Size
}

// Rate информация о тарифе
type Rate struct {
	// Name имя тарифа
	Name string `json:"name"`
	// IsActive указывает, активен ли текущий тариф
	IsActive bool `json:"active"`
	// ID уникальный ID
	ID string `json:"id"`
	// IsAvailable указывает, доступен ли для включения
	IsAvailable bool `json:"available"`
	// Size дополнительный размер, который будет применен для облачного диска
	Size *Size
	// Cost информация о стоимости текущего тарифа
	Cost []*CostItem `json:"cost"`
	// SizeBytes дополнительный размер в байтах, который будет применен для облачного диска
	SizeBytes int64 `json:"size"`
}

// CostItem элемент стоимости
type CostItem struct {
	Cost          float64  `json:"cost"`
	SpecialCost   float64  `json:"special_cost"`
	Currency      string   `json:"currency"`
	Duration      *Duration `json:"duration"`
	SpecialDuration *Duration `json:"special_duration"`
	ID            string   `json:"id"`
}

// Duration длительность
type Duration struct {
	DaysCount   int `json:"days_count"`
	MonthsCount int `json:"months_count"`
}

// Rates список тарифов
type Rates struct {
	Items []*Rate `json:"body"`
}

// AuthToken данные токена авторизации
type AuthToken struct {
	Token string `json:"body"`
}

// DefaultResponse стандартный ответ от API облака
type DefaultResponse struct {
	Email  string      `json:"email"`
	Body   interface{} `json:"body"`
	Status int         `json:"status"`
}

// ShardInfo информация о шарде
type ShardInfo struct {
	Count int    `json:"count"`
	URL   string `json:"url"`
}

// ShardsList список различных типов шардов
type ShardsList struct {
	Video           []*ShardInfo `json:"video"`
	ViewDirect      []*ShardInfo `json:"view_direct"`
	WeblinkView     []*ShardInfo `json:"weblink_view"`
	WeblinkVideo    []*ShardInfo `json:"weblink_video"`
	WeblinkGet      []*ShardInfo `json:"weblink_get"`
	Stock           []*ShardInfo `json:"stock"`
	WeblinkThumbnails []*ShardInfo `json:"weblink_thumbnails"`
	Web             []*ShardInfo `json:"web"`
	Auth            []*ShardInfo `json:"auth"`
	View            []*ShardInfo `json:"view"`
	Get             []*ShardInfo `json:"get"`
	Upload          []*ShardInfo `json:"upload"`
	Thumbnails      []*ShardInfo `json:"thumbnails"`
}

// Count количество различных типов записей
type Count struct {
	Folders int `json:"folders"`
	Files   int `json:"files"`
}

// Sort параметры сортировки
type Sort struct {
	By  string `json:"by"`
	Asc bool   `json:"asc"`
}

// CloudStructureEntry определяет DTO объект элемента для структуры облака
type CloudStructureEntry struct {
	Count         *Count                `json:"count"`
	Tree          string                `json:"tree"`
	Name          string                `json:"name"`
	Grev          string                `json:"grev"`
	Size          int64                `json:"size"`
	Sort          *Sort                 `json:"sort"`
	Kind          string                `json:"kind"`
	Rev           int                   `json:"rev"`
	Type          string                `json:"type"`
	Home          string                `json:"home"`
	Weblink       string                `json:"weblink"`
	Mtime         int64                 `json:"mtime"`
	Time          int64                 `json:"time"`
	VirusScan     string                `json:"virus_scan"`
	Hash          string                `json:"hash"`
	List          []*CloudStructureEntry `json:"list"`
}

