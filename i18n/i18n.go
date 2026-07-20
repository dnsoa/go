// Package i18n is a thin, production-safe wrapper around
// golang.org/x/text/message for application localization.
//
// Zero configuration: set a global language with SetLanguage, register
// translations via message.SetString (or a gotext-generated catalog), and
// fetch text with GetText. Per-request languages travel through context via
// WithLanguage / GetTextWithContext. Optionally declare the languages your
// application supports with SetSupported to enable Accept-Language
// negotiation (Match) and normalization of arbitrary tags.
package i18n

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var defaultLocale = language.AmericanEnglish

// Localizer pairs a language tag with a message printer for that language.
type Localizer struct {
	tag     language.Tag
	printer *message.Printer
}

// Tag returns the localizer's language tag.
func (l *Localizer) Tag() language.Tag { return l.tag }

// Printer returns the underlying message printer, giving access to the full
// x/text formatting API (plurals, currencies, ...).
func (l *Localizer) Printer() *message.Printer { return l.printer }

// GetText returns the translation of key in the localizer's language,
// formatted with the given arguments.
func (l *Localizer) GetText(key message.Reference, a ...any) string {
	return l.printer.Sprintf(key, a...)
}

var (
	global    atomic.Pointer[Localizer]
	cacheMu   sync.RWMutex
	cache     = map[language.Tag]*Localizer{}
	supported atomic.Pointer[matcherSet]
)

type matcherSet struct {
	tags    []language.Tag
	matcher language.Matcher
}

func newLocalizer(tag language.Tag) *Localizer {
	return &Localizer{tag: tag, printer: message.NewPrinter(tag)}
}

// maxCacheSize bounds the localizer cache when no supported set is declared,
// so attacker-controlled tags cannot grow memory without limit. Tags beyond
// the bound still work; they just get an uncached printer.
const maxCacheSize = 256

// normalize maps tag to the closest supported language when SetSupported has
// been called; otherwise it returns tag unchanged.
func normalize(tag language.Tag) language.Tag {
	if s := supported.Load(); s != nil {
		// Use the matched index, not the returned tag: Matcher.Match may
		// synthesize tags that differ from the declared supported set.
		_, idx, _ := s.matcher.Match(tag)
		return s.tags[idx]
	}
	return tag
}

// localizerFor is the single entry point for resolving a tag to a Localizer.
func localizerFor(tag language.Tag) *Localizer {
	tag = normalize(tag)
	cacheMu.RLock()
	loc, ok := cache[tag]
	cacheMu.RUnlock()
	if ok {
		return loc
	}
	loc = newLocalizer(tag)
	cacheMu.Lock()
	if existing, ok := cache[tag]; ok {
		loc = existing
	} else if len(cache) < maxCacheSize {
		cache[tag] = loc
	}
	cacheMu.Unlock()
	return loc
}

// SetLanguage sets the global language used by GetText and as the fallback
// for GetTextWithContext when the context carries no language.
func SetLanguage(tag language.Tag) {
	global.Store(localizerFor(tag))
}

// Language returns the current global language.
func Language() language.Tag {
	return globalLocalizer().tag
}

func globalLocalizer() *Localizer {
	if l := global.Load(); l != nil {
		return l
	}
	return localizerFor(defaultLocale)
}

// GetText returns the translation of key in the global language.
func GetText(key message.Reference, a ...any) string {
	return globalLocalizer().GetText(key, a...)
}

// SetSupported declares the set of languages the application supports and
// enables negotiation: Match, SetLanguage and context lookups normalize any
// tag to the closest supported one. The first tag is the fallback.
// Calling SetSupported with no arguments clears the set.
func SetSupported(tags ...language.Tag) {
	if len(tags) == 0 {
		supported.Store(nil)
		return
	}
	tags = append([]language.Tag(nil), tags...)
	supported.Store(&matcherSet{
		tags:    tags,
		matcher: language.NewMatcher(tags),
	})
}

// Match resolves one or more Accept-Language header values to the best
// matching supported language. It returns the first supported tag when
// nothing matches or the input is unparsable, and the global language when
// SetSupported has not been called.
func Match(preferred ...string) language.Tag {
	s := supported.Load()
	if s == nil {
		return Language()
	}
	tags, _, err := language.ParseAcceptLanguage(strings.Join(preferred, ","))
	if err != nil || len(tags) == 0 {
		return s.tags[0]
	}
	_, idx, _ := s.matcher.Match(tags...)
	return s.tags[idx]
}

type contextKey struct{}

// WithLanguage returns a new context carrying the given language tag.
func WithLanguage(ctx context.Context, tag language.Tag) context.Context {
	return context.WithValue(ctx, contextKey{}, tag)
}

// LanguageFromContext returns the language tag stored in ctx by WithLanguage.
func LanguageFromContext(ctx context.Context) (language.Tag, bool) {
	tag, ok := ctx.Value(contextKey{}).(language.Tag)
	return tag, ok
}

// LocalizerFromContext returns the Localizer for the language stored in ctx,
// falling back to the global localizer when ctx carries no language.
func LocalizerFromContext(ctx context.Context) *Localizer {
	if tag, ok := LanguageFromContext(ctx); ok {
		return localizerFor(tag)
	}
	return globalLocalizer()
}

// GetTextWithContext returns the translation of key in the context's
// language, falling back to the global language.
func GetTextWithContext(ctx context.Context, key message.Reference, a ...any) string {
	return LocalizerFromContext(ctx).GetText(key, a...)
}

// SystemLanguage returns the operating system's configured language,
// falling back to American English when it is unset or unparsable.
func SystemLanguage() language.Tag {
	return parseLocale(sysLocale())
}

// parseLocale converts a raw OS locale string ("en_US.UTF-8", "fr_FR@euro",
// "en-US") to a language.Tag, falling back to defaultLocale.
func parseLocale(locale string) language.Tag {
	if i := strings.IndexAny(locale, ".@"); i >= 0 {
		locale = locale[:i]
	}
	if locale == "" || locale == "C" || locale == "POSIX" {
		return defaultLocale
	}
	tag, err := language.Parse(strings.ReplaceAll(locale, "_", "-"))
	if err != nil {
		return defaultLocale
	}
	return tag
}
