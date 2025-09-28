package main

import (
	"fmt"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
)

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

	return cleaned
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
