package request

import (
	"fmt"
	"image"
	"io"
	"net/http"

	// image.Decode only recognizes formats whose decoder packages are registered.
	// Keep all formats accepted by FetchImage explicit in this package.
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

func setHeader(req *http.Request, cookies string) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.bilibili.com")
	req.Header.Set("Cookie", cookies)
}

func DoGet(url string, cookies string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	setHeader(req, cookies)

	// 发送请求
	client := &http.Client{}
	return client.Do(req)
}

func DoPost(url, contentType string, body io.Reader, cookies string) (*http.Response, error) {
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	setHeader(req, cookies)
	client := &http.Client{}
	return client.Do(req)
}

// FetchImage 从给定的URL下载图像，并返回 image.Image 对象
func FetchImage(url string) (image.Image, error) {
	// 发送 GET 请求
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image: %w", err)
	}
	defer resp.Body.Close()

	// 检查 HTTP 状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch image: status code %d", resp.StatusCode)
	}

	// 读取并解码图像数据
	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image (content type %q): %w", resp.Header.Get("Content-Type"), err)
	}

	return img, nil
}
