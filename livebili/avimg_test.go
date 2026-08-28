package livebili

import (
	"github.com/kohmebot/livebili/request"
	"image"
	"image/png"
	"os"
	"testing"
)

func TestAVImg(t *testing.T) {
	ava, err := request.FetchImage("https://i0.hdslb.com/bfs/face/51b3e812b74d5bc4a541e172af64c55b6828d106.jpg")
	if err != nil {
		t.Fatal(err)
	}
	coverImg, err := request.FetchImage("http://i1.hdslb.com/bfs/archive/fef3e5a9ffa0649b2d2db3bf3a4af20db87f1224.jpg")
	if err != nil {
		t.Fatal(err)
	}

	avImg := NewAvImg("msyh.ttc", ava, "脆易折")

	i, err := avImg.DrawOnAv(coverImg, "互联网神人百科", "BV19s6cBxETi", "12:42")
	if err != nil {
		t.Fatal(err)
	}
	SavePNG(i, "./test.png")
}

func SavePNG(img image.Image, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return png.Encode(f, img)
}
