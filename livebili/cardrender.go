package livebili

import (
	"bytes"
	"context"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"image"
	"image/jpeg"
	"image/png"
	"strings"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

const (
	cardViewportWidth  = 440
	cardViewportHeight = 900
	cardScale          = 2
)

//go:embed templates/*.html
var cardTemplateFS embed.FS

var cardTemplate = template.Must(
	template.New("card").Funcs(template.FuncMap{
		// 图片只接受本包 imageDataURI 生成的内嵌数据；显式标记后避免
		// html/template 将可信的 data: URL 替换成 #ZgotmplZ。
		"safeImageURL": func(src string) template.URL { return template.URL(src) },
	}).ParseFS(cardTemplateFS, "templates/*.html"),
)

// CardData 是所有 B 站通知卡片共用的视图模型。空字段不会占据布局空间。
// Body 和 Description 由 CSS 保留换行并自然撑高卡片，不做字符数截断。
type CardData struct {
	Theme          string
	Icon           string
	Label          string
	Badge          string
	Avatar         string
	AvatarFallback string
	Author         string
	Meta           string
	Title          string
	Body           string
	Description    string
	RichBody       []CardRichTextNode
	Cover          string
	Gallery        []string
	GalleryItems   []CardGalleryImage
	StatLabel      string
	StatValue      string
	Footer         string
}

// CardGalleryImage 为需要保护完整构图的极宽或极长图片提供无裁切渲染标记。
// Gallery 字段继续保留，以兼容已有调用方。
type CardGalleryImage struct {
	Source  string
	Contain bool
}

func newCardGalleryImage(source string, img image.Image) CardGalleryImage {
	item := CardGalleryImage{Source: source}
	if img == nil {
		return item
	}
	bounds := img.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return item
	}
	aspectRatio := float64(bounds.Dx()) / float64(bounds.Dy())
	item.Contain = aspectRatio > 1.8 || aspectRatio < 1/1.8
	return item
}

// CardRichTextNode 将正文拆成普通文本和可信的内嵌图片。
// Text 仍由 html/template 自动转义；Image 只能传入本包生成的 data URL。
type CardRichTextNode struct {
	Text      string
	Image     string
	ImageAlt  string
	ImageSize int
}

func renderCardHTML(data CardData) (string, error) {
	data.Description = strings.TrimSpace(data.Description)
	if strings.TrimSpace(data.Theme) == "" {
		data.Theme = "coral"
	}
	if strings.TrimSpace(data.Footer) == "" {
		data.Footer = "动态通知"
	}
	if strings.TrimSpace(data.AvatarFallback) == "" {
		data.AvatarFallback = firstTextRune(data.Author)
	}

	var buf bytes.Buffer
	if err := cardTemplate.ExecuteTemplate(&buf, "card", data); err != nil {
		return "", fmt.Errorf("渲染卡片 HTML 失败: %w", err)
	}
	return buf.String(), nil
}

func firstTextRune(s string) string {
	for _, r := range strings.TrimSpace(s) {
		return string(r)
	}
	return "B"
}

// renderCardImage 使用本机 Chrome；配置 chromeAddr 时则连接远程 Chrome。
// 页面内容和图片均已内嵌，因此远程 Chrome 不需要访问调用方文件系统。
func renderCardImage(data CardData, chromeAddr string) ([]byte, error) {
	htmlData, err := renderCardHTML(data)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	chromeAddr = strings.TrimSpace(chromeAddr)
	if chromeAddr != "" {
		ctx, cancel = chromedp.NewRemoteAllocator(ctx, chromeAddr)
		defer cancel()
	}
	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()

	var imageBytes []byte
	var resourcesReady bool
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(cardViewportWidth, cardViewportHeight, chromedp.EmulateScale(cardScale)),
		chromedp.Navigate("about:blank"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			frameTree, err := page.GetFrameTree().Do(ctx)
			if err != nil {
				return err
			}
			return page.SetDocumentContent(frameTree.Frame.ID, htmlData).Do(ctx)
		}),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Evaluate(`Promise.all(Array.from(document.images).map(function(img) {
			if (img.complete) return Promise.resolve();
			return new Promise(function(resolve) {
				img.addEventListener('load', resolve, {once:true});
				img.addEventListener('error', resolve, {once:true});
			});
		})).then(function() { return document.fonts ? document.fonts.ready : true; }).then(function() { return true; })`,
			&resourcesReady,
			func(p *runtime.EvaluateParams) *runtime.EvaluateParams { return p.WithAwaitPromise(true) },
		),
		chromedp.Screenshot(".page", &imageBytes, chromedp.ByQuery),
	)
	if err != nil {
		return nil, fmt.Errorf("Chrome 卡片截图失败: %w", err)
	}
	if !resourcesReady {
		return nil, fmt.Errorf("Chrome 卡片资源未加载完成")
	}
	return imageBytes, nil
}

func imageDataURI(img image.Image) (string, error) {
	if img == nil {
		return "", nil
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		return "", fmt.Errorf("编码卡片图片失败: %w", err)
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func pngImageDataURI(img image.Image) (string, error) {
	if img == nil {
		return "", nil
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", fmt.Errorf("编码卡片 PNG 图片失败: %w", err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func durationText(since time.Time) string {
	if since.IsZero() {
		return "刚刚"
	}
	d := time.Since(since)
	if d < 0 {
		d = 0
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	if hours > 0 {
		return fmt.Sprintf("%d时%d分%d秒", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%d分%d秒", minutes, seconds)
	}
	return fmt.Sprintf("%d秒", seconds)
}
