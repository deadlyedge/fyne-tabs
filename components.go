package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

/*
labelButton: 一个无圆角、可点击的简单 label 风格按钮，用于替换默认的 widget.Button。

	特点：
	- 无内置背景（使用外部矩形作为背景以控制圆角与饱和度）
	- 文本左对齐
	- 支持 Tapped 事件回调
*/
type labelButton struct {
	widget.BaseWidget
	text    string
	onTap   func()
	onHover func(bool)
}

func newLabelButton(text string, onTap func(), onHover func(bool)) *labelButton {
	l := &labelButton{text: text, onTap: onTap, onHover: onHover}
	l.ExtendBaseWidget(l)
	return l
}

func (l *labelButton) CreateRenderer() fyne.WidgetRenderer {
	txt := canvas.NewText(l.text, color.Black)
	txt.Alignment = fyne.TextAlignLeading
	objects := []fyne.CanvasObject{txt}
	return &labelButtonRenderer{label: l, objects: objects, text: txt}
}

func (l *labelButton) Tapped(_ *fyne.PointEvent) {
	if l.onTap != nil {
		l.onTap()
	}
}

func (l *labelButton) TappedSecondary(_ *fyne.PointEvent) {}

// Hover handling (desktop): forward hover state to optional onHover callback.
func (l *labelButton) MouseIn(*desktop.MouseEvent) {
	if l.onHover != nil {
		l.onHover(true)
	}
}

func (l *labelButton) MouseMoved(*desktop.MouseEvent) {}

func (l *labelButton) MouseOut() {
	if l.onHover != nil {
		l.onHover(false)
	}
}

type labelButtonRenderer struct {
	label   *labelButton
	objects []fyne.CanvasObject
	text    *canvas.Text
}

func (r *labelButtonRenderer) Layout(size fyne.Size) {
	r.text.Resize(size)
	r.text.Move(fyne.NewPos(4, 0))
}

func (r *labelButtonRenderer) MinSize() fyne.Size {
	m := r.text.MinSize()
	// 给左侧一点内边距
	return fyne.NewSize(m.Width+8, m.Height)
}

func (r *labelButtonRenderer) Refresh() {
	r.text.Text = r.label.text
	r.text.Refresh()
}

func (r *labelButtonRenderer) Objects() []fyne.CanvasObject { return r.objects }

func (r *labelButtonRenderer) Destroy() {}

// twoColumnLayout 简单的两列布局，按 LeftRatio 分配宽度给左列，右列占剩余宽度。
// 仅支持最多两个子对象，忽略额外对象；高度以可用高度为准。
type twoColumnLayout struct {
	LeftRatio float32
}

func (l *twoColumnLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	leftW := float32(0)
	if l.LeftRatio > 0 {
		leftW = float32(float64(size.Width) * float64(l.LeftRatio))
	}
	rightW := size.Width - leftW

	if len(objects) > 0 && objects[0] != nil {
		objects[0].Resize(fyne.NewSize(leftW, size.Height))
		objects[0].Move(fyne.NewPos(0, 0))
	}
	if len(objects) > 1 && objects[1] != nil {
		objects[1].Resize(fyne.NewSize(rightW, size.Height))
		objects[1].Move(fyne.NewPos(leftW, 0))
	}
}

