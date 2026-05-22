// Package i18n provides a small message catalog with per-language
// fallback. Strings are loaded from JSON bundles at startup.
package i18n

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

//go:embed locales/*.json
var bundles embed.FS

const (
	LangVI      = "vi"
	LangEN      = "en"
	DefaultLang = LangVI
)

type catalog struct {
	once     sync.Once
	messages map[string]map[string]string // lang -> key -> template
	loadErr  error
}

var defaultCatalog = &catalog{}

func (c *catalog) load() {
	c.messages = make(map[string]map[string]string)
	entries, err := bundles.ReadDir("locales")
	if err != nil {
		c.loadErr = fmt.Errorf("read locales dir: %w", err)
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		lang := strings.TrimSuffix(entry.Name(), ".json")
		raw, err := bundles.ReadFile("locales/" + entry.Name())
		if err != nil {
			c.loadErr = fmt.Errorf("read %s: %w", entry.Name(), err)
			return
		}
		var msgs map[string]string
		if err := json.Unmarshal(raw, &msgs); err != nil {
			c.loadErr = fmt.Errorf("parse %s: %w", entry.Name(), err)
			return
		}
		c.messages[lang] = msgs
	}
	if _, ok := c.messages[DefaultLang]; !ok {
		c.loadErr = fmt.Errorf("missing default locale bundle %q", DefaultLang)
	}
}

// T returns the localized message for key, falling back to the
// default language if the requested language or key is missing.
// Args are interpolated through fmt.Sprintf.
func T(lang, key string, args ...any) string {
	defaultCatalog.once.Do(defaultCatalog.load)
	if defaultCatalog.loadErr != nil {
		return key
	}

	lang = Normalize(lang)
	if msgs, ok := defaultCatalog.messages[lang]; ok {
		if tmpl, ok := msgs[key]; ok {
			return format(tmpl, args)
		}
	}
	if msgs, ok := defaultCatalog.messages[DefaultLang]; ok {
		if tmpl, ok := msgs[key]; ok {
			return format(tmpl, args)
		}
	}
	return key
}

func format(tmpl string, args []any) string {
	if len(args) == 0 {
		return tmpl
	}
	return fmt.Sprintf(tmpl, args...)
}

// Normalize maps any locale string to a supported language code.
// Anything starting with "en" returns "en"; everything else "vi".
func Normalize(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	if lang == "" {
		return DefaultLang
	}
	if strings.HasPrefix(lang, "en") {
		return LangEN
	}
	return LangVI
}

// FromUserSettings extracts preferred_language from a user's
// settings JSONB blob. Returns DefaultLang if missing or invalid.
func FromUserSettings(settings []byte) string {
	if len(settings) == 0 {
		return DefaultLang
	}
	var m map[string]any
	if err := json.Unmarshal(settings, &m); err != nil {
		return DefaultLang
	}
	if v, ok := m["preferred_language"].(string); ok && v != "" {
		return Normalize(v)
	}
	return DefaultLang
}

type ctxKey struct{}

// WithLang stores a normalized language code in the context.
func WithLang(ctx context.Context, lang string) context.Context {
	return context.WithValue(ctx, ctxKey{}, Normalize(lang))
}

// FromContext returns the request language, defaulting to DefaultLang.
func FromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKey{}).(string); ok && v != "" {
		return v
	}
	return DefaultLang
}

// ParseAcceptLanguage parses a single Accept-Language header value
// and returns the highest-priority supported language.
func ParseAcceptLanguage(header string) string {
	if header == "" {
		return DefaultLang
	}
	for _, part := range strings.Split(header, ",") {
		tag := strings.TrimSpace(part)
		if i := strings.Index(tag, ";"); i >= 0 {
			tag = tag[:i]
		}
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(tag), "en") {
			return LangEN
		}
		if strings.HasPrefix(strings.ToLower(tag), "vi") {
			return LangVI
		}
	}
	return DefaultLang
}
