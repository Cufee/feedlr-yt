package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"
)

var GoogleAuthClientID = os.Getenv("GOOGLE_AUTH_CLIENT_ID")
var GoogleAuthRedirectURL = os.Getenv("GOOGLE_AUTH_REDIRECT_URL")
var googleHttpClient = http.Client{Timeout: time.Second}

type GoogleUserInfo struct {
	Aud     string `json:"aud"`
	Issuer  string `json:"iss"`
	Subject string `json:"sub"`

	Name          string `json:"name"`
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`

	Picture string `json:"picture"`
	Locale  string `json:"locale"`
}

func GoogleTokenInfo(token string) (GoogleUserInfo, error) {
	req, err := http.NewRequest("GET", "https://oauth2.googleapis.com/tokeninfo?id_token="+token, nil)
	if err != nil {
		return GoogleUserInfo{}, err
	}

	res, err := googleHttpClient.Do(req)
	if err != nil {
		if os.IsTimeout(err) {
			return GoogleUserInfo{}, errors.New("request timeout")
		}
		return GoogleUserInfo{}, err
	}

	var user GoogleUserInfo
	decoder := json.NewDecoder(res.Body)
	if err := decoder.Decode(&user); err != nil {
		return GoogleUserInfo{}, err
	}

	return user, nil
}

func BuildGoogleRedirectURL(session string, nonce string) string {
	return fmt.Sprintf("https://accounts.google.com/o/oauth2/v2/auth?response_type=code&client_id=%s&scope=openid&redirect_uri=%s&state=%s&nonce=%s", GoogleAuthClientID, GoogleAuthRedirectURL, session, nonce)
}
