package i18n

import (
	"context"
	"sync"
	"testing"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

func init() {
	message.SetString(language.Chinese, "greeting", "你好")
	message.SetString(language.English, "greeting", "Hello")
	message.SetString(language.French, "greeting", "Bonjour")
}

// restore resets global state after a test using only public API.
func restore(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		SetSupported()
		SetLanguage(language.AmericanEnglish)
	})
}

func TestDefaultLanguage(t *testing.T) {
	restore(t)
	if got := Language(); got != language.AmericanEnglish {
		t.Errorf("Language() = %v, want %v", got, language.AmericanEnglish)
	}
}

func TestSetLanguageAndGetText(t *testing.T) {
	restore(t)
	SetLanguage(language.Chinese)
	if got := Language(); got != language.Chinese {
		t.Errorf("Language() = %v, want %v", got, language.Chinese)
	}
	if got := GetText("greeting"); got != "你好" {
		t.Errorf("GetText(greeting) = %q, want %q", got, "你好")
	}
	SetLanguage(language.AmericanEnglish)
	if got := GetText("greeting"); got != "Hello" {
		t.Errorf("GetText(greeting) = %q, want %q", got, "Hello")
	}
}

func TestGetTextUnknownKeyReturnsKey(t *testing.T) {
	restore(t)
	if got := GetText("no-such-key"); got != "no-such-key" {
		t.Errorf("GetText(no-such-key) = %q, want key itself", got)
	}
}

func TestWithLanguageOverridesGlobal(t *testing.T) {
	restore(t)
	SetLanguage(language.AmericanEnglish)
	ctx := WithLanguage(context.Background(), language.Chinese)
	if got := GetTextWithContext(ctx, "greeting"); got != "你好" {
		t.Errorf("GetTextWithContext = %q, want %q", got, "你好")
	}
	// global remains untouched
	if got := GetText("greeting"); got != "Hello" {
		t.Errorf("GetText = %q, want %q", got, "Hello")
	}
}

func TestGetTextWithContextFallsBackToGlobal(t *testing.T) {
	restore(t)
	SetLanguage(language.French)
	if got := GetTextWithContext(context.Background(), "greeting"); got != "Bonjour" {
		t.Errorf("GetTextWithContext = %q, want %q", got, "Bonjour")
	}
}

func TestLanguageFromContext(t *testing.T) {
	restore(t)
	if _, ok := LanguageFromContext(context.Background()); ok {
		t.Error("LanguageFromContext on empty ctx: ok = true, want false")
	}
	ctx := WithLanguage(context.Background(), language.Chinese)
	tag, ok := LanguageFromContext(ctx)
	if !ok || tag != language.Chinese {
		t.Errorf("LanguageFromContext = %v, %v; want %v, true", tag, ok, language.Chinese)
	}
}

func TestLocalizerFromContext(t *testing.T) {
	restore(t)
	ctx := WithLanguage(context.Background(), language.Chinese)
	loc := LocalizerFromContext(ctx)
	if loc.Tag() != language.Chinese {
		t.Errorf("Tag() = %v, want %v", loc.Tag(), language.Chinese)
	}
	if loc.Printer() == nil {
		t.Fatal("Printer() = nil")
	}
	if got := loc.GetText("greeting"); got != "你好" {
		t.Errorf("loc.GetText = %q, want %q", got, "你好")
	}

	// no language in ctx → global localizer
	SetLanguage(language.French)
	loc = LocalizerFromContext(context.Background())
	if loc.Tag() != language.French {
		t.Errorf("Tag() = %v, want %v", loc.Tag(), language.French)
	}
}

func TestSetSupportedNormalizesContextLanguage(t *testing.T) {
	restore(t)
	SetSupported(language.English, language.Chinese)
	ctx := WithLanguage(context.Background(), language.MustParse("zh-TW"))
	loc := LocalizerFromContext(ctx)
	if loc.Tag() != language.Chinese {
		t.Errorf("Tag() = %v, want %v (normalized)", loc.Tag(), language.Chinese)
	}
	if got := GetTextWithContext(ctx, "greeting"); got != "你好" {
		t.Errorf("GetTextWithContext = %q, want %q", got, "你好")
	}
}

