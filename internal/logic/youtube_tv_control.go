package logic

import (
	"context"
	"errors"
	"math"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/youtube/lounge"
	"github.com/cufee/feedlr-yt/internal/database"
)

var (
	ErrTVOffline      = errors.New("TV is offline")
	ErrInvalidTVVideo = errors.New("invalid TV video or position")
	tvVideoID         = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
)

// TVPlayerStatus describes receiver presence, independently of the persisted
// Lounge transport status shown in settings. It never exposes pairing tokens.
type TVPlayerStatus struct {
	Online     bool   `json:"online"`
	ScreenName string `json:"screenName"`
}

type tvSyncConnection struct {
	ctx     context.Context
	cancel  context.CancelFunc
	session *lounge.Session
	runtime *tvSyncRuntime
	// Serialize explicit handoffs with progress/resume decisions on the stream.
	eventsMu sync.Mutex
}

type tvSyncHandoff struct {
	videoID  string
	position int
	at       time.Time
}

func (s *YouTubeTVSyncService) registerTVConnection(userID string, connection *tvSyncConnection) {
	s.connectionsMu.Lock()
	defer s.connectionsMu.Unlock()
	if connection.ctx.Err() != nil {
		return
	}
	if s.connections == nil {
		s.connections = make(map[string]*tvSyncConnection)
	}
	if previous := s.connections[userID]; previous != nil {
		previous.cancel()
	}
	s.connections[userID] = connection
}

// A nil connection removes any current session (disable/unpair). A specific
// connection only removes itself, never a replacement installed by a worker.
func (s *YouTubeTVSyncService) removeTVConnection(userID string, connection *tvSyncConnection) {
	s.connectionsMu.Lock()
	defer s.connectionsMu.Unlock()
	current := s.connections[userID]
	if current != nil && (connection == nil || current == connection) {
		current.cancel()
		delete(s.connections, userID)
	}
}

func (s *YouTubeTVSyncService) tvConnectionOnline(connection *tvSyncConnection) bool {
	if connection == nil || connection.ctx.Err() != nil {
		return false
	}
	timeout := s.noEventTimeout
	if timeout <= 0 {
		timeout = tvSyncNoEventTimeout
	}
	connection.runtime.mu.Lock()
	defer connection.runtime.mu.Unlock()
	return connection.runtime.screenOnline && time.Since(connection.runtime.lastEventAt) < timeout
}

func (s *YouTubeTVSyncService) tvConnection(ctx context.Context, userID string) (*tvSyncConnection, string, error) {
	account, err := s.db.GetYouTubeTVSyncAccountByUserID(ctx, userID)
	if database.IsErrNotFound(err) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	if !account.SyncEnabled {
		return nil, "", nil
	}
	s.connectionsMu.Lock()
	connection := s.connections[userID]
	s.connectionsMu.Unlock()
	if connection == nil || connection.session.ScreenID != account.ScreenID || !s.tvConnectionOnline(connection) {
		return nil, "", nil
	}
	return connection, account.ScreenName, nil
}

func (s *YouTubeTVSyncService) PlayerStatus(ctx context.Context, userID string) (TVPlayerStatus, error) {
	connection, name, err := s.tvConnection(ctx, userID)
	return TVPlayerStatus{Online: connection != nil, ScreenName: name}, err
}

// SendVideo sends a single video to this user's live receiver. Pairing or
// connecting to Lounge alone is insufficient; a fresh receiver presence event
// must have been observed on the current subscription.
func (s *YouTubeTVSyncService) SendVideo(ctx context.Context, userID, videoID string, position float64) error {
	if !tvVideoID.MatchString(videoID) || math.IsNaN(position) || math.IsInf(position, 0) || position < 0 || position > math.MaxInt32 {
		return ErrInvalidTVVideo
	}
	connection, _, err := s.tvConnection(ctx, userID)
	if err != nil {
		return err
	}
	if connection == nil {
		return ErrTVOffline
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	stop := context.AfterFunc(connection.ctx, cancel)
	defer stop()
	connection.eventsMu.Lock()
	defer connection.eventsMu.Unlock()
	if !s.tvConnectionOnline(connection) {
		return ErrTVOffline
	}
	if err := s.lounge.PlayVideo(ctx, connection.session, videoID, position); err != nil {
		if errors.Is(err, lounge.ErrAuthExpired) || errors.Is(err, lounge.ErrUnknownSID) || errors.Is(err, lounge.ErrSessionGone) {
			s.removeTVConnection(userID, connection)
		}
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	connection.runtime.handoff = &tvSyncHandoff{videoID: strings.Clone(videoID), position: int(position), at: time.Now()}
	return nil
}
