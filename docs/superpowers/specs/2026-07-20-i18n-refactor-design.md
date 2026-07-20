# i18n 包重构设计

日期:2026-07-20
状态:已由用户确认方向(允许破坏性变更;删除用户级缓存)

## 背景与问题

现有 i18n 包(基于 golang.org/x/text 的薄封装)存在以下生产阻碍:

1. `langLocalizers` 以任意 `tag.String()` 为 key 无限缓存 —— 若 tag 来自用户输入(Accept-Language),构成内存耗尽 DoS 向量。
2. `userLocalizers` 用户级缓存是单实例内存态:多实例不一致、重启丢失、无限增长。
3. 无语言协商:没有 Matcher、没有支持语言集合声明、没有 Accept-Language 解析。
4. 导出函数返回未导出类型 `*localizer`。
5. `SystemLocal` 死代码;`Locale()` 在 Windows 返回 `en-US`、Unix 返回 `en_US.UTF-8`,格式不一致且无法解析为 `language.Tag`。
6. 测试覆盖率 33%,核心路径无测试。

## 设计目标

保持简单易用:零配置即可用(设置语言、注册翻译、取文本);协商能力为可选增强;单文件实现(平台相关的系统 locale 检测除外)。

## 新 API

```go
type Localizer struct { /* tag + printer,类型导出 */ }
func (l *Localizer) Tag() language.Tag
func (l *Localizer) Printer() *message.Printer
func (l *Localizer) GetText(key message.Reference, a ...any) string

// 全局语言
func SetLanguage(tag language.Tag)
func Language() language.Tag
func GetText(key message.Reference, a ...any) string

// context 传递(每请求语言)
func WithLanguage(ctx context.Context, tag language.Tag) context.Context
func LanguageFromContext(ctx context.Context) (language.Tag, bool)
func LocalizerFromContext(ctx context.Context) *Localizer
func GetTextWithContext(ctx context.Context, key message.Reference, a ...any) string

// 语言协商(可选)
func SetSupported(tags ...language.Tag) // 声明支持集合;传空清除
func Match(preferred ...string) language.Tag // Accept-Language 值 → 支持集合内的 tag

// 系统语言
func SystemLanguage() language.Tag // 解析 OS locale,失败回退 language.AmericanEnglish
```

## 删除的 API(破坏性变更)

- `SetUserLanguage` / `GetUserLanguage` / `ClearUserLanguage` / `ClearAllUserLanguages` / `WithUserID` / `UserIDKey` —— 用户偏好由应用自己存储(DB/session),中间件用 `WithLanguage` 注入。
- `GetGlobalLanguage` / `ResetGlobalLanguage` —— 由 `Language()` / `SetLanguage` 覆盖。
- `ClearLangLocalizers` —— 缓存自我限界后不再需要手动清理。
- `SystemLocal` 包变量、导出的 `Locale() string` —— 由 `SystemLanguage() language.Tag` 取代(内部保留平台实现)。
- `Localizer()` 全局获取函数 —— 与导出类型重名,由 `Language()` + `LocalizerFromContext` 覆盖。

## 关键机制

### 缓存限界(防 DoS)

`localizerFor(tag)` 是唯一的 printer 获取入口:

1. 若已 `SetSupported`:先用 `language.Matcher` 将 tag 归一到支持集合内(用匹配到的 index 取 supported 中的原始 tag,规避 matcher 返回合成 tag 的已知坑),缓存 key 恒有界(= 支持语言数)。
2. 若未 `SetSupported`:直接以传入 tag 为 key,但缓存有硬上限(256 条);超限的 tag 不入缓存,临时创建 printer 返回(`message.NewPrinter` 开销很小)。

两条路径下均不存在无限增长。缓存用 `sync.RWMutex + map`(需要原子的 size 检查,`sync.Map` 不适合)。

### context key

单个未导出的 `struct{}` 类型 key,不再导出 `LanguageKey`(外部一律走 `WithLanguage` / `LanguageFromContext`)。

### 系统语言检测

- Unix:读 `LC_ALL` → `LANG`,得到 `en_US.UTF-8` 形态。
- Windows:`GetUserDefaultLocaleName` → `GetSystemDefaultLocaleName`,得到 `en-US` 形态。
- `SystemLanguage()` 统一清洗(截断 `.`/`@` 后缀、`_`→`-`)后 `language.Parse`,失败回退 `language.AmericanEnglish`。

### SetSupported 语义

- 第一个 tag 即 Matcher 的 fallback(x/text 语义)。
- 调用后 `Match`、`localizerFor` 均经归一;`SetLanguage` 传入的 tag 同样经归一(保证全局语言也在支持集合内)。
- 传空参数清除支持集合,恢复零配置模式。
- 用 `atomic.Pointer` 存 matcher+tags,并发安全。

## 测试计划

- 全局:默认语言、SetLanguage 后 GetText 出对应翻译、Language() 读回。
- context:WithLanguage 覆盖全局;无 ctx 值时回退全局;LocalizerFromContext 返回类型可用。
- 协商:SetSupported 后 zh-TW 归一到 zh;Match 解析 Accept-Language;不支持语言回退首个 tag。
- 缓存限界:未设 supported 时塞入 300 个不同 tag,缓存不超过 256,且超限 tag 仍能正确取文本。
- SystemLanguage:对 `en_US.UTF-8`、`en-US`、空值的解析(通过内部函数单测)。
- 并发:`go test -race` 下并发 SetLanguage/GetText/GetTextWithContext。
- 测试间用 t.Cleanup 恢复全局状态。

## 不做的事(YAGNI)

- 不做翻译文件加载(gotext 生成的 catalog 由使用方自行注册)。
- 不做复数/性别封装(直接暴露 `Printer()`,x/text 原生能力足够)。
- 不做用户偏好持久化。
