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
		Total int      `toml:"total"`
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

type customTheme struct {
	base         fyne.Theme
	primaryColor color.Color
	fontResource fyne.Resource
}

func (t *customTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNamePrimary, theme.ColorNameButton, theme.ColorNameHover, theme.ColorNameFocus:
		return t.primaryColor
	default:
		return t.base.Color(name, variant)
	}
}

func (t *customTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(name)
}

func (t *customTheme) Font(style fyne.TextStyle) fyne.Resource {
	if t.fontResource != nil && !style.Monospace && !style.Symbol {
		return t.fontResource
	}
	return t.base.Font(style)
}

func (t *customTheme) Size(name fyne.ThemeSizeName) float32 {
	return t.base.Size(name)
}

func main() {
	cfg, err := loadSettings("settings.toml")
	if err != nil {
		log.Fatalf("failed to load settings: %v", err)
	}

	tabColor, err := parseColor(cfg.Theme.Color)
	if err != nil {
		log.Printf("unable to parse color %q, falling back to default theme color: %v", cfg.Theme.Color, err)
		tabColor = theme.DefaultTheme().Color(theme.ColorNamePrimary, theme.VariantLight)
	}

	fontResource, err := loadFontResource(cfg.Theme.Font)
	if err != nil {
		log.Printf("unable to load font %q, falling back to default font: %v", cfg.Theme.Font, err)
	}

	application := app.New()
	application.Settings().SetTheme(&customTheme{
		base:         theme.DefaultTheme(),
		primaryColor: tabColor,
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

	tabNames := normalizeTabNames(cfg.Tabs.Total, cfg.Tabs.Names)
	if len(tabNames) == 0 {
		log.Println("no tab names provided, falling back to default")
		tabNames = []string{"Tab"}
	}

	tabContents := make([]fyne.CanvasObject, len(tabNames))
	for i, name := range tabNames {
		tabContents[i] = buildTabContent(name, tabColor)
	}

	window.SetContent(newVerticalTabs(tabNames, tabContents))
	window.ShowAndRun()
}

func loadSettings(path string) (*settings, error) {
	var cfg settings
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

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

func parseColor(value string) (color.Color, error) {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	switch {
	case strings.HasPrefix(trimmed, "hsl("):
		return parseHSL(trimmed)
	case strings.HasPrefix(trimmed, "#"):
		return parseHex(trimmed)
	default:
		return nil, fmt.Errorf("unsupported color format: %s", value)
	}
}

func parseHex(value string) (color.Color, error) {
	hex := strings.TrimPrefix(value, "#")
	var r, g, b, a uint8 = 0, 0, 0, 255

	switch len(hex) {
	case 6:
		parsed, err := strconv.ParseUint(hex, 16, 32)
		if err != nil {
			return nil, err
		}
		r = uint8(parsed >> 16)
		g = uint8(parsed >> 8)
		b = uint8(parsed)
	case 8:
		parsed, err := strconv.ParseUint(hex, 16, 32)
		if err != nil {
			return nil, err
		}
		r = uint8(parsed >> 24)
		g = uint8(parsed >> 16)
		b = uint8(parsed >> 8)
		a = uint8(parsed)
	default:
		return nil, fmt.Errorf("unsupported hex length: %d", len(hex))
	}

	return color.NRGBA{R: r, G: g, B: b, A: a}, nil
}

func parseHSL(value string) (color.Color, error) {
	inner := strings.TrimSuffix(strings.TrimPrefix(value, "hsl("), ")")
	inner = strings.ReplaceAll(inner, ",", " ")
	fields := strings.Fields(inner)
	if len(fields) != 3 {
		return nil, fmt.Errorf("invalid hsl format: %s", value)
	}

	hue, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid hue: %w", err)
	}

	sat, err := parsePercentage(fields[1])
	if err != nil {
		return nil, fmt.Errorf("invalid saturation: %w", err)
	}

	light, err := parsePercentage(fields[2])
	if err != nil {
		return nil, fmt.Errorf("invalid lightness: %w", err)
	}

	return hslToNRGBA(hue, sat, light), nil
}

func parsePercentage(value string) (float64, error) {
	trimmed := strings.TrimSuffix(strings.TrimSpace(value), "%")
	val, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, err
	}
	return val / 100, nil
}

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

func fontCandidates(font string) []string {
	unique := make(map[string]struct{})
	add := func(path string) {
		if path == "" {
			return
		}
		if _, exists := unique[path]; !exists {
			unique[path] = struct{}{}
		}
	}

	add(font)

	fontFileNames := []string{font}
	lower := strings.ToLower(font)
	if lower != font {
		fontFileNames = append(fontFileNames, lower)
	}

	extensions := []string{"", ".ttf", ".otf", ".ttc"}

	for _, name := range fontFileNames {
		if filepath.Ext(name) != "" {
			add(name)
			continue
		}
		for _, ext := range extensions {
			if ext == "" {
				continue
			}
			add(name + ext)
		}
	}

	dirs := fontDirectories()

	var results []string
	for candidate := range unique {
		results = append(results, candidate)
		if filepath.IsAbs(candidate) {
			continue
		}
		for _, dir := range dirs {
			results = append(results, filepath.Join(dir, candidate))
		}
	}

	return results
}

func fontDirectories() []string {
	var dirs []string

	switch runtime.GOOS {
	case "windows":
		if windir := os.Getenv("WINDIR"); windir != "" {
			dirs = append(dirs, filepath.Join(windir, "Fonts"))
		}
	case "darwin":
		dirs = append(dirs, "/System/Library/Fonts", "/Library/Fonts", filepath.Join(os.Getenv("HOME"), "Library", "Fonts"))
	default:
		dirs = append(dirs, "/usr/share/fonts", "/usr/local/share/fonts")
		if home, err := os.UserHomeDir(); err == nil {
			dirs = append(dirs, filepath.Join(home, ".fonts"), filepath.Join(home, ".local", "share", "fonts"))
		}
	}

	return dirs
}

func normalizeTabNames(total int, names []string) []string {
	cleaned := make([]string, 0, len(names))
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}

	if total <= 0 {
		total = len(cleaned)
	}

	for len(cleaned) < total {
		cleaned = append(cleaned, fmt.Sprintf("Tab %d", len(cleaned)+1))
	}

	if total > 0 && len(cleaned) > total {
		cleaned = cleaned[:total]
	}

	return cleaned
}

