package main

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

// buildTabContentHSL 根据标题与 HSL 颜色构建单个标签页内容（内部使用 HSL）。
// 返回值为 *fyne.Container，结构为：index 0 = background (*canvas.Rectangle)，
// index 1 = inner content（任意 fyne.CanvasObject，用于动画移动）。
func buildTabContentHSL(title string, h, s, l float64) *fyne.Container {
	// label 背景在基础色上降低 10% 的亮度
	labelL := clamp01(l - 0.10)
	labelBg := hslToNRGBA(h, s, labelL)
	background := canvas.NewRectangle(labelBg)
	background.SetMinSize(fyne.NewSize(0, 0))

	contentText := canvas.NewText("CONTENT: "+title, foregroundColorFor(labelBg))
	contentText.Alignment = fyne.TextAlignLeading

	// 使用单一整体容器承载未来内容（不分栏）
	content := container.NewStack(contentText)

	return container.NewStack(background, content)
}

/* 辅助：获取内容容器内的“inner content”对象（按 buildTabContentHSL 的结构，index 1 为 content） */
func getInnerContent(c *fyne.Container) fyne.CanvasObject {
	if c == nil {
		return nil
	}
	if len(c.Objects) > 1 {
		return c.Objects[1]
	}
	return nil
}

/* 辅助：使用滑动动画将 oldObj 向左滑出并将 newObj 从右侧滑入。
   left=true 表示旧内容向左滑出（新内容从右侧进入）。动画结束后隐藏旧内容并重置 new 内部位置。 */
func animateSlide(oldObj, newObj fyne.CanvasObject, left bool) {
	oldC, _ := oldObj.(*fyne.Container)
	newC, _ := newObj.(*fyne.Container)

	// 计算动画区域宽度，优先使用旧内容的宽度
	var width float32
	if oldC != nil {
		width = oldC.Size().Width
	} else if newC != nil {
		width = newC.Size().Width
	}
	// 若无法获得宽度，退化为直接切换
	if width == 0 {
		if oldObj != nil {
			oldObj.Hide()
		}
		if newObj != nil {
			newObj.Show()
		}
		return
	}

	oldInner := getInnerContent(oldC)
	newInner := getInnerContent(newC)

	// 确保起始位置
	if oldInner != nil {
		oldInner.Move(fyne.NewPos(0, 0))
	}
	if newInner != nil {
		if left {
			newInner.Move(fyne.NewPos(width, 0))
		} else {
			newInner.Move(fyne.NewPos(-width, 0))
		}
	}

	// 创建动画（在主循环中执行）
	duration := 240 * time.Millisecond
	anim := fyne.NewAnimation(duration, func(progress float32) {
		var oldX, newX float32
		if left {
			oldX = -progress * width
			newX = width - progress*width
		} else {
			oldX = progress * width
			newX = -width + progress*width
		}
		if oldInner != nil {
			oldInner.Move(fyne.NewPos(oldX, 0))
		}
		if newInner != nil {
			newInner.Move(fyne.NewPos(newX, 0))
		}
	})
	anim.Start()

	// 在动画结束后隐藏旧内容并把 newInner 位置重置为 0
	go func() {
		time.Sleep(duration)
		fyne.Do(func() {
			if oldObj != nil {
				oldObj.Hide()
			}
			if newInner != nil {
				newInner.Move(fyne.NewPos(0, 0))
			}
		})
	}()
}
