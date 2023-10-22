package templates

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/a-h/templ"
)

//go:generate templ generate

// This should be done properly, but for now it's ok.
//go:generate go test -v "github.com/byvko-dev/youtube-app/internal/templates/gen"

type renderer struct {
	layouts map[string]func(...templ.Component) templ.Component
}

var FiberEngine *renderer = &renderer{}

func (r *renderer) Load() error {
	r.layouts = layouts
	return nil
}

func (r *renderer) Render(w io.Writer, layoutName string, component interface{}, _ ...string) error {
	layout := layouts["layouts/blank"]
	selectedLayout, ok := r.layouts[layoutName]
	if ok {
		layout = selectedLayout
	}

	// Component can be a single component or a slice of components.
	var children []templ.Component
	switch component := component.(type) {
	case templ.Component:
		children = []templ.Component{component}
	case []templ.Component:
		children = component
	default:
		_, err := w.Write([]byte("invalid component type, expected templ.Component/[]templ.Component"))
		return err
	}

	// Render the layout with the children to a buffer.
	buf := templ.GetBuffer()
	defer templ.ReleaseBuffer(buf)
	err := layout(children...).Render(templ.InitializeContext(context.Background()), buf)
	if err != nil {
		return err
	}

	// Merge head tags.
	html, err := mergeHeadTags(buf.String())
	if err != nil {
		fmt.Println(err)
		_, err = w.Write(buf.Bytes())
		return err
	}

	_, err = w.Write([]byte(html))
	return err
}

func mergeHeadTags(content string) (string, error) {
	headTags := []string{"meta", "link", "title", "style"}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		return "", err
	}

	var headTagNodes []*goquery.Selection
	for _, tag := range headTags {
		doc.Find("body").Find(tag).Each(func(i int, s *goquery.Selection) {
			headTagNodes = append(headTagNodes, s.Remove())
		})
	}

	for _, bodyNode := range headTagNodes {
		name := bodyNode.Get(0).Data
		head := doc.Find("head")
		if head.Length() == 0 {
			doc.Add("head").AppendSelection(bodyNode)
			continue
		}

		currentHeadTags := head.Find(name)
		currentHeadTags.Each(func(i int, s *goquery.Selection) {
			if shouldMergeTags(s, bodyNode) {
				s.Remove()
				return
			}
		})
		doc.Find("head").AppendSelection(bodyNode)
	}

	return doc.Html()
}

func shouldMergeTags(head, body *goquery.Selection) bool {
	return body.Get(0).Data == "title" || (body.Get(0).Data == "meta" && (head.AttrOr("property", "#h") == body.AttrOr("property", "#b") || head.AttrOr("name", "#h") == body.AttrOr("name", "#b")))
}