func buildTabContent(title string, tabColor color.Color) fyne.CanvasObject {
	background := canvas.NewRectangle(tabColor)
	background.SetMinSize(fyne.NewSize(0, 0))

	label := canvas.NewText(title, foregroundColorFor(tabColor))
	label.Alignment = fyne.TextAlignCenter

	return container.NewMax(background, container.NewCenter(label))
}

func newVerticalTabs(names []string, contents []fyne.CanvasObject) fyne.CanvasObject {
	if len(names) == 0 || len(contents) == 0 {
		return widget.NewLabel("No tabs")
	}

	if len(contents) < len(names) {
		names = names[:len(contents)]
	}

	activeIndex := 0

	buttons := make([]*widget.Button, len(names))
	contentWrappers := make([]fyne.CanvasObject, len(names))
	tabObjects := make([]fyne.CanvasObject, 0, len(names)*2)

	refreshButtons := func() {
		for i, btn := range buttons {
			if btn == nil {
				continue
			}
			if i == activeIndex {
				btn.Importance = widget.HighImportance
			} else {
				btn.Importance = widget.MediumImportance
			}
			btn.Refresh()
		}
	}

	for i, name := range names {
		idx := i
		btn := widget.NewButton(name, func() {
			if activeIndex == idx {
				return
			}
			contentWrappers[activeIndex].Hide()
			activeIndex = idx
			contentWrappers[activeIndex].Show()
			refreshButtons()
		})
		buttons[i] = btn

		wrapper := container.NewMax(contents[i])
		wrapper.Hide()
		contentWrappers[i] = wrapper

		tabObjects = append(tabObjects, btn, wrapper)
	}

	contentWrappers[activeIndex].Show()
	refreshButtons()

	return container.New(&verticalTabsLayout{}, tabObjects...)
}

type verticalTabsLayout struct{}

func (l *verticalTabsLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	buttonHeights := make([]float32, 0, len(objects)/2)
	var totalButtonsHeight float32
	activeContentIndex := -1
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
		activeContentIndex = i
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
	activeButtonIndex := activeContentIndex / 2

	for i, obj := range objects {
		if i%2 != 0 {
			continue
		}

		height := buttonHeights[buttonIndex]
		obj.Resize(fyne.NewSize(size.Width, height))
		obj.Move(fyne.NewPos(0, y))
		y += height

		if activeContent != nil && buttonIndex == activeButtonIndex {
			activeContent.Resize(fyne.NewSize(size.Width, contentHeight))
			activeContent.Move(fyne.NewPos(0, y))
			y += contentHeight
		}

		buttonIndex++
	}
}

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

func foregroundColorFor(background color.Color) color.Color {
	nrgba := color.NRGBAModel.Convert(background).(color.NRGBA)
	luminance := 0.2126*float64(nrgba.R)/255 + 0.7152*float64(nrgba.G)/255 + 0.0722*float64(nrgba.B)/255
	if luminance > 0.5 {
		return color.Black
	}
	return color.White
}
