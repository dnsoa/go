package i18n

import (
	"testing"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

func TestI18n(t *testing.T) {
	lo := Localizer()
	if lo.Tag() != defaultLocale {
		t.Errorf("Expected %v, got %v", defaultLocale, lo.Tag())
	}
	if GetText("Hello") != "Hello" {
		t.Errorf("Expected %v, got %v", "你好", GetText("Hello"))
	}
	SetLanguage(language.Chinese)
	lo = Localizer()

	message.SetString(language.Chinese, "Hello", "你好")
	if GetText("Hello") != "你好" {
		t.Errorf("Expected %v, got %v", "你好", GetText("Hello"))
	}
	if lo.Tag() != language.Chinese {
		t.Errorf("Expected %v, got %v", language.Chinese, lo.Tag())
	}

}

func TestLocale(t *testing.T) {
	locale := Locale()
	t.Log(locale)
	p := message.NewPrinter(language.Afrikaans)
	p.Printf("Hello, world!")
	t.Log(p.Sprintf("Hello, world!"))

	t.Log(GetText("Language"))
	t.Log(GetText("Language"))

}
