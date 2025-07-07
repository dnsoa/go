package i18n

import (
	"context"
	"sync"
	"sync/atomic"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var (
	SystemLocal    = Locale()
	altLocalizer   atomic.Pointer[localizer]
	defaultLocale  = language.AmericanEnglish
	defaultPrinter = message.NewPrinter(defaultLocale)
	// 用户语言缓存池
	userLocalizers sync.Map
)

type contextKey string

const (
	UserIDKey   contextKey = "user_id"
	LanguageKey contextKey = "language"
)

type localizer struct {
	tag     language.Tag
	printer *message.Printer
}

func (l *localizer) Tag() language.Tag {
	return l.tag
}

func (l *localizer) Printer() *message.Printer {
	return l.printer
}

// 从 context 获取用户特定的 localizer
func LocalizerFromContext(ctx context.Context) *localizer {
	// 优先使用用户ID获取用户专属语言设置
	if userID := ctx.Value(UserIDKey); userID != nil {
		if cached, exists := userLocalizers.Load(userID); exists {
			return cached.(*localizer)
		}
	}

	// 其次使用 context 中的语言设置
	if lang, ok := ctx.Value(LanguageKey).(language.Tag); ok {
		return getOrCreateLocalizer(lang.String(), lang)
	}

	// 最后使用全局设置
	return Localizer()
}

// 设置用户语言偏好
func SetUserLanguage[T comparable](userID T, tag language.Tag) {
	localizer := &localizer{
		tag:     tag,
		printer: message.NewPrinter(tag),
	}
	userLocalizers.Store(userID, localizer)
}

func GetUserLanguage[T comparable](userID T) (language.Tag, bool) {
	if cached, exists := userLocalizers.Load(userID); exists {
		return cached.(*localizer).tag, true
	}
	return language.Und, false
}

func ClearUserLanguage[T comparable](userID T) {
	userLocalizers.Delete(userID)
}

// 获取或创建 localizer（带缓存）
func getOrCreateLocalizer(key string, tag language.Tag) *localizer {
	if cached, exists := userLocalizers.Load(key); exists {
		return cached.(*localizer)
	}

	localizer := &localizer{
		tag:     tag,
		printer: message.NewPrinter(tag),
	}
	userLocalizers.Store(key, localizer)
	return localizer
}

// 支持 context 的多语言文本获取
func GetTextWithContext(ctx context.Context, key message.Reference, a ...any) string {
	localizer := LocalizerFromContext(ctx)
	return localizer.printer.Sprintf(key, a...)
}

func Localizer() *localizer {
	if f := altLocalizer.Load(); f != nil {
		return f
	}
	return &localizer{
		tag:     defaultLocale,
		printer: defaultPrinter,
	}
}

func SetLanguage(tag language.Tag) {
	var trampoline *localizer
	p := message.NewPrinter(tag)
	trampoline = &localizer{
		tag:     tag,
		printer: p,
	}
	altLocalizer.Store(trampoline)
}

func GetText(key message.Reference, a ...any) string {
	if f := altLocalizer.Load(); f != nil {
		return f.printer.Sprintf(key, a...)
	}
	return defaultPrinter.Sprintf(key, a...)
}
