package i18n

import (
	"sync/atomic"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var (
	SystemLocal    = Locale()
	altLocalizer   atomic.Pointer[localizer]
	defaultLocale  = language.AmericanEnglish
	defaultPrinter = message.NewPrinter(defaultLocale)
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
