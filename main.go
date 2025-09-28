package main

import (
	"fmt"
	"image/color"
	"log"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/theme"
	"github.com/BurntSushi/toml"
)

type settings struct {
	Tabs struct {
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

/* 上述工具函数已提取到 utils.go；在 main.go 保留更高层逻辑，避免重复定义。 */
