package dashboard

import (
	"bytes"
	"net/http"

	"github.com/a-h/templ"
)

func renderTempl(w http.ResponseWriter, r *http.Request, status int, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := c.Render(r.Context(), w); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func renderComponentToString(r *http.Request, c templ.Component) (string, error) {
	var b bytes.Buffer
	if err := c.Render(r.Context(), &b); err != nil {
		return "", err
	}
	return b.String(), nil
}
