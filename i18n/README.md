# i18n

轻量、生产可用的多语言工具包,基于 `golang.org/x/text` 的 `language` 与 `message.Printer` 封装。零配置即可使用,可选的语言协商能力,内置防内存耗尽的缓存限界。

## 快速开始

```go
import (
    "github.com/dnsoa/go/i18n"
    "golang.org/x/text/language"
    "golang.org/x/text/message"
)

// 注册翻译(也可用 gotext 生成的 catalog)
message.SetString(language.Chinese, "greeting", "你好")

// 设置全局语言并取文本
i18n.SetLanguage(language.Chinese)
fmt.Println(i18n.GetText("greeting")) // 你好
```

## 每请求语言(context)

用户语言偏好由应用自己存储(数据库/session),在中间件里注入 context:

```go
func middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        tag := i18n.Match(r.Header.Get("Accept-Language"))
        next.ServeHTTP(w, r.WithContext(i18n.WithLanguage(r.Context(), tag)))
    })
}

// handler 中
msg := i18n.GetTextWithContext(ctx, "greeting")
```

## 语言协商(可选)

声明应用支持的语言集合后,`Match` 与所有 tag 查找都会归一到最接近的支持语言,第一个 tag 为兜底:

```go
i18n.SetSupported(language.English, language.Chinese, language.French)

i18n.Match("zh-TW,zh;q=0.9")  // → zh(归一)
i18n.Match("ja-JP")           // → en(兜底)
```

## API 一览

| 函数 | 说明 |
|---|---|
| `SetLanguage(tag)` / `Language()` | 设置/读取全局语言 |
| `GetText(key, a...)` | 全局语言取文本 |
| `WithLanguage(ctx, tag)` | 向 context 注入语言 |
| `LanguageFromContext(ctx)` | 从 context 读取语言 |
| `GetTextWithContext(ctx, key, a...)` | context 语言取文本(回退全局) |
| `LocalizerFromContext(ctx) *Localizer` | 获取 Localizer(`Tag()` / `Printer()` / `GetText()`) |
| `SetSupported(tags...)` | 声明支持语言集合,启用协商;空参清除 |
| `Match(preferred...)` | Accept-Language → 支持集合内的 tag |
| `SystemLanguage()` | 操作系统语言(解析失败回退 en-US) |

## 设计要点

- **并发安全**:全局语言用 `atomic.Pointer`,printer 缓存用 `RWMutex`,通过 `-race` 测试。
- **缓存有界**:声明了 `SetSupported` 时缓存 key 数量恒等于支持语言数;未声明时缓存有硬上限(256),超限的 tag 仍可用(走临时 printer),不存在无限增长路径 —— 即使 tag 来自不可信输入也不会被打爆内存。
- **翻译注册时机自由**:printer 的翻译查找发生在每次调用时,缓存的 printer 在之后 `message.SetString` 注入的翻译同样生效。
- **不做的事**:不持久化用户偏好(应用自己存,`WithLanguage` 注入)、不封装复数/货币(用 `Printer()` 直接访问 x/text 原生能力)。

## 从旧版迁移

| 旧 API | 替代 |
|---|---|
| `SetUserLanguage` / `GetUserLanguage` / `ClearUserLanguage` / `ClearAllUserLanguages` / `WithUserID` | 应用自行存储用户偏好,请求时 `WithLanguage(ctx, tag)` |
| `GetGlobalLanguage()` / `ResetGlobalLanguage()` | `Language()` / `SetLanguage(language.AmericanEnglish)` |
| `ClearLangLocalizers()` | 不再需要,缓存自动限界 |
| `Localizer()`(函数) | `LocalizerFromContext(ctx)` |
| `Locale() string` / `SystemLocal` | `SystemLanguage() language.Tag` |
