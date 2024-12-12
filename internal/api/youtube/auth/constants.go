package auth

import "regexp"

const (
	youtubeBaseURL           = "https://www.youtube.com"
	youtubeTVURL             = youtubeBaseURL + "/tv"
	authServerCodeURL        = youtubeBaseURL + "/o/oauth2/device/code"
	authServerTokenURL       = youtubeBaseURL + "/o/oauth2/token"
	authServerRevokeTokenURL = youtubeBaseURL + "/o/oauth2/revoke"

	constStoreKey = "youtube-oauth-store"
)

var (
	regexClientIdentity = regexp.MustCompile(`clientId:"(?<client_id>[^"]+)",[^"]*?:"(?<client_secret>[^"]+)"`)
)
