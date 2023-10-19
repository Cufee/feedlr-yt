package templates

import (
	"context"
	"io"

	"github.com/a-h/templ"
)

//go:generate templ generate

type renderer struct {
	layouts map[string]func(templ.Component) templ.Component
}

var FiberEngine *renderer = &renderer{}

func (r *renderer) Load() error {
	r.layouts = layouts
	return nil
}

func (r *renderer) Render(w io.Writer, _ string, component interface{}, l ...string) error {
	child, ok := component.(templ.Component)
	if !ok {
		_, err := w.Write([]byte("invalid component type, expected templ.Component"))
		return err
	}

	if len(layouts) == 0 {
		return child.Render(context.Background(), w)
	}

	layout, ok := r.layouts[l[0]]
	if !ok {
		_, err := w.Write([]byte("invalid layout name"))
		return err
	}

	return layout(child).Render(context.Background(), w)
}