func (l *twoColumnLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var height float32
	var leftW, rightW float32
	if len(objects) > 0 && objects[0] != nil {
		m := objects[0].MinSize()
		if m.Height > height {
			height = m.Height
		}
		leftW = m.Width
	}
	if len(objects) > 1 && objects[1] != nil {
		m := objects[1].MinSize()
		if m.Height > height {
			height = m.Height
		}
		rightW = m.Width
	}
	return fyne.NewSize(leftW+rightW, height)
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
	buttons := make([]fyne.CanvasObject, len(names))
	statusTexts := make([]*canvas.Text, len(names))
	contentContainers := make([]*fyne.Container, len(names))
	tabContainers := make([]fyne.CanvasObject, len(names))
	// 为每个 header 创建一个背景矩形，用于在非激活时降低饱和度
	btnBgs := make([]*canvas.Rectangle, len(names))

	refreshHeaders := func() {
		// 只更新 header 背景饱和度与刷新 label 渲染
		for i := range btnBgs {
			if btnBgs[i] == nil {
				continue
			}
			// s := originalS
			// if i != activeIndex {
			// 	s = clamp01(originalS - 0.20)
			// }
			btnBgs[i].FillColor = hslToNRGBA(originalH, originalS, clamp01(originalL-0.15))
			btnBgs[i].Refresh()
		}
		// 刷新每个 label 的文本（在必要时）
		for i := range buttons {
			if buttons[i] == nil {
				continue
			}
			if r, ok := buttons[i].(fyne.Widget); ok {
				r.Refresh()
			}
		}
	}

	for i, name := range names {
		idx := i
		// 可点击的 label（无圆角，自定义实现，文本左对齐）
		// 为了实现 hover 效果，我们传入一个 onHover 回调，用来更新对应的 header 背景颜色
		hoverIdx := idx
		onHover := func(h bool) {
			if btnBgs[hoverIdx] == nil {
				return
			}
			if h {
				// hover 时略微提高亮度
				btnBgs[hoverIdx].FillColor = hslToNRGBA(originalH, originalS, clamp01(originalL+0.02))
			} else {
				// 非 hover 时恢复到按激活状态计算的饱和度/亮度
				s := originalS
				if hoverIdx != activeIndex {
					s = clamp01(originalS - 0.10)
				}
				btnBgs[hoverIdx].FillColor = hslToNRGBA(originalH, s, clamp01(originalL-0.10))
			}
			btnBgs[hoverIdx].Refresh()
		}
		btn := newLabelButton("LABEL: "+name, func() {
			if activeIndex == idx {
				return
			}
			// 直接切换 activeIndex 并更新显示与样式（无动画）
			activeIndex = idx
			for j := range contentContainers {
				if contentContainers[j] != nil {
					if j == activeIndex {
						contentContainers[j].Show()
					} else {
						contentContainers[j].Hide()
					}
				}
			}
			// 当激活项变更时，需要更新所有 header 背景色（refreshHeaders 会处理）
			refreshHeaders()
		}, onHover)
		buttons[i] = btn

		status := canvas.NewText("status:OK", foregroundColorFor(hslToNRGBA(originalH, originalS, clamp01(originalL-0.10))))
		status.Alignment = fyne.TextAlignCenter
		statusTexts[i] = status

		// header：为按钮创建背景矩形以控制饱和度，按钮占左 70%，状态占右 30%
		sBtn := originalS
		if i != activeIndex {
			sBtn = clamp01(originalS - 0.10)
		}
		bg := canvas.NewRectangle(hslToNRGBA(originalH, sBtn, clamp01(originalL-0.10)))
		bg.SetMinSize(fyne.NewSize(0, 0))
		btnBgs[i] = bg

		btnLeft := container.NewStack(bg, btn)
		header := container.New(&twoColumnLayout{LeftRatio: 0.7}, btnLeft, status)

		// 内容容器（直接使用传入的 contents[i]）
		wrapped := contents[i]
		if wc, ok := wrapped.(*fyne.Container); ok {
			contentContainers[i] = wc
			// 默认隐藏非激活内容
			wc.Hide()
		} else {
			// 保证 contentContainers 索引对应，即使类型不同也填入 nil 占位
			contentContainers[i] = nil
		}

		// 单一 tab 容器：垂直排列 header 与 content
		tab := container.NewVBox(header, wrapped)
		tabContainers[i] = tab
	}

	// 初始化内容背景色（保持一致，不随激活状态变化），并显示激活 tab 的内容
	for j := range contentContainers {
		c := contentContainers[j]
		if c == nil || len(c.Objects) == 0 {
			continue
		}
		rect, ok := c.Objects[0].(*canvas.Rectangle)
		if !ok {
			continue
		}
		// 内容背景保持统一主色（仅轻微降低亮度），不随激活状态改变
		rect.FillColor = hslToNRGBA(originalH, originalS, clamp01(originalL-0.10))
		rect.Refresh()
	}
	if contentContainers[activeIndex] != nil {
		contentContainers[activeIndex].Show()
	}
	// 刷新 header 背景与按钮样式
	refreshHeaders()

	return container.New(&verticalTabsLayout{}, tabContainers...)
}

