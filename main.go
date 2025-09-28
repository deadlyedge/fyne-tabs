package main

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/BurntSushi/toml"
)

type settings struct {
	Tabs struct {
		// Total int      `toml:"total"`
		Names []string `toml:"names"`
	} `toml:"tabs"`
	Theme struct {
		Font  string `toml:"font"`
		Color string `toml:"color"`
	} `toml:"theme"`
	Window struct {
		Size       string `toml:"size"`
		Resizeable bool   `toml:"resizeble"`
	} `toml:"window"`
}

// customTheme 在默认主题基础上覆盖主色与字体资源。
type customTheme struct {
	base         fyne.Theme
	primaryColor color.Color
	fontResource fyne.Resource
}

// Color 返回指定主题颜色名对应的颜色值，优先使用自定义主色调。
func (t *customTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNamePrimary, theme.ColorNameButton, theme.ColorNameHover, theme.ColorNameFocus:
		return t.primaryColor
	default:
		return t.base.Color(name, variant)
	}
}

// Icon 委托基础主题提供图标资源。
func (t *customTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(name)
}

// Font 根据文本样式返回字体资源，如提供自定义字体则优先使用。
func (t *customTheme) Font(style fyne.TextStyle) fyne.Resource {
	if t.fontResource != nil && !style.Monospace && !style.Symbol {
		return t.fontResource
	}
	return t.base.Font(style)
}

// Size 返回主题中配置的尺寸值。
func (t *customTheme) Size(name fyne.ThemeSizeName) float32 {
	return t.base.Size(name)
}

// main 是应用入口，负责加载配置、应用主题并启动窗口。
func main() {
	cfg, err := loadSettings("settings.toml")
	if err != nil {
		log.Fatalf("failed to load settings: %v", err)
	}

	// 解析配置颜色为 HSL，之后全程以 HSL 作为内部表示，只有在需要渲染时才转换为 NRGBA。
	origH, origS, origL, err := parseColorToHSL(cfg.Theme.Color)
	if err != nil {
		log.Printf("unable to parse color %q, falling back to default theme color: %v", cfg.Theme.Color, err)
		def := theme.DefaultTheme().Color(theme.ColorNamePrimary, theme.VariantLight)
		defNR := color.NRGBAModel.Convert(def).(color.NRGBA)
		origH, origS, origL = nrgbaToHSL(defNR)
	}

	fontResource, err := loadFontResource(cfg.Theme.Font)
	if err != nil {
		log.Printf("unable to load font %q, falling back to default font: %v", cfg.Theme.Font, err)
	}

	application := app.New()
	// 将内部 HSL 转换为 NRGBA 用于主题主色（只做一次转换）
	application.Settings().SetTheme(&customTheme{
		base:         theme.DefaultTheme(),
		primaryColor: hslToNRGBA(origH, origS, origL),
		fontResource: fontResource,
	})

	window := application.NewWindow("Fyne Tabs")

	width, height, err := parseWindowSize(cfg.Window.Size)
	if err != nil {
		log.Printf("failed to parse window size, using default size: %v", err)
	} else {
		window.Resize(fyne.NewSize(width, height))
	}
	window.SetFixedSize(!cfg.Window.Resizeable)

	tabNames := normalizeTabNames(cfg.Tabs.Names)
	if len(tabNames) == 0 {
		log.Println("no tab names provided, falling back to default")
		tabNames = []string{"Tab"}
	}

	// 使用 HSL 原始值作为基础色；tabs 数量由 names 决定（已在 normalizeTabNames 中处理）
	tabContents := make([]fyne.CanvasObject, len(tabNames))
	for i, name := range tabNames {
		h := origH
		s := origS
		l := origL
		if i != 0 {
			s = clamp01(origS - 0.10) // 未激活 tabs 降低 10% 饱和度
		}
		tabContents[i] = buildTabContentHSL(name, h, s, l)
	}

	window.SetContent(newVerticalTabs(tabNames, tabContents, origH, origS, origL))
	window.ShowAndRun()
}

