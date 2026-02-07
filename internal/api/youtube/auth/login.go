package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/cufee/feedlr-yt/internal/metrics"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

func NewClient(store database.ConfigurationClient) *Client {
	return &Client{
		http:   http.DefaultClient,
		authMx: &sync.Mutex{},
		store:  store,
	}
}

func (c *Client) Close() error {
	if c.taskTicker != nil {
		c.taskTicker.Stop()
	}
	return nil
}

func (c *Client) Token(ctx context.Context) (string, error) {
	if c.authStatus != AuthStatusAuthenticated || c.authData.Token.Access == "" {
		err := errors.New("not authenticated")
		metrics.ObserveYouTubeOAuthCall("token", err)
		return "", err
	}
	if c.authData.Token.Expiration.Before(time.Now()) {
		err := c.RefreshToken(ctx)
		if err != nil {
			metrics.ObserveYouTubeOAuthCall("token", err)
			return "", err
		}
	}

	metrics.ObserveYouTubeOAuthCall("token", nil)
	return c.authData.Token.Access, nil
}

func (c *Client) AuthStatus() authStatus {
	c.authMx.Lock()
	defer c.authMx.Unlock()

	return c.authStatus
}

func (c *Client) RefreshToken(ctx context.Context) error {
	if !c.authMx.TryLock() {
		return errors.New("auth already in progress")
	}
	defer c.authMx.Unlock()

	c.authStatus = AuthStatusRefreshing
	newToken, err := c.refreshAccessToken(ctx, c.authData.Client, c.authData.Token)
	metrics.ObserveYouTubeOAuthCall("refresh_access_token", err)
	if err != nil {
		c.authStatus = AuthStatusExpired
		return err
	}
	c.authData.Token.Access = newToken.Access
	c.authData.Token.Expiration = newToken.Expiration
	c.authStatus = AuthStatusAuthenticated

	err = c.saveAuthCache(ctx, c.authData)
	metrics.ObserveYouTubeOAuthCall("save_auth_cache", err)
	if err != nil {
		log.Err(err).Msg("failed to save auth cache")
	}

	return nil
}

func (c *Client) Authenticate(ctx context.Context, skipCache bool) (<-chan struct{}, error) {
	if !skipCache {
		cache, err := c.getAuthCache(ctx)
		if err != nil && !database.IsErrNotFound(err) {
			metrics.ObserveYouTubeOAuthCall("load_auth_cache", err)
			return nil, err
		}
		if err == nil {
			log.Debug().Msg("found a token cache")
			metrics.ObserveYouTubeOAuthCall("load_auth_cache", nil)

			done := make(chan struct{})
			close(done)

			c.authData = cache
			c.authStatus = AuthStatusAuthenticated

			err := c.RefreshToken(ctx)
			if err != nil {
				metrics.ObserveYouTubeOAuthCall("authenticate_with_cache", err)
				return nil, err
			}
			metrics.ObserveYouTubeOAuthCall("authenticate_with_cache", nil)
			return done, nil
		}
	}

	log.Debug().Msg("requesting a new client ID")
	url, code, done, err := c.authenticateNewClient(ctx)
	metrics.ObserveYouTubeOAuthCall("authenticate_new_client", err)
	if err != nil {
		return nil, err
	}
	log.Info().Str("url", url).Str("code", code).Msg("Waiting for authentication")

	return done, nil
}

func (c *Client) authenticateNewClient(ctx context.Context) (string, string, <-chan struct{}, error) {
	if !c.authMx.TryLock() {
		return "", "", nil, errors.New("auth already in progress")
	}

	var err error

	c.authData = authData{}
	c.authStatus = AuthStatusStarted
	c.deviceUserCode = deviceAndUserCode{}

	c.authData.Client, err = c.getClientID(ctx)
	metrics.ObserveYouTubeOAuthCall("get_client_id", err)
	if err != nil {
		c.authStatus = AuthStatusExpired
		return "", "", nil, err
	}

	c.deviceUserCode, err = c.getDeviceAndUsercode(ctx, c.authData.Client.ID)
	metrics.ObserveYouTubeOAuthCall("get_device_user_code", err)
	if err != nil {
		c.authStatus = AuthStatusExpired
		return "", "", nil, err
	}

	done := make(chan struct{})
	go func() {
		defer c.authMx.Unlock()
		defer func() {
			done <- struct{}{}
		}()

		ctx, cancel := context.WithDeadline(context.Background(), c.deviceUserCode.ExpiresAt)
		defer cancel()

		c.authData.Token, err = c.getAccessTokens(ctx, c.deviceUserCode, c.authData.Client)
		metrics.ObserveYouTubeOAuthCall("get_access_tokens", err)
		if err != nil {
			c.authStatus = AuthStatusExpired
			log.Err(err).Msg("failed to get access tokens")
			return
		}
		c.authStatus = AuthStatusAuthenticated

		err := c.saveAuthCache(ctx, c.authData)
		metrics.ObserveYouTubeOAuthCall("save_auth_cache", err)
		if err != nil {
			log.Err(err).Msg("failed to save auth cache")
		}
	}()

	return c.deviceUserCode.VerificationURL, c.deviceUserCode.UserCode, done, nil
}

