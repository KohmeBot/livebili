package livebili

import (
	"fmt"
	"github.com/disintegration/imaging"
	"github.com/fogleman/gg"
	"image"
	"unicode/utf8"
)

type AvImg struct {
	// 字体文件路径
	TTFPath string
	// 头像
	Avatar image.Image
	// 昵称
	NickName string
}

func NewAvImg(ttf string, ava image.Image, nickName string) *AvImg {
	return &AvImg{TTFPath: ttf, Avatar: ava, NickName: nickName}
}

func (l *AvImg) DrawOnAv(cover image.Image, avTitle string, bv string, duration string) (image.Image, error) {
	dw, dh := 744, 700
	pw, ph := 704.0, 396.0
	dc := gg.NewContext(dw, dh)
	cover = imaging.Resize(cover, int(pw), int(ph), imaging.Lanczos)
	blurred := imaging.Blur(cover, 100.0)
	blurred = imaging.Resize(blurred, dw, dh, imaging.Lanczos)

	dc.DrawImage(blurred, 0, 0)

	// 图片框的位置和尺寸
	x, y := 20.0, 20.0
	radius := 30.0           // 圆角半径
	borderWidth := 1.5       // 边框宽度
	dc.SetRGBA(1, 1, 1, 0.5) // 设置边框颜色

	drawRoundedRect(dc, x-borderWidth, y-borderWidth, pw+2*borderWidth, ph+2*borderWidth, radius+borderWidth)
	dc.Fill()
	drawImageInRoundedRect(dc, cover, x, y, pw, ph, radius)
	img := dc.Image()
	dc = gg.NewContextForImage(img)

	dc.SetRGBA(1, 1, 1, 0.3)
	err := drawAv(dc, l.TTFPath, 20, 450, l.Avatar, l.NickName, avTitle, bv, duration)
	if err != nil {
		return nil, err
	}

	return dc.Image(), nil
}

func drawAv(dc *gg.Context, ttf string, x, y float64, img image.Image, name string, desc string, bv string, duration string) error {
	drawRoundedRect(dc, x, y, 704, 200, 30)
	dc.Fill()

	dc.SetRGB(0, 0, 0)
	err := dc.LoadFontFace(ttf, 30)
	if err != nil {
		return err
	}
	dc.DrawStringAnchored(name, x+60+60, y+60, 0, 0.5)

	err = dc.LoadFontFace(ttf, 20)
	if err != nil {
		return err
	}
	dc.SetRGBA(0, 0, 0, 0.6)
	dc.DrawStringAnchored(bv, x+704-30, y+30, 1, 0.5)

	err = dc.LoadFontFace(ttf, 15)
	if err != nil {
		return err
	}
	dc.SetRGBA(0, 0, 0, 0.6)

	dc.DrawStringAnchored(fmt.Sprintf("时长: %s", duration), x+704-30, y+170, 1, 0.5)

	err = dc.LoadFontFace(ttf, 20)
	if err != nil {
		return err
	}
	dc.SetRGBA(0, 0, 0, 0.8)

	var rByte []byte
	rCount := 0
	for _, r := range desc {
		if rCount >= 30 {
			rByte = append(rByte, []byte("...")...)
			break
		}
		rByte = utf8.AppendRune(rByte, r)
		rCount++
	}
	desc = string(rByte)

	dc.DrawStringAnchored(desc, x+30, y+60+75, 0, 0.5)

	// 绘制头像（圆形）
	avatarRadius := 40.0 // 头像的半径，比例可调整
	avatarCenterX := x + 60
	avatarCenterY := y + 60 // 头像位置，居上

	dc.SetRGB(1, 1, 1) // 设置头像背景颜色
	dc.NewSubPath()
	dc.DrawCircle(avatarCenterX, avatarCenterY, avatarRadius)
	dc.ClosePath()
	dc.Clip()

	img = imaging.Resize(img, 80, 80, imaging.Lanczos)

	dc.DrawImageAnchored(img, int(avatarCenterX), int(avatarCenterY), 0.5, 0.5)
	return nil
}
