# i18n 包说明

这是一个轻量的多语言工具包，基于 golang.org/x/text 提供的 language 与 message.Printer 做封装，支持全局语言、按语言缓存和按用户缓存的本地化策略。

主要特性

- 支持全局语言设置（SetLanguage / GetGlobalLanguage / ResetGlobalLanguage）。
- 支持按语言缓存 localizer（内部缓存，ClearLangLocalizers 可清空）。
- 支持按用户缓存 localizer（SetUserLanguage / GetUserLanguage / ClearUserLanguage / ClearAllUserLanguages）。
- 支持从 context 中获取 localizer（WithUserID / WithLanguage + LocalizerFromContext）。
- GetText 与 GetTextWithContext 两种文本获取方式。

重要设计与优先级

- LocalizerFromContext 优先级（高 -> 低）：
  1. context 中的 Language（WithLanguage）——会覆盖用户偏好
  2. context 中的 UserID（WithUserID），按 userID 从缓存获取
  3. 全局设置（SetLanguage）或默认语言

- userID 仅支持 string 类型：SetUserLanguage / WithUserID 等接口要求 userID 为 string。请在调用处统一转换。

并发与缓存

- 使用 sync.Map 做并发安全的缓存，无需额外加锁。
- langLocalizers 与 userLocalizers 都没有自动回收策略，如有大量动态语言或用户，请考虑定期调用 ClearLangLocalizers / ClearAllUserLanguages 清理缓存。

主要 API 概览

- WithUserID(ctx, userID string) context.Context
- WithLanguage(ctx, tag language.Tag) context.Context
- LocalizerFromContext(ctx) *localizer
- SetUserLanguage(userID string, tag language.Tag)
- GetUserLanguage(userID string) (language.Tag, bool)
- ClearUserLanguage(userID string)
- ClearAllUserLanguages()
- ClearLangLocalizers()
- SetLanguage(tag language.Tag)
- GetGlobalLanguage() language.Tag
- ResetGlobalLanguage()
- GetText(key message.Reference, a ...any) string
- GetTextWithContext(ctx context.Context, key message.Reference, a ...any) string

示例

1) 基本全局用法：

使用全局设置（影响不主动覆盖用户缓存）

    i18n.SetLanguage(language.SimplifiedChinese)
    msg := i18n.GetText(myMessageKey)

2) 按用户设置：

    i18n.SetUserLanguage("user-123", language.French)
    ctx := i18n.WithUserID(context.Background(), "user-123")
    msg := i18n.GetTextWithContext(ctx, myMessageKey)

3) 临时覆盖用户语言（context 优先）：

    ctx := i18n.WithUserID(context.Background(), "user-123")
    ctx = i18n.WithLanguage(ctx, language.Spanish) // 覆盖 user-123 的偏好
    msg := i18n.GetTextWithContext(ctx, myMessageKey)

注意事项

- 请确保传入的 message.Reference 在初始化时已注册对应语言的翻译条目。
- userID 应统一为 string 类型；若代码中使用其他类型，请在调用处转换为 string。
- 若需要全局强制刷新所有用户为新语言，可调用 ClearAllUserLanguages 后 SetLanguage。

许可证与贡献

仓库主模块的许可证与贡献说明请参考项目根目录。
