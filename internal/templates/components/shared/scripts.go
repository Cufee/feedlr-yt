package shared

import (
	"context"
	"encoding/json"
	"io"

	"github.com/a-h/templ"
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
