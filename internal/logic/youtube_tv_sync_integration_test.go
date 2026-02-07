package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/cufee/feedlr-yt/internal/api/youtube/lounge"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
)

type mockTVSyncStore struct {
	mu sync.Mutex

	account  *database.YouTubeTVSyncAccount
	settings *models.Setting
}

func (m *mockTVSyncStore) GetYouTubeTVSyncAccountByUserID(_ context.Context, userID string) (*database.YouTubeTVSyncAccount, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.account == nil || m.account.UserID != userID {
		return nil, sql.ErrNoRows
	}
	cp := *m.account
	cp.LoungeTokenEnc = append([]byte(nil), m.account.LoungeTokenEnc...)
	return &cp, nil
}

func (m *mockTVSyncStore) UpsertYouTubeTVSyncCredentials(_ context.Context, userID, screenID, screenName string, loungeTokenEnc []byte, secretHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.account = &database.YouTubeTVSyncAccount{
		UserID:          userID,
		ScreenID:        screenID,
		ScreenName:      screenName,
		LoungeTokenEnc:  append([]byte(nil), loungeTokenEnc...),
		EncSecretHash:   secretHash,
		SyncEnabled:     true,
		ConnectionState: tvSyncStateDisconnected,
	}
	return nil
}

func (m *mockTVSyncStore) UpdateYouTubeTVSyncLoungeToken(_ context.Context, userID, screenID, screenName string, loungeTokenEnc []byte, secretHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.account == nil || m.account.UserID != userID {
		return sql.ErrNoRows
	}
	m.account.ScreenID = screenID
	m.account.ScreenName = screenName
	m.account.LoungeTokenEnc = append([]byte(nil), loungeTokenEnc...)
	m.account.EncSecretHash = secretHash
	return nil
}

func (m *mockTVSyncStore) SetYouTubeTVSyncAccountEnabled(_ context.Context, userID string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.account == nil || m.account.UserID != userID {
		return sql.ErrNoRows
	}
	m.account.SyncEnabled = enabled
	return nil
}

func (m *mockTVSyncStore) DeleteYouTubeTVSyncAccount(_ context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.account == nil || m.account.UserID != userID {
		return sql.ErrNoRows
	}
	m.account = nil
	return nil
}

func (m *mockTVSyncStore) ListEnabledYouTubeTVSyncAccounts(_ context.Context, _ int) ([]*database.YouTubeTVSyncAccount, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.account == nil || !m.account.SyncEnabled {
		return nil, nil
	}
	cp := *m.account
	cp.LoungeTokenEnc = append([]byte(nil), m.account.LoungeTokenEnc...)
	return []*database.YouTubeTVSyncAccount{&cp}, nil
}

func (m *mockTVSyncStore) UpdateYouTubeTVSyncState(_ context.Context, userID string, update database.YouTubeTVSyncStateUpdate) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.account == nil || m.account.UserID != userID {
		return sql.ErrNoRows
	}
	m.account.ConnectionState = update.ConnectionState
	m.account.StateReason = update.StateReason
	m.account.LastError = update.LastError
	if update.LastConnectedAt.Valid {
		m.account.LastConnectedAt = update.LastConnectedAt
	}
	if update.LastEventAt.Valid {
		m.account.LastEventAt = update.LastEventAt
	}
	if update.LastDisconnectAt.Valid {
		m.account.LastDisconnectAt = update.LastDisconnectAt
	}
	if update.LastUserActivity.Valid {
		m.account.LastUserActivity = update.LastUserActivity
	}
	if update.LastVideoID.Valid {
		m.account.LastVideoID = update.LastVideoID
	}
	return nil
}

func (m *mockTVSyncStore) GetUserLastSessionActivity(_ context.Context, _ string) (null.Time, error) {
	return null.Time{}, sql.ErrNoRows
}

func (m *mockTVSyncStore) GetUserViews(_ context.Context, _ string, _ ...string) ([]*models.View, error) {
	return nil, nil
}

func (m *mockTVSyncStore) GetRecentUserViews(_ context.Context, _ string, _ int) ([]*models.View, error) {
	return nil, nil
}

func (m *mockTVSyncStore) UpsertView(_ context.Context, _ *models.View) error {
	return nil
}

func (m *mockTVSyncStore) GetUserSettings(_ context.Context, _ string) (*models.Setting, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.settings == nil {
		return nil, sql.ErrNoRows
	}
	cp := *m.settings
	cp.Data = append([]byte(nil), m.settings.Data...)
	return &cp, nil
}

func (m *mockTVSyncStore) UpsertSettings(_ context.Context, settings *models.Setting) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *settings
	cp.Data = append([]byte(nil), settings.Data...)
	m.settings = &cp
	return nil
}