// loadSettings 从给定路径读取 TOML 配置并解析为 settings 结构体。
func loadSettings(path string) (*settings, error) {
	var cfg settings
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// parseWindowSize 解析形如 "宽x高" 的窗口尺寸字符串，返回宽度、高度与错误信息。
func parseWindowSize(size string) (float32, float32, error) {
	cleaned := strings.ToLower(strings.TrimSpace(size))
	if cleaned == "" {
		return 0, 0, fmt.Errorf("size is empty")
	}

	parts := strings.Split(cleaned, "x")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid size format: %s", size)
	}

	width, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid width: %w", err)
	}

	height, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid height: %w", err)
	}

	return float32(width), float32(height), nil
}

// parseColor 根据字符串表达式解析颜色，目前支持 HSL 与十六进制格式。
// func parseColor(value string) (color.Color, error) {
// 	trimmed := strings.TrimSpace(strings.ToLower(value))
// 	switch {
// 	case strings.HasPrefix(trimmed, "hsl("):
// 		return parseHSL(trimmed)
// 	case strings.HasPrefix(trimmed, "#"):
// 		return parseHex(trimmed)
// 	default:
// 		return nil, fmt.Errorf("unsupported color format: %s", value)
// 	}
// }

// parseHex 解析 value 中的 #RRGGBB/#RGB 等十六进制色值，返回颜色对象和解析错误。
func parseHex(value string) (color.Color, error) {
	hex := strings.TrimPrefix(value, "#")

	var expanded string
	switch len(hex) {
	case 3, 4:
		var builder strings.Builder
		builder.Grow(len(hex) * 2)
		for _, r := range hex {
			builder.WriteRune(r)
			builder.WriteRune(r)
		}
		expanded = builder.String()
	case 6, 8:
		expanded = hex
	default:
		return nil, fmt.Errorf("unsupported hex length: %d", len(hex))
	}

	if len(expanded) == 6 {
		expanded += "ff"
	}

	parsed, err := strconv.ParseUint(expanded, 16, 32)
	if err != nil {
		return nil, err
	}

	return color.NRGBA{
		R: uint8(parsed >> 24),
		G: uint8(parsed >> 16),
		B: uint8(parsed >> 8),
		A: uint8(parsed),
	}, nil
}

 // parseHSL 将 HSL 字符串转换为 NRGBA 颜色，返回颜色与可能的错误。
// func parseHSL(value string) (color.Color, error) {
// 	inner := strings.TrimSuffix(strings.TrimPrefix(value, "hsl("), ")")
// 	inner = strings.ReplaceAll(inner, ",", " ")
// 	fields := strings.Fields(inner)
// 	if len(fields) != 3 {
// 		return nil, fmt.Errorf("invalid hsl format: %s", value)
// 	}

// 	hue, err := strconv.ParseFloat(fields[0], 64)
// 	if err != nil {
// 		return nil, fmt.Errorf("invalid hue: %w", err)
// 	}

// 	sat, err := parsePercentage(fields[1])
// 	if err != nil {
// 		return nil, fmt.Errorf("invalid saturation: %w", err)
// 	}

// 	light, err := parsePercentage(fields[2])
// 	if err != nil {
// 		return nil, fmt.Errorf("invalid lightness: %w", err)
// 	}

// 	return hslToNRGBA(hue, sat, light), nil
// }

// parseColorToHSL 解析配置色值并返回 H, S, L（H: 0-360, S/L: 0-1）。
// 支持 hsl(...) 与 #RRGGBB/#RGB（若为其它格式，返回错误）。
func parseColorToHSL(value string) (h, s, l float64, err error) {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if strings.HasPrefix(trimmed, "hsl(") {
		inner := strings.TrimSuffix(strings.TrimPrefix(value, "hsl("), ")")
		inner = strings.ReplaceAll(inner, ",", " ")
		fields := strings.Fields(inner)
		if len(fields) != 3 {
			return 0, 0, 0, fmt.Errorf("invalid hsl format: %s", value)
		}
		hue, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("invalid hue: %w", err)
		}
		sat, err := parsePercentage(fields[1])
		if err != nil {
			return 0, 0, 0, fmt.Errorf("invalid saturation: %w", err)
		}
		light, err := parsePercentage(fields[2])
		if err != nil {
			return 0, 0, 0, fmt.Errorf("invalid lightness: %w", err)
		}
		return hue, sat, light, nil
	} else if strings.HasPrefix(trimmed, "#") {
		c, err := parseHex(trimmed)
		if err != nil {
			return 0, 0, 0, err
		}
		n := color.NRGBAModel.Convert(c).(color.NRGBA)
		h, s, l = nrgbaToHSL(n)
		return h, s, l, nil
	}
	return 0, 0, 0, fmt.Errorf("unsupported color format: %s", value)
}