func TestSetSupportedNormalizesSetLanguage(t *testing.T) {
	restore(t)
	SetSupported(language.English, language.Chinese)
	SetLanguage(language.MustParse("zh-HK"))
	if got := Language(); got != language.Chinese {
		t.Errorf("Language() = %v, want %v (normalized)", got, language.Chinese)
	}
}

func TestSetSupportedUnsupportedFallsBackToFirst(t *testing.T) {
	restore(t)
	SetSupported(language.English, language.Chinese)
	ctx := WithLanguage(context.Background(), language.Japanese)
	if got := LocalizerFromContext(ctx).Tag(); got != language.English {
		t.Errorf("Tag() = %v, want fallback %v", got, language.English)
	}
}

func TestMatchAcceptLanguage(t *testing.T) {
	restore(t)
	SetSupported(language.English, language.Chinese, language.French)
	if got := Match("zh-TW,zh;q=0.9,en;q=0.8"); got != language.Chinese {
		t.Errorf("Match = %v, want %v", got, language.Chinese)
	}
	if got := Match("fr-CA"); got != language.French {
		t.Errorf("Match = %v, want %v", got, language.French)
	}
	if got := Match("ja-JP"); got != language.English {
		t.Errorf("Match = %v, want fallback %v", got, language.English)
	}
	if got := Match(); got != language.English {
		t.Errorf("Match() = %v, want fallback %v", got, language.English)
	}
	if got := Match("!!invalid!!"); got != language.English {
		t.Errorf("Match(invalid) = %v, want fallback %v", got, language.English)
	}
}

func TestMatchWithoutSupportedReturnsGlobal(t *testing.T) {
	restore(t)
	SetLanguage(language.French)
	if got := Match("zh-CN"); got != language.French {
		t.Errorf("Match = %v, want global %v", got, language.French)
	}
}

func TestCacheIsBounded(t *testing.T) {
	restore(t)
	for i := 0; i < 300; i++ {
		tag := language.MustParse("en-x-t" + string(rune('a'+i/26)) + string(rune('a'+i%26)))
		ctx := WithLanguage(context.Background(), tag)
		if got := GetTextWithContext(ctx, "greeting"); got != "Hello" {
			t.Fatalf("GetTextWithContext(#%d) = %q, want %q", i, got, "Hello")
		}
	}
	cacheMu.RLock()
	n := len(cache)
	cacheMu.RUnlock()
	if n > maxCacheSize {
		t.Errorf("cache size = %d, want <= %d", n, maxCacheSize)
	}
}

func TestParseLocale(t *testing.T) {
	cases := []struct {
		in   string
		want language.Tag
	}{
		{"en_US.UTF-8", language.MustParse("en-US")},
		{"zh_CN.GB2312", language.MustParse("zh-CN")},
		{"fr_FR@euro", language.MustParse("fr-FR")},
		{"en-US", language.MustParse("en-US")},
		{"C", defaultLocale},
		{"C.UTF-8", defaultLocale},
		{"POSIX", defaultLocale},
		{"", defaultLocale},
		{"!!garbage!!", defaultLocale},
	}
	for _, c := range cases {
		if got := parseLocale(c.in); got != c.want {
			t.Errorf("parseLocale(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestSystemLanguage(t *testing.T) {
	t.Setenv("LC_ALL", "fr_FR.UTF-8")
	if got := SystemLanguage(); got != language.MustParse("fr-FR") {
		t.Errorf("SystemLanguage() = %v, want fr-FR", got)
	}
	t.Setenv("LC_ALL", "")
	t.Setenv("LANG", "")
	if got := SystemLanguage(); got != defaultLocale {
		t.Errorf("SystemLanguage() with empty env = %v, want %v", got, defaultLocale)
	}
}

func TestConcurrentAccess(t *testing.T) {
	restore(t)
	SetSupported(language.English, language.Chinese)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				switch i % 4 {
				case 0:
					SetLanguage(language.Chinese)
				case 1:
					_ = GetText("greeting")
				case 2:
					ctx := WithLanguage(context.Background(), language.MustParse("zh-TW"))
					_ = GetTextWithContext(ctx, "greeting")
				case 3:
					_ = Match("zh-CN,en;q=0.5")
				}
			}
		}(i)
	}
	wg.Wait()
}
