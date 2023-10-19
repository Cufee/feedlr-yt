package templates

import (
	"io"

	"github.com/a-h/templ"
)

type TemplRenderer struct {
	pages map[string]templ.Component
}

func (r *TemplRenderer) Load() error {
	return nil
}

func (r *TemplRenderer) Render(w io.Writer, page string, props interface{}, locals ...string) error {
	w.Write([]byte("Not implemented"))
	return nil
}
