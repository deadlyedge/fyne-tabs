package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

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
	buttons := make([]*widget.Button, len(names))
	statusTexts := make([]*canvas.Text, len(names))
	contentContainers := make([]*fyne.Container, len(names))
	tabContainers := make([]fyne.CanvasObject, len(names))

	refreshHeaders := func() {
		for i := range buttons {
			if buttons[i] == nil {
				continue
			}
			desired := widget.MediumImportance
			if i == activeIndex {
				desired = widget.HighImportance
			}
			if buttons[i].Importance != desired {
				buttons[i].Importance = desired
				buttons[i].Refresh()
			}
		}
	}

	for i, name := range names {
		idx := i
		// 可点击的 label（使用 Button 以获得按压样式）
		btn := widget.NewButton("LABEL: "+name, func() {
			if activeIndex == idx {
				return
			}

			// 更新所有内容的背景色（根据当前激活索引决定是否降低饱和度）
			for j := range contentContainers {
				c := contentContainers[j]
				if c == nil || len(c.Objects) == 0 {
					continue
				}
				rect, ok := c.Objects[0].(*canvas.Rectangle)
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

			// 已移除动画：直接切换 activeIndex 并更新显示与样式（无动画）
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
			refreshHeaders()
		})
		buttons[i] = btn

		status := canvas.NewText("status:OK", foregroundColorFor(hslToNRGBA(originalH, originalS, clamp01(originalL-0.10))))
		status.Alignment = fyne.TextAlignTrailing
		statusTexts[i] = status

		// header：按钮占左 70%，状态占右 30%
		header := container.New(&twoColumnLayout{LeftRatio: 0.7}, container.NewStack(btn), status)

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

	// 初始化背景色并显示激活 tab 的内容
	for j := range contentContainers {
		c := contentContainers[j]
		if c == nil || len(c.Objects) == 0 {
			continue
		}
		rect, ok := c.Objects[0].(*canvas.Rectangle)
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
	if contentContainers[activeIndex] != nil {
		contentContainers[activeIndex].Show()
	}
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
