package shared

import (
	"html"
	"net/url"
	"strings"

	"github.com/a-h/templ"
	"github.com/microcosm-cc/bluemonday"
	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"mvdan.cc/xurls/v2"
)

var (
	podcastHTMLPolicy = newPodcastHTMLPolicy()
	podcastURLPattern = xurls.Relaxed()
)

func newPodcastHTMLPolicy() *bluemonday.Policy {
	policy := bluemonday.UGCPolicy()
	policy.RequireNoFollowOnLinks(true)
	policy.RequireNoReferrerOnLinks(true)
	policy.AddTargetBlankToFullyQualifiedLinks(true)
	return policy
}

// PodcastHTML renders sanitized RSS description markup.
func PodcastHTML(text string) templ.Component {
	return templ.Raw(SanitizePodcastHTML(text))
}

// SanitizePodcastHTML preserves harmless RSS formatting while removing active content.
func SanitizePodcastHTML(text string) string {
	return podcastHTMLPolicy.Sanitize(linkifyPodcastHTML(html.UnescapeString(text)))
}

func linkifyPodcastHTML(fragment string) string {
	root := &xhtml.Node{Type: xhtml.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, err := xhtml.ParseFragment(strings.NewReader(fragment), root)
	if err != nil {
		return fragment
	}
	for _, node := range nodes {
		root.AppendChild(node)
	}
	linkifyPodcastTextNodes(root)

	var out strings.Builder
	for node := root.FirstChild; node != nil; node = node.NextSibling {
		if err := xhtml.Render(&out, node); err != nil {
			return fragment
		}
	}
	return out.String()
}

func linkifyPodcastTextNodes(node *xhtml.Node) {
	for child := node.FirstChild; child != nil; {
		next := child.NextSibling
		if child.Type == xhtml.TextNode {
			linkifyPodcastTextNode(child)
		} else if child.Type == xhtml.ElementNode && !skipPodcastLinkification(child.Data) {
			linkifyPodcastTextNodes(child)
		}
		child = next
	}
}

func skipPodcastLinkification(element string) bool {
	switch strings.ToLower(element) {
	case "a", "code", "pre", "script", "style":
		return true
	default:
		return false
	}
}

func linkifyPodcastTextNode(node *xhtml.Node) {
	matches := podcastURLPattern.FindAllStringIndex(node.Data, -1)
	if len(matches) == 0 || node.Parent == nil {
		return
	}

	last := 0
	for _, match := range matches {
		if match[0] > last {
			node.Parent.InsertBefore(&xhtml.Node{Type: xhtml.TextNode, Data: node.Data[last:match[0]]}, node)
		}
		rawURL := node.Data[match[0]:match[1]]
		if href, ok := podcastLinkURL(rawURL); ok {
			link := &xhtml.Node{
				Type: xhtml.ElementNode,
				Data: "a",
				Attr: []xhtml.Attribute{{Key: "href", Val: href}},
			}
			link.AppendChild(&xhtml.Node{Type: xhtml.TextNode, Data: rawURL})
			node.Parent.InsertBefore(link, node)
		} else {
			node.Parent.InsertBefore(&xhtml.Node{Type: xhtml.TextNode, Data: rawURL}, node)
		}
		last = match[1]
	}
	if last < len(node.Data) {
		node.Parent.InsertBefore(&xhtml.Node{Type: xhtml.TextNode, Data: node.Data[last:]}, node)
	}
	node.Parent.RemoveChild(node)
}

func podcastLinkURL(rawURL string) (string, bool) {
	href := rawURL
	parsed, err := url.Parse(href)
	if err != nil {
		return "", false
	}
	if parsed.Scheme == "" {
		href = "https://" + href
		parsed, err = url.Parse(href)
		if err != nil {
			return "", false
		}
	}
	if parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", false
	}
	return href, true
}
