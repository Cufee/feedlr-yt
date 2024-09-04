package youtube

import (
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"time"
)

var proxyLock sync.Mutex
var registeredProxies []*proxyBucket

type proxyBucket struct {
	url           *url.URL
	lastUsed      time.Time
	disabledUntil time.Time
}

func (b *proxyBucket) disableFor(duration time.Duration) {
	proxyLock.Lock()
	defer proxyLock.Unlock()

	b.disabledUntil = time.Now().Add(duration)
}

func init() {
	if raw := os.Getenv("YOUTUBE_PLAYER_PROXY"); raw != "" {
		for _, u := range strings.Split(raw, ";") {
			playerProxyURL, err := url.Parse(u)
			if err != nil {
				panic(err)
			}
			registeredProxies = append(registeredProxies, &proxyBucket{url: playerProxyURL})
		}
	}
}

func getPlayerProxy() (*proxyBucket, bool) {
	proxyLock.Lock()
	defer proxyLock.Unlock()

	if len(registeredProxies) == 0 {
		return nil, false
	}

	slices.SortFunc(registeredProxies, func(a, b *proxyBucket) int {
		return a.lastUsed.Compare(b.lastUsed)
	})

	for _, bucket := range registeredProxies {
		if bucket.disabledUntil.Before(time.Now()) {
			return bucket, true
		}
	}
	return nil, false
}
