package auth

import (
	"net/http"
	"sync"
	"time"

	"github.com/cufee/feedlr-yt/internal/database"
)

type Client struct {
	authStatus authStatus
	authMx     *sync.Mutex

	http *http.Client

	authData       authData
	deviceUserCode deviceAndUserCode

	taskTicker *time.Ticker

	store database.ConfigurationClient

	context *WebPlayerRequestContext
}

type AuthToken struct {
	Type       string
	Scope      string
	Access     string
	Refresh    string
	Expiration time.Time
}

type oAuth2TokensResponse struct {
	RefreshToken string `json:"refresh_token"`
	AccessToken  string `json:"access_token"`
	Scope        string `json:"scope"`
	Type         string `json:"token_type"`

	ExpiryDate string `json:"expiry_date"`
	ExpiresIn  int    `json:"expires_in"`

	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type deviceAndUserCode struct {
	DeviceCode      string    `json:"device_code"`
	ExpiresIn       int       `json:"expires_in"`
	ExpiresAt       time.Time `json:"-"`
	Interval        int       `json:"interval"`
	UserCode        string    `json:"user_code"`
	VerificationURL string    `json:"verification_url"`
	ErrorCode       string    `json:"error_code"`
}

type clientID struct {
	ID     string `json:"id"`
	Secret string `json:"secret"`
}

type authData struct {
	Token  AuthToken `json:"token"`
	Client clientID  `json:"client"`
}

type authStatus int

const (
	AuthStatusNotStarted = iota
	AuthStatusStarted
	AuthStatusPendingApproval
	AuthStatusRefreshing
	AuthStatusAuthenticated
	AuthStatusExpired
)
