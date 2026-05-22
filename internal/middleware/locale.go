package middleware

import (
	"net/http"

	"github.com/team-dang/aiseebackend/pkg/i18n"
)

// Locale extracts a supported language from the Accept-Language
// header and stores it in the request context for handlers to use.
func Locale(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := i18n.ParseAcceptLanguage(r.Header.Get("Accept-Language"))
		ctx := i18n.WithLang(r.Context(), lang)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