// parsePercentage 解析带百分号的字符串并返回 0-1 范围的小数。
func parsePercentage(value string) (float64, error) {
	trimmed := strings.TrimSuffix(strings.TrimSpace(value), "%")
	val, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, err
	}
	return val / 100, nil
}

 // hslToNRGBA 将 HSL 数值转换为 NRGBA 颜色。
func hslToNRGBA(h, s, l float64) color.NRGBA {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}

	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2

	var r, g, b float64

	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}

	toUint8 := func(value float64) uint8 {
		return uint8(math.Round((value + m) * 255))
	}

	return color.NRGBA{R: toUint8(r), G: toUint8(g), B: toUint8(b), A: 255}
}

// nrgbaToHSL 将 color.NRGBA 转为 H, S, L（H: 0-360, S/L: 0-1）
func nrgbaToHSL(c color.NRGBA) (h, s, l float64) {
	r := float64(c.R) / 255.0
	g := float64(c.G) / 255.0
	b := float64(c.B) / 255.0

	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	l = (max + min) / 2.0

	if max == min {
		// 灰色，无饱和度
		h = 0
		s = 0
		return
	}

	d := max - min
	if l > 0.5 {
		s = d / (2.0 - max - min)
	} else {
		s = d / (max + min)
	}

	switch max {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	case b:
		h = (r-g)/d + 4
	}
	h = h * 60.0
	return
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// adjustSaturationNRGBA 在 NRGBA 颜色上增加（或减少）饱和度（delta 可为负）。
// delta 是绝对值，比如 -0.10 表示降低 10%（即 -0.10）。
// func adjustSaturationNRGBA(c color.NRGBA, delta float64) color.NRGBA {
// 	h, s, l := nrgbaToHSL(c)
// 	s = clamp01(s + delta)
// 	return hslToNRGBA(h, s, l)
// }

// // adjustLightnessNRGBA 在 NRGBA 颜色上增加（或减少）亮度（delta 可为负）。
// func adjustLightnessNRGBA(c color.NRGBA, delta float64) color.NRGBA {
// 	h, s, l := nrgbaToHSL(c)
// 	l = clamp01(l + delta)
// 	return hslToNRGBA(h, s, l)
// }

// loadFontResource 尝试加载字体文件，返回可供 Fyne 使用的资源对象。
func loadFontResource(font string) (fyne.Resource, error) {
	font = strings.TrimSpace(font)
	if font == "" {
		return nil, nil
	}

	candidates := fontCandidates(font)
	for _, candidate := range candidates {
		data, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		return fyne.NewStaticResource(filepath.Base(candidate), data), nil
	}

	return nil, fmt.Errorf("font %q not found in known locations", font)
}

// fontCandidates 根据配置的字体名生成待尝试的可能文件路径列表。
func fontCandidates(font string) []string {
	seen := make(map[string]struct{})
	ordered := make([]string, 0, 8)

	add := func(path string) {
		if path == "" {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		ordered = append(ordered, path)
	}

	baseNames := make([]string, 0, 2)
	if font != "" {
		baseNames = append(baseNames, font)
		if lower := strings.ToLower(font); lower != font {
			baseNames = append(baseNames, lower)
		}
	}

	extensions := []string{"", ".ttf", ".otf", ".ttc"}
	for _, name := range baseNames {
		if filepath.Ext(name) != "" {
			add(name)
			continue
		}
		for _, ext := range extensions {
			add(name + ext)
		}
	}

	dirs := fontDirectories()
	results := make([]string, 0, len(ordered)*(len(dirs)+1))
	resultSeen := make(map[string]struct{})

	appendResult := func(path string) {
		if path == "" {
			return
		}
		if _, ok := resultSeen[path]; ok {
			return
		}
		resultSeen[path] = struct{}{}
		results = append(results, path)
	}

	for _, candidate := range ordered {
		appendResult(candidate)
		if filepath.IsAbs(candidate) {
			continue
		}
		for _, dir := range dirs {
			appendResult(filepath.Join(dir, candidate))
		}
	}

	return results
}

// fontDirectories 返回当前系统中常见的字体目录，用于查找字体文件。
func fontDirectories() []string {
	dirs := make([]string, 0, 6)
	seen := make(map[string]struct{})

	add := func(path string) {
		if path == "" {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		dirs = append(dirs, path)
	}

	switch runtime.GOOS {
	case "windows":
		if windir := os.Getenv("WINDIR"); windir != "" {
			add(filepath.Join(windir, "Fonts"))
		}
	case "darwin":
		add("/System/Library/Fonts")
		add("/Library/Fonts")
		if home, err := os.UserHomeDir(); err == nil {
			add(filepath.Join(home, "Library", "Fonts"))
		}
	default:
		add("/usr/share/fonts")
		add("/usr/local/share/fonts")
		if home, err := os.UserHomeDir(); err == nil {
			add(filepath.Join(home, ".fonts"))
			add(filepath.Join(home, ".local", "share", "fonts"))
		}
	}

	return dirs
}

// normalizeTabNames 清洗标签名称，填充或裁剪到 total 指定数量并补全默认名称。
func normalizeTabNames(names []string) []string {
	cleaned := make([]string, 0, len(names))
	for _, name := range names {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}

	// if total <= 0 {
	// 	total = len(cleaned)
	// }
	// if total <= 0 {
	// 	return cleaned
	// }

	// if len(cleaned) > total {
	// 	cleaned = cleaned[:total]
	// }

	// for len(cleaned) < total {
	// 	cleaned = append(cleaned, fmt.Sprintf("Tab %d", len(cleaned)+1))
	// }

	return cleaned
}

 // buildTabContentHSL 根据标题与 HSL 颜色构建单个标签页内容（内部使用 HSL）。
func buildTabContentHSL(title string, h, s, l float64) *fyne.Container {
	// label 背景在基础色上降低 10% 的亮度
	labelL := clamp01(l - 0.10)
	labelBg := hslToNRGBA(h, s, labelL)
	background := canvas.NewRectangle(labelBg)
	background.SetMinSize(fyne.NewSize(0, 0))

	label := canvas.NewText(title, foregroundColorFor(labelBg))
	label.Alignment = fyne.TextAlignCenter

	return container.NewStack(background, container.NewCenter(label))
}

 // newVerticalTabs 构建带按钮和内容区域的自定义纵向标签组件。
 // originalH, originalS, originalL 为基础 HSL 值（内部统一使用 HSL）
func newVerticalTabs(names []string, contents []fyne.CanvasObject, originalH, originalS, originalL float64) fyne.CanvasObject {
	if len(names) == 0 || len(contents) == 0 {
		return widget.NewLabel("No tabs")
	}

	if len(contents) < len(names) {
		names = names[:len(contents)]
	}

	activeIndex := 0
	buttons := make([]*widget.Button, len(names))
	contentWrappers := make([]fyne.CanvasObject, len(names))
	tabObjects := make([]fyne.CanvasObject, len(names)*2)

	refreshButtons := func() {
		for i, btn := range buttons {
			if btn == nil {
				continue
			}
			desired := widget.MediumImportance
			if i == activeIndex {
				desired = widget.HighImportance
			}
			if btn.Importance != desired {
				btn.Importance = desired
				btn.Refresh()
			}
		}
	}

	// 初始化每个内容的背景颜色，contents 中每个元素应当是 *fyne.Container，
	// 且其第一个对象为用作背景的 *canvas.Rectangle（由 buildTabContentHSL 创建）。
	for i, name := range names {
		idx := i
		btn := widget.NewButton(name, func() {
			if activeIndex == idx {
				return
			}
			// 隐藏旧的内容
			contentWrappers[activeIndex].Hide()
			activeIndex = idx

			// 更新所有内容的背景色（根据当前激活索引决定是否降低饱和度）
			for j := range contentWrappers {
				inner, ok := contents[j].(*fyne.Container)
				if !ok || len(inner.Objects) == 0 {
					continue
				}
				rect, ok := inner.Objects[0].(*canvas.Rectangle)
				if !ok {
					continue
				}
				s := originalS
				if j != activeIndex {
					s = clamp01(originalS - 0.10)
				}
				rect.FillColor = hslToNRGBA(originalH, s, clamp01(originalL-0.10))
				rect.Refresh()
			}

			// 显示新激活内容并刷新按钮样式
			contentWrappers[activeIndex].Show()
			refreshButtons()
		})
		buttons[i] = btn

		// 直接使用 contents[i] 作为可显示/隐藏的内容容器
		wrapper := contents[i]
		wrapper.Hide()
		contentWrappers[i] = wrapper

		tabObjects[2*i] = btn
		tabObjects[2*i+1] = wrapper
	}

	// 根据初始激活索引更新背景色并显示对应内容
	for j := range contentWrappers {
		inner, ok := contents[j].(*fyne.Container)
		if !ok || len(inner.Objects) == 0 {
			continue
		}
		rect, ok := inner.Objects[0].(*canvas.Rectangle)
		if !ok {
			continue
		}
		s := originalS
		if j != activeIndex {
			s = clamp01(originalS - 0.10)
		}
		rect.FillColor = hslToNRGBA(originalH, s, clamp01(originalL-0.10))
		rect.Refresh()
	}

	contentWrappers[activeIndex].Show()
	refreshButtons()

	return container.New(&verticalTabsLayout{}, tabObjects...)
}

// verticalTabsLayout 实现纵向标签的自定义布局逻辑。
type verticalTabsLayout struct{}

// Layout 将纵向按钮置于顶部，并在按钮下方展开当前激活的标签内容。
func (l *verticalTabsLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	buttonCount := len(objects) / 2
	buttonHeights := make([]float32, 0, buttonCount)
	var totalButtonsHeight float32

	activeButtonIndex := -1
	var activeContent fyne.CanvasObject
	var activeContentMin fyne.Size

	for i, obj := range objects {
		if i%2 == 0 {
			min := obj.MinSize()
			buttonHeights = append(buttonHeights, min.Height)
			totalButtonsHeight += min.Height
			continue
		}
		if !obj.Visible() {
			continue
		}
		activeButtonIndex = (i - 1) / 2
		activeContent = obj
		activeContentMin = obj.MinSize()
	}

	var contentHeight float32
	if activeContent != nil {
		contentHeight = size.Height - totalButtonsHeight
		if contentHeight < activeContentMin.Height {
			contentHeight = activeContentMin.Height
		}
	}

	y := float32(0)
	buttonIndex := 0

	for i := 0; i < len(objects); i += 2 {
		btn := objects[i]
		height := buttonHeights[buttonIndex]
		btn.Resize(fyne.NewSize(size.Width, height))
		btn.Move(fyne.NewPos(0, y))
		y += height

		if activeContent != nil && buttonIndex == activeButtonIndex {
			activeContent.Resize(fyne.NewSize(size.Width, contentHeight))
			activeContent.Move(fyne.NewPos(0, y))
			y += contentHeight
		}

		buttonIndex++
	}
}

// MinSize 计算布局所需的最小尺寸，确保按钮与内容完整显示。
func (l *verticalTabsLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var width float32
	var buttonHeight float32
	var contentHeight float32

	for i, obj := range objects {
		min := obj.MinSize()
		if min.Width > width {
			width = min.Width
		}
		if i%2 == 0 {
			buttonHeight += min.Height
			continue
		}
		if !obj.Visible() {
			continue
		}
		if min.Height > contentHeight {
			contentHeight = min.Height
		}
	}

	return fyne.NewSize(width, buttonHeight+contentHeight)
}

// foregroundColorFor 根据背景色亮度选择合适的前景色。
func foregroundColorFor(background color.Color) color.Color {
	nrgba := color.NRGBAModel.Convert(background).(color.NRGBA)
	luminance := 0.2126*float64(nrgba.R)/255 + 0.7152*float64(nrgba.G)/255 + 0.0722*float64(nrgba.B)/255
	if luminance > 0.5 {
		return color.Black
	}
	return color.White
}
