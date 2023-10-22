package shared

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/a-h/templ"
	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/js"
)

func EmbedScript(script templ.ComponentScript, params ...interface{}) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) (err error) {
		if _, err = io.WriteString(w, `<script type="text/javascript">`+"\r\n"+script.Function+"\r\n"+script.Name+"("); err != nil {
			return err
		}
		paramsLen := len(params)
		for i, param := range params {
			paramEncodedBytes, err := json.Marshal(param)
			if err != nil {
				return err
			}
			if _, err = w.Write(paramEncodedBytes); err != nil {
				return err
			}
			if i+1 != paramsLen {
				if _, err = io.WriteString(w, ", "); err != nil {
					return err
				}
			}
		}
		if _, err = io.WriteString(w, ")\r\n</script>"); err != nil {
			return err
		}
		return nil
	})
}

var m = minify.New()

func EmbedMinifiedScript(script templ.ComponentScript, params ...interface{}) templ.Component {
	r := bytes.NewReader([]byte(script.Function))
	w := bytes.NewBuffer(nil)
	err := js.Minify(m, w, r, nil)
	if err == nil {
		script.Function = w.String()
	}

	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) (err error) {
		if _, err = io.WriteString(w, `<script type="text/javascript">`+"\r\n"+script.Function+"\r\n"+script.Name+"("); err != nil {
			return err
		}
		paramsLen := len(params)
		for i, param := range params {
			paramEncodedBytes, err := json.Marshal(param)
			if err != nil {
				return err
			}
			if _, err = w.Write(paramEncodedBytes); err != nil {
				return err
			}
			if i+1 != paramsLen {
				if _, err = io.WriteString(w, ", "); err != nil {
					return err
				}
			}
		}
		if _, err = io.WriteString(w, ")\r\n</script>"); err != nil {
			return err
		}
		return nil
	})
}