func (c *Client) getAuthCache(ctx context.Context) (authData, error) {
	config, err := c.store.GetConfiguration(ctx, constStoreKey)
	if err != nil {
		return authData{}, err
	}

	var data authData
	err = json.Unmarshal(config.Data, &data)
	if err != nil {
		return authData{}, err
	}

	return data, nil
}

func (c *Client) saveAuthCache(ctx context.Context, data authData) error {
	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = c.store.UpsertConfiguration(ctx, &models.AppConfiguration{ID: constStoreKey, Data: encoded, Version: 1})
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) getAccessTokens(ctx context.Context, deviceAndUserCode deviceAndUserCode, clientID clientID) (AuthToken, error) {
	payload := map[string]string{
		"client_id":     clientID.ID,
		"client_secret": clientID.Secret,
		"code":          deviceAndUserCode.DeviceCode,
		"grant_type":    "http://oauth.net/grant_type/device/1.0",
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return AuthToken{}, errors.Wrap(err, "failed to encode server code payload")
	}

	ticker := time.NewTicker(time.Second * time.Duration(max(1, deviceAndUserCode.Interval)))
	log.Debug().Int("interval", c.deviceUserCode.Interval).Msg("polling for auth token")

tickerLoop:
	for range ticker.C {
		req, err := http.NewRequest("POST", authServerTokenURL, bytes.NewReader(encoded))
		if err != nil {
			return AuthToken{}, errors.Wrap(err, "failed to make a server code request")
		}
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(ctx)

		reqTime := time.Now()
		res, err := c.http.Do(req)
		metrics.ObserveYouTubeOAuthCall("poll_token", err)
		if err != nil {
			return AuthToken{}, err
		}

		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			return AuthToken{}, err
		}

		var data oAuth2TokensResponse
		err = json.Unmarshal(body, &data)
		metrics.ObserveYouTubeOAuthCall("poll_token_decode", err)
		if err != nil {
			return AuthToken{}, err
		}

		switch data.Error {
		default:
			ticker.Stop()
			return AuthToken{
				Type:       data.Type,
				Scope:      data.Scope,
				Access:     data.AccessToken,
				Refresh:    data.RefreshToken,
				Expiration: reqTime.Add(time.Second * time.Duration(data.ExpiresIn)),
			}, nil

		case "access_denied":
			ticker.Stop()
			return AuthToken{}, errors.New("access denied")
		case "expired_token":
			ticker.Stop()
			return AuthToken{}, errors.New("token expired")

		case "slow_down":
			fallthrough
		case "authorization_pending":
			res.Body.Close()
			log.Debug().Str("description", data.ErrorDescription).Msg("waiting for access tokens")
			continue tickerLoop
		}
	}
	return AuthToken{}, errors.New("ticker stopped before a token was obtained")
}

