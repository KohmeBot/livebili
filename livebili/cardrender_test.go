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
		Theme:  "pink",
		Icon:   "✎",
		Label:  "发布动态",
		Author: "卡片预览用户",
		Meta:   "刚刚",
		Body: strings.Repeat(
			"迁移到 HTML 以后，正文会按照实际内容自动换行并撑高卡片，不会再被硬截断。\n",
			4,
		),
		Gallery: []string{
			makeImage(color.RGBA{R: 255, G: 145, B: 180, A: 255}),
			makeImage(color.RGBA{R: 115, G: 189, B: 232, A: 255}),
			makeImage(color.RGBA{R: 95, G: 201, B: 160, A: 255}),
		},
		Footer: "哔哩哔哩 · 图文动态",
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
