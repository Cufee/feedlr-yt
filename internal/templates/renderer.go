package templates

import (
	"context"
	"io"

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

	return layout(children...).Render(context.Background(), w)
}