func (c *Client) getClientID(ctx context.Context) (clientID, error) {
	req, err := http.NewRequest("GET", youtubeTVURL, nil)
	if err != nil {
		return clientID{}, errors.Wrap(err, "failed to make a new tv script request")
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (ChromiumStylePlatform) Cobalt/Version")
	req.Header.Set("Referer", "https://www.youtube.com/tv")
	req.Header.Set("Accept-Language", "en-US")

	res, err := c.http.Do(req)
	metrics.ObserveYouTubeOAuthCall("fetch_tv_page", err)
	if err != nil {
		return clientID{}, errors.Wrap(err, "failed to fetch tv script body")
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return clientID{}, errors.New("failed to read tv script response body")
	}

	document, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return clientID{}, errors.Wrap(err, "failed to parse document")
	}

	script := document.Find(`script[id="base-js"]`)
	if script == nil {
		println(string(body))
		return clientID{}, errors.New("failed to find a tv script in response body")
	}

	src, exists := script.Attr("src")
	if !exists {
		return clientID{}, errors.New("failed to find a tv script src in response body")
	}

	req, err = http.NewRequest("GET", youtubeBaseURL+src, nil)
	if err != nil {
		return clientID{}, errors.Wrap(err, "failed to create a tv script content request")
	}
	req = req.WithContext(ctx)

	res, err = c.http.Do(req)
	metrics.ObserveYouTubeOAuthCall("fetch_tv_script", err)
	if err != nil {
		return clientID{}, errors.Wrap(err, "failed to get tv script content")
	}
	defer res.Body.Close()

	scriptBody, err := io.ReadAll(res.Body)
	if err != nil {
		return clientID{}, errors.Wrap(err, "failed to read tv script content")
	}

	// https://github.com/LuanRT/YouTube.js/blob/main/src/core/OAuth2.ts#L300
	// junk, id, secret
	group := regexClientIdentity.FindStringSubmatch(string(scriptBody))
	if len(group) < 3 {
		return clientID{}, errors.New("failed to find clientId in tv script body")
	}

	return clientID{ID: group[1], Secret: group[2]}, nil
}

func (c *Client) getDeviceAndUsercode(ctx context.Context, clientID string) (deviceAndUserCode, error) {
	payload := map[string]string{
		"client_id":    clientID,
		"scope":        "http://gdata.youtube.com https://www.googleapis.com/auth/youtube-paid-content",
		"device_id":    uuid.NewString(),
		"device_model": "ytlr::",
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return deviceAndUserCode{}, errors.Wrap(err, "failed to encode server code payload")
	}

	req, err := http.NewRequest("POST", authServerCodeURL, bytes.NewReader(encoded))
	if err != nil {
		return deviceAndUserCode{}, errors.Wrap(err, "failed to make a server code request")
	}
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctx)

	reqTime := time.Now()
	res, err := c.http.Do(req)
	metrics.ObserveYouTubeOAuthCall("request_device_code", err)
	if err != nil {
		return deviceAndUserCode{}, err
	}
	defer res.Body.Close()

	var body deviceAndUserCode
	err = json.NewDecoder(res.Body).Decode(&body)
	metrics.ObserveYouTubeOAuthCall("decode_device_code", err)
	if err != nil {
		return deviceAndUserCode{}, errors.Wrap(err, "failed to unmarshal server code response body")
	}

	if body.ErrorCode != "" {
		return deviceAndUserCode{}, errors.Wrap(errors.New(body.ErrorCode), "server code request returned an error")
	}

	body.ExpiresAt = reqTime.Add(time.Second * time.Duration(body.ExpiresIn))
	return body, nil
}

func (c *Client) refreshAccessToken(ctx context.Context, client clientID, token AuthToken) (AuthToken, error) {
	payload := map[string]string{
		"client_id":     client.ID,
		"client_secret": client.Secret,
		"refresh_token": token.Refresh,
		"grant_type":    "refresh_token",
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return AuthToken{}, errors.Wrap(err, "failed to encode refresh token payload")
	}

	req, err := http.NewRequest("POST", authServerTokenURL, bytes.NewReader(encoded))
	if err != nil {
		return AuthToken{}, errors.Wrap(err, "failed to make a refresh token request")
	}
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctx)

	reqTime := time.Now()
	res, err := c.http.Do(req)
	metrics.ObserveYouTubeOAuthCall("refresh_token_request", err)
	if err != nil {
		return AuthToken{}, err
	}
	defer res.Body.Close()

	var data oAuth2TokensResponse
	err = json.NewDecoder(res.Body).Decode(&data)
	metrics.ObserveYouTubeOAuthCall("refresh_token_decode", err)
	if err != nil {
		return AuthToken{}, errors.Wrap(err, "failed to decode token response")
	}

	return AuthToken{
		Type:       data.Type,
		Scope:      data.Scope,
		Access:     data.AccessToken,
		Refresh:    data.RefreshToken,
		Expiration: reqTime.Add(time.Second * time.Duration(data.ExpiresIn)),
	}, nil

}
