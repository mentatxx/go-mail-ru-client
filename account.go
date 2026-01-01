package mailrucloud

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
)

// Account определяет аккаунт Mail.ru
type Account struct {
	// Email логин как email
	Email string
	// Password пароль
	Password string
	// ActivatedTariffs список активированных тарифов для аккаунта
	ActivatedTariffs []*Rate
	// AuthToken токен авторизации
	authToken string
	// httpClient HTTP клиент
	httpClient *http.Client
	// cookies контейнер cookies
	cookies *cookiejar.Jar
}

// NewAccount создает новый экземпляр Account
func NewAccount(email, password string) *Account {
	jar, _ := cookiejar.New(nil)
	return &Account{
		Email:    email,
		Password: password,
		cookies:  jar,
	}
}

// Has2GBUploadSizeLimit возвращает true, если включен лимит размера загрузки 2GB для аккаунта
func (a *Account) Has2GBUploadSizeLimit() bool {
	for _, rate := range a.ActivatedTariffs {
		if rate.ID != "ZERO" {
			return false
		}
	}
	return true
}

// Login выполняет вход в облачный сервер
func (a *Account) Login() error {
	if err := a.checkAuthorization(true); err != nil {
		return err
	}

	// Инициализация HTTP клиента для авторизации
	a.initHttpClient(BaseMailRuAuth)

	// Авторизация
	authURL := BaseMailRuAuth + Auth
	formData := url.Values{}
	formData.Set("Login", a.Email)
	formData.Set("Domain", "mail.ru")
	formData.Set("Password", a.Password)

	req, err := http.NewRequest("POST", authURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", UserAgent)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("авторизация не удалась: статус %d", resp.StatusCode)
	}

	// Обеспечение SDC cookies
	sdcURL := BaseMailRuAuth + EnsureSdc
	req, err = http.NewRequest("GET", sdcURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err = a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("получение SDC cookies не удалось: статус %d", resp.StatusCode)
	}

	// Инициализация HTTP клиента для облака
	a.initHttpClient(BaseMailRuCloud)

	// Получение токена авторизации
	tokenURL := BaseMailRuCloud + AuthTokenURL
	req, err = http.NewRequest("GET", tokenURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err = a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var authTokenResp struct {
		Body struct {
			Token string `json:"token"`
		} `json:"body"`
	}
	if err := json.Unmarshal(body, &authTokenResp); err != nil {
		return err
	}

	a.authToken = authTokenResp.Body.Token
	if a.authToken == "" {
		return fmt.Errorf("токен не найден в ответе")
	}

	// Получение тарифов
	rates, err := a.getRates()
	if err != nil {
		return err
	}

	var activatedRates []*Rate
	for _, rate := range rates {
		if rate.IsActive {
			activatedRates = append(activatedRates, rate)
		}
	}
	a.ActivatedTariffs = activatedRates

	return nil
}

// CheckAuthorization проверяет текущую авторизацию клиента
func (a *Account) CheckAuthorization() (bool, error) {
	err := a.checkAuthorization(false)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// GetDiskUsage получает использование диска для аккаунта
func (a *Account) GetDiskUsage() (*DiskUsage, error) {
	return a.getDiskUsageInternal(true)
}

// checkAuthorization проверяет опции авторизации
func (a *Account) checkAuthorization(baseCheckout bool) error {
	if a.Email == "" {
		return &NotAuthorizedError{
			Message: "Email не определен",
			Source:  "Login",
		}
	}

	if a.Password == "" {
		return &NotAuthorizedError{
			Message: "Password не определен",
			Source:  "Password",
		}
	}

	if !baseCheckout {
		if a.cookies == nil {
			return &NotAuthorizedError{Message: "Отсутствуют cookies"}
		}

		if a.authToken == "" {
			return &NotAuthorizedError{Message: "Отсутствует токен авторизации"}
		}

		_, err := a.getDiskUsageInternal(false)
		if err != nil {
			return err
		}
	}

	return nil
}

// getDiskUsageInternal получает использование диска для аккаунта
func (a *Account) getDiskUsageInternal(checkAuthorization bool) (*DiskUsage, error) {
	if checkAuthorization {
		if err := a.checkAuthorization(false); err != nil {
			return nil, err
		}
	}

	diskSpaceURL := fmt.Sprintf(BaseMailRuCloud+DiskSpace, a.Email, a.authToken)
	req, err := http.NewRequest("GET", diskSpaceURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &NotAuthorizedError{Message: "Клиент не авторизован"}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var responseData map[string]interface{}
	if err := json.Unmarshal(body, &responseData); err != nil {
		return nil, err
	}

	bytesTotal, _ := responseData["bytes_total"].(float64)
	bytesUsed, _ := responseData["bytes_used"].(float64)

	return &DiskUsage{
		Total: NewSize(int64(bytesTotal) * 1024 * 1024),
		Used:  NewSize(int64(bytesUsed) * 1024 * 1024),
		Free:  NewSize(int64(bytesTotal-bytesUsed) * 1024 * 1024),
	}, nil
}

// getRates получает активированные тарифы
func (a *Account) getRates() ([]*Rate, error) {
	if err := a.checkAuthorization(false); err != nil {
		return nil, err
	}

	ratesURL := fmt.Sprintf(BaseMailRuCloud+RatesURL, a.Email, a.Email, a.authToken)
	req, err := http.NewRequest("GET", ratesURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var ratesResp struct {
		Body []*Rate `json:"body"`
	}
	if err := json.Unmarshal(body, &ratesResp); err != nil {
		return nil, err
	}

	var rates []*Rate
	for _, rate := range ratesResp.Body {
		rate.Size = NewSize(rate.SizeBytes)
		if rate.Name == "" {
			rate.Name = rate.ID
		}
		rates = append(rates, rate)
	}

	return rates, nil
}

// initHttpClient инициализирует HTTP клиент
func (a *Account) initHttpClient(baseURL string) {
	// Создаем новый jar, если его нет, или используем существующий
	if a.cookies == nil {
		jar, _ := cookiejar.New(nil)
		a.cookies = jar
	}

	// Создаем HTTP клиент с jar для cookies
	a.httpClient = &http.Client{
		Jar:     a.cookies,
		Timeout: 0, // Без таймаута
	}
}

// getAuthToken возвращает токен авторизации
func (a *Account) getAuthToken() string {
	return a.authToken
}

// getHttpClient возвращает HTTP клиент
func (a *Account) getHttpClient() *http.Client {
	return a.httpClient
}

// deserializeJSON десериализует JSON в объект
func deserializeJSON(data []byte, target interface{}) error {
	var resp struct {
		Body json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		// Если структура не соответствует DefaultResponse, пробуем десериализовать напрямую
		return json.Unmarshal(data, target)
	}

	if len(resp.Body) == 0 {
		return json.Unmarshal(data, target)
	}

	return json.Unmarshal(resp.Body, target)
}
