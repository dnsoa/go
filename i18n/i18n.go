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
	langLocalizers sync.Map
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
	// 先使用 context 中的语言设置（优先覆盖用户偏好）
	if lang, ok := ctx.Value(LanguageKey).(language.Tag); ok {
		return getOrCreateLocalizer(lang.String(), lang)
	}

	// 优先使用用户ID获取用户专属语言设置（仅支持 string 类型）
	if userID := ctx.Value(UserIDKey); userID != nil {
		if uid, ok := userID.(string); ok {
			if cached, exists := userLocalizers.Load(uid); exists {
				return cached.(*localizer)
			}
		}
	}

	// 最后使用全局设置
	return Localizer()
}

// 设置用户语言偏好
func SetUserLanguage(userID string, tag language.Tag) {
	localizer := &localizer{
		tag:     tag,
		printer: message.NewPrinter(tag),
	}
	userLocalizers.Store(userID, localizer)
}

func GetUserLanguage(userID string) (language.Tag, bool) {
	if cached, exists := userLocalizers.Load(userID); exists {
		return cached.(*localizer).tag, true
	}
	return language.Und, false
}

func ClearUserLanguage(userID string) {
	userLocalizers.Delete(userID)
}

// ClearAllUserLanguages 删除所有用户本地化设置
func ClearAllUserLanguages() {
	userLocalizers.Range(func(k, v any) bool {
		userLocalizers.Delete(k)
		return true
	})
}

// 获取或创建 localizer（带缓存）
func getOrCreateLocalizer(key string, tag language.Tag) *localizer {
	if cached, exists := langLocalizers.Load(key); exists {
		return cached.(*localizer)
	}

	loc := &localizer{
		tag:     tag,
		printer: message.NewPrinter(tag),
	}
	langLocalizers.Store(key, loc)
	return loc
}

// ClearLangLocalizers 清理按语言缓存
func ClearLangLocalizers() {
	langLocalizers.Range(func(k, v any) bool {
		langLocalizers.Delete(k)
		return true
	})
}

// WithUserID returns a new context with the given user ID set.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

// WithLanguage returns a new context with the given language tag set.
func WithLanguage(ctx context.Context, tag language.Tag) context.Context {
	return context.WithValue(ctx, LanguageKey, tag)
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

func GetGlobalLanguage() language.Tag {
	return Localizer().tag
}

func ResetGlobalLanguage() {
	SetLanguage(defaultLocale)
}

func GetText(key message.Reference, a ...any) string {
	if f := altLocalizer.Load(); f != nil {
		return f.printer.Sprintf(key, a...)
	}
	return defaultPrinter.Sprintf(key, a...)
}