func TestConnectAndRunOnce_RefreshReconnectAndNoEventPause(t *testing.T) {
	const userID = "user-1"
	const screenID = "screen-1"

	secret := "test-tv-sync-secret"
	crypto := newYouTubeSyncCrypto(secret)

	encryptedToken, err := crypto.Encrypt([]byte("expired-token"), userID)
	if err != nil {
		t.Fatalf("encrypt token: %v", err)
	}

	store := &mockTVSyncStore{
		account: &database.YouTubeTVSyncAccount{
			UserID:          userID,
			ScreenID:        screenID,
			ScreenName:      "Living Room",
			LoungeTokenEnc:  encryptedToken,
			EncSecretHash:   crypto.secretHash,
			SyncEnabled:     true,
			ConnectionState: tvSyncStateDisconnected,
		},
	}

	var connectCalls atomic.Uint64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/lounge/pairing/get_lounge_token_batch":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"screens":[{"screenId":"screen-1","name":"Living Room","loungeToken":"fresh-token"}]}`))
		case "/api/lounge/bc/bind":
			if r.Method == http.MethodPost && r.URL.Query().Get("RID") == "1" {
				_ = r.ParseForm()
				token := r.FormValue("loungeIdToken")
				connectCalls.Add(1)
				if token == "expired-token" {
					w.WriteHeader(http.StatusUnauthorized)
					_, _ = w.Write([]byte("Expired"))
					return
				}
				if token != "fresh-token" {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte("unexpected token"))
					return
				}

				chunk := `[[1,["c","sid-1"]],[2,["S","gs-1"]]]`
				_, _ = fmt.Fprintf(w, "%d\n%s\n", len(chunk)+1, chunk)
				return
			}

			if r.Method == http.MethodGet {
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
				<-r.Context().Done()
				return
			}

			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	service := &YouTubeTVSyncService{
		db:                   store,
		crypto:               crypto,
		lounge:               lounge.NewClientWithBaseURL(server.Client(), server.URL+"/api/lounge"),
		noEventTimeout:       120 * time.Millisecond,
		watchdogPollInterval: 20 * time.Millisecond,
		reconnectMin:         50 * time.Millisecond,
		reconnectMax:         100 * time.Millisecond,
		metrics:              &tvSyncMetrics{},
		workers:              map[string]*tvSyncWorker{},
	}

	account, err := store.GetYouTubeTVSyncAccountByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("load account: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = service.connectAndRunOnce(ctx, account)
	if err == nil || !strings.Contains(err.Error(), "no events") {
		t.Fatalf("expected no events error, got %v", err)
	}

	finalAccount, err := store.GetYouTubeTVSyncAccountByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("load final account: %v", err)
	}
	if finalAccount.ConnectionState != tvSyncStatePausedNoEvents {
		t.Fatalf("expected connection state %s, got %s", tvSyncStatePausedNoEvents, finalAccount.ConnectionState)
	}
	if !finalAccount.LastDisconnectAt.Valid {
		t.Fatal("expected last disconnect timestamp to be set")
	}

	decrypted, err := crypto.Decrypt(finalAccount.LoungeTokenEnc, userID)
	if err != nil {
		t.Fatalf("decrypt final token: %v", err)
	}
	if string(decrypted) != "fresh-token" {
		t.Fatalf("expected refreshed token to be persisted, got %q", string(decrypted))
	}
	if connectCalls.Load() < 2 {
		t.Fatalf("expected at least 2 connect attempts (refresh flow), got %d", connectCalls.Load())
	}

	if service.metrics.connects.Load() != 1 {
		t.Fatalf("expected connect counter 1, got %d", service.metrics.connects.Load())
	}
	if service.metrics.disconnects.Load() != 1 {
		t.Fatalf("expected disconnect counter 1, got %d", service.metrics.disconnects.Load())
	}
}

func TestConnectAndRunOnce_RefreshEndpointMalformed(t *testing.T) {
	const userID = "user-2"
	secret := "test-tv-sync-secret"
	crypto := newYouTubeSyncCrypto(secret)
	encryptedToken, _ := crypto.Encrypt([]byte("expired-token"), userID)

	store := &mockTVSyncStore{
		account: &database.YouTubeTVSyncAccount{
			UserID:         userID,
			ScreenID:       "screen-2",
			ScreenName:     "Bedroom",
			LoungeTokenEnc: encryptedToken,
			EncSecretHash:  crypto.secretHash,
			SyncEnabled:    true,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/lounge/bc/bind":
			if r.Method == http.MethodPost && r.URL.Query().Get("RID") == "1" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte("Expired"))
				return
			}
		case "/api/lounge/pairing/get_lounge_token_batch":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"screens": []any{}})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	service := &YouTubeTVSyncService{
		db:      store,
		crypto:  crypto,
		lounge:  lounge.NewClientWithBaseURL(server.Client(), server.URL+"/api/lounge"),
		metrics: &tvSyncMetrics{},
		workers: map[string]*tvSyncWorker{},
	}

	account, _ := store.GetYouTubeTVSyncAccountByUserID(context.Background(), userID)
	err := service.connectAndRunOnce(context.Background(), account)
	if err == nil {
		t.Fatal("expected connectAndRunOnce to fail on malformed refresh response")
	}

	finalAccount, _ := store.GetYouTubeTVSyncAccountByUserID(context.Background(), userID)
	if finalAccount.ConnectionState != tvSyncStateError {
		t.Fatalf("expected error state, got %s", finalAccount.ConnectionState)
	}
}
