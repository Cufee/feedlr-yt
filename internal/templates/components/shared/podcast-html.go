package shared

import (
	"html"

	"github.com/a-h/templ"
	"github.com/microcosm-cc/bluemonday"
)

var podcastHTMLPolicy = bluemonday.UGCPolicy()

// PodcastHTML renders sanitized RSS description markup.
func PodcastHTML(text string) templ.Component {
	return templ.Raw(SanitizePodcastHTML(text))
}

// SanitizePodcastHTML preserves harmless RSS formatting while removing active content.
func SanitizePodcastHTML(text string) string {
	return podcastHTMLPolicy.Sanitize(html.UnescapeString(text))
}