// verticalTabsLayout 实现纵向标签的自定义布局逻辑。
type verticalTabsLayout struct{}

/*
新版 Layout：每个 objects 项为一个 tab 容器（header, content）。

	仅显示并布局当前激活 tab 的 content，其余 tab 只显示 header。
*/
func (l *verticalTabsLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	count := len(objects)
	if count == 0 {
		return
	}

	headerHeights := make([]float32, count)
	var totalHeaderHeight float32
	activeIndex := -1

	// 先计算每个 header 的高度并累计，同时找到当前可见（active）的 content
	for i, obj := range objects {
		if obj == nil {
			continue
		}
		cont, ok := obj.(*fyne.Container)
		if !ok || len(cont.Objects) == 0 {
			// 回退到整体 MinSize
			min := obj.MinSize()
			headerHeights[i] = min.Height
			totalHeaderHeight += min.Height
			continue
		}
		headerMin := cont.Objects[0].MinSize()
		headerHeights[i] = headerMin.Height
		totalHeaderHeight += headerMin.Height

		// 判断 content 是否为当前激活（通过 Visible 判定）
		if len(cont.Objects) > 1 && cont.Objects[1].Visible() {
			activeIndex = i
		}
	}

	// 计算激活内容可用高度（剩余空间），并确保不小于其最小高度
	var contentHeight float32
	if activeIndex >= 0 {
		cont := objects[activeIndex].(*fyne.Container)
		if len(cont.Objects) > 1 {
			min := cont.Objects[1].MinSize()
			contentHeight = size.Height - totalHeaderHeight
			if contentHeight < min.Height {
				contentHeight = min.Height
			}
		} else {
			contentHeight = size.Height - totalHeaderHeight
		}
	}

	// 布局：先放 header，再为激活项放置 content
	y := float32(0)
	for i, obj := range objects {
		if obj == nil {
			continue
		}
		cont, ok := obj.(*fyne.Container)
		h := headerHeights[i]
		if ok && len(cont.Objects) > 0 {
			// header
			header := cont.Objects[0]
			header.Resize(fyne.NewSize(size.Width, h))
			header.Move(fyne.NewPos(0, y))
			y += h

			// 如果是激活项，布局 content
			if i == activeIndex && len(cont.Objects) > 1 {
				content := cont.Objects[1]
				content.Resize(fyne.NewSize(size.Width, contentHeight))
				content.Move(fyne.NewPos(0, y))
				y += contentHeight
			}
		} else {
			// 回退：把整个对象当成 header 处理
			obj.Resize(fyne.NewSize(size.Width, h))
			obj.Move(fyne.NewPos(0, y))
			y += h
		}
	}
}

/* 新版 MinSize：每个对象为 tab 容器（header, content）。计算所有 header 的高度之和，加上 content 中最大的最小高度。 */
func (l *verticalTabsLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var width float32
	var totalHeader float32
	var maxContentHeight float32

	for _, obj := range objects {
		if obj == nil {
			continue
		}
		cont, ok := obj.(*fyne.Container)
		if !ok || len(cont.Objects) == 0 {
			min := obj.MinSize()
			if min.Width > width {
				width = min.Width
			}
			totalHeader += min.Height
			continue
		}
		headerMin := cont.Objects[0].MinSize()
		totalHeader += headerMin.Height
		if headerMin.Width > width {
			width = headerMin.Width
		}
		if len(cont.Objects) > 1 {
			contentMin := cont.Objects[1].MinSize()
			if contentMin.Width > width {
				width = contentMin.Width
			}
			if contentMin.Height > maxContentHeight {
				maxContentHeight = contentMin.Height
			}
		}
	}

	return fyne.NewSize(width, totalHeader+maxContentHeight)
}
