package livebili

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	_ "image/png"
	"os"
	"strings"
	"testing"
)

func TestRenderCardHTMLKeepsLongText(t *testing.T) {
	longText := strings.Repeat("这是一段需要自动换行而不是截断的动态正文。", 20)
	html, err := renderCardHTML(CardData{
		Theme:  "pink",
		Label:  "发布动态",
		Author: "测试用户",
		Body:   longText,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, longText) {
		t.Fatal("长文本没有完整写入 HTML")
	}
	if strings.Contains(html, "text-overflow: ellipsis") || strings.Contains(html, "-webkit-line-clamp") {
		t.Fatal("卡片样式不应截断长文本")
	}
}

func TestRenderCardHTMLEscapesDynamicText(t *testing.T) {
	html, err := renderCardHTML(CardData{
		Label:  "发布动态",
		Author: "<script>alert(1)</script>",
		Body:   "<img src=x onerror=alert(1)>",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html, "<script>alert(1)</script>") || strings.Contains(html, "<img src=x") {
		t.Fatal("动态文本没有经过 HTML 转义")
	}
}

func TestRenderCardHTMLKeepsGeneratedDataImages(t *testing.T) {
	const imageURL = "data:image/jpeg;base64,/9j/4AAQSkZJRg=="
	html, err := renderCardHTML(CardData{
		Label:   "发布动态",
		Author:  "测试用户",
		Avatar:  imageURL,
		Gallery: []string{imageURL},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html, "#ZgotmplZ") || !strings.Contains(html, imageURL) {
		t.Fatal("服务端生成的 data: 图片地址没有正确写入 HTML")
	}
}

func TestRenderCardHTMLProtectsExtremeGalleryAspectRatio(t *testing.T) {
	const imageURL = "data:image/png;base64,iVBORw0KGgo="
	item := newCardGalleryImage(imageURL, image.NewRGBA(image.Rect(0, 0, 2000, 500)))
	if !item.Contain {
		t.Fatal("极宽图片应使用完整显示模式")
	}

	html, err := renderCardHTML(CardData{
		Theme:        "pink",
		Label:        "发布动态",
		Author:       "测试用户",
		GalleryItems: []CardGalleryImage{item},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, `class="gallery-image-contain"`) || !strings.Contains(html, imageURL) {
		t.Fatal("极端宽高比图片没有使用无裁切样式")
	}
}

func TestRenderCardHTMLRendersRichTextInOrder(t *testing.T) {
	const imageURL = "data:image/png;base64,iVBORw0KGgo="
	html, err := renderCardHTML(CardData{
		Label:  "发布动态",
		Author: "测试用户",
		Title:  "有标题的动态",
		RichBody: []CardRichTextNode{
			{Text: "表情之前<script>"},
			{Image: imageURL, ImageAlt: "[测试表情]", ImageSize: 2},
			{Text: "表情之后"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "有标题的动态") {
		t.Fatal("动态标题没有写入 HTML")
	}
	if strings.Contains(html, "<script>") || !strings.Contains(html, "&lt;script&gt;") {
		t.Fatal("富文本中的普通文本没有经过 HTML 转义")
	}
	before := strings.Index(html, "表情之前")
	emoji := strings.Index(html, imageURL)
	after := strings.Index(html, "表情之后")
	if before < 0 || emoji < before || after < emoji {
		t.Fatal("富文本节点没有按原顺序渲染")
	}
	if !strings.Contains(html, "body-emoji-large") {
		t.Fatal("size=2 的表情没有使用大表情样式")
	}
}

func TestRenderCardHTMLRendersVideoDescriptionBelowCover(t *testing.T) {
	const imageURL = "data:image/jpeg;base64,/9j/4AAQSkZJRg=="
	html, err := renderCardHTML(CardData{
		Label:       "视频投稿",
		Author:      "测试用户",
		Title:       "视频标题",
		Cover:       imageURL,
		Description: "第一行简介\n第二行<script>",
		StatLabel:   "视频时长",
		StatValue:   "03:24",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html, "<script>") || !strings.Contains(html, "第二行&lt;script&gt;") {
		t.Fatal("视频简介没有经过 HTML 转义")
	}
	cover := strings.Index(html, `class="cover-wrap"`)
	description := strings.Index(html, `class="description-note"`)
	stat := strings.Index(html, `class="stat-note"`)
	if cover < 0 || description < cover || stat < description {
		t.Fatal("视频简介应位于封面之后、视频时长之前")
	}
}

func TestRenderCardHTMLOmitsEmptyVideoDescription(t *testing.T) {
	html, err := renderCardHTML(CardData{
		Label:       "视频投稿",
		Author:      "测试用户",
		Cover:       "data:image/jpeg;base64,/9j/4AAQSkZJRg==",
		Description: " \n\t ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html, `<section class="description-note">`) || strings.Contains(html, "视频简介") {
		t.Fatal("空视频简介不应占据卡片布局")
	}
}

func TestChromeAddr(t *testing.T) {
	tests := map[string]string{
		"":                        "",
		"127.0.0.1:9222/devtools": "ws://127.0.0.1:9222/devtools",
		"ws://chrome:9222":        "ws://chrome:9222",
		"wss://chrome.example":    "wss://chrome.example",
	}
	for input, want := range tests {
		if got := (Config{ChromeWs: input}).ChromeAddr(); got != want {
			t.Fatalf("ChromeAddr(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestRenderCardImageWithLocalChrome(t *testing.T) {
	if os.Getenv("LIVEBILI_TEST_CHROME") == "" {
		t.Skip("设置 LIVEBILI_TEST_CHROME=1 后运行本机 Chrome 截图测试")
	}

	makeImage := func(c color.RGBA) string {
		img := image.NewRGBA(image.Rect(0, 0, 640, 360))
		draw.Draw(img, img.Bounds(), &image.Uniform{C: c}, image.Point{}, draw.Src)
		dataURI, err := imageDataURI(img)
		if err != nil {
			t.Fatal(err)
		}
		return dataURI
	}

	imgBytes, err := renderCardImage(CardData{
		Theme:  "coral",
		Icon:   "✎",
		Label:  "发布动态",
		Author: "卡片预览用户",
		Meta:   "刚刚",
		Title:  "测试测试标题",
		Body: strings.Repeat(
			"迁移到 HTML 以后，正文会按照实际内容自动换行并撑高卡片，不会再被硬截断。\n",
			4,
		),
		Gallery: []string{
			makeImage(color.RGBA{R: 255, G: 145, B: 180, A: 255}),
			makeImage(color.RGBA{R: 115, G: 189, B: 232, A: 255}),
			makeImage(color.RGBA{R: 95, G: 201, B: 160, A: 255}),
		},
		Footer:      "哔哩哔哩 · 图文动态",
		Description: "testtstesatt\n测secess",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := image.Decode(bytes.NewReader(imgBytes)); err != nil {
		t.Fatalf("Chrome 返回的不是有效图片: %v", err)
	}
	if preview := os.Getenv("LIVEBILI_CARD_PREVIEW"); preview != "" {
		if err := os.WriteFile(preview, imgBytes, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
