package livebili

import (
	"encoding/json"
	"image"
	"reflect"
	"strings"
	"testing"
)

func TestOpusUnmarshalAndDynamicContent(t *testing.T) {
	const payload = `{
		"type":"MAJOR_TYPE_OPUS",
		"opus":{
			"jump_url":"//www.bilibili.com/opus/972972618448633857",
			"title":"你好我是haze",
			"summary":{
				"text":"正文[测试表情]",
				"rich_text_nodes":[
					{"text":"正文","type":"RICH_TEXT_NODE_TYPE_TEXT"},
					{"text":"[测试表情]","type":"RICH_TEXT_NODE_TYPE_EMOJI","emoji":{"size":2,"text":"[测试表情]","icon_url":"https://i0.hdslb.com/test.png"}}
				]
			},
			"pics":[{"url":"http://i0.hdslb.com/bfs/new_dyn/test.png","width":1080,"height":1920}]
		}
	}`

	var major Major
	if err := json.Unmarshal([]byte(payload), &major); err != nil {
		t.Fatal(err)
	}
	dynamic := DynamicModules{
		ModuleDynamic: ModuleDynamic{
			Major: major,
			Desc:  Desc{Text: "已弃用的 desc"},
		},
	}

	if got, want := dynamic.dynamicTitle(), "你好我是haze"; got != want {
		t.Fatalf("dynamicTitle() = %q, want %q", got, want)
	}
	if got, want := dynamic.dynamicText(), "正文[测试表情]"; got != want {
		t.Fatalf("dynamicText() = %q, want %q", got, want)
	}
	if got, want := dynamic.dynamicJumpURL(), "//www.bilibili.com/opus/972972618448633857"; got != want {
		t.Fatalf("dynamicJumpURL() = %q, want %q", got, want)
	}
	if got, want := dynamic.dynamicImageURLs(), []string{"http://i0.hdslb.com/bfs/new_dyn/test.png"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("dynamicImageURLs() = %#v, want %#v", got, want)
	}
	if got, want := major.Opus.Summary.RichTextNodes[1].Emoji.IconUrl, "https://i0.hdslb.com/test.png"; got != want {
		t.Fatalf("emoji icon_url = %q, want %q", got, want)
	}
}

func TestDynamicContentFallsBackToLegacyFields(t *testing.T) {
	dynamic := DynamicModules{
		ModuleDynamic: ModuleDynamic{
			Desc: Desc{Text: "旧版正文"},
			Major: Major{Draw: MajorDraw{Items: []DrawItem{
				{Src: "https://i0.hdslb.com/legacy.png"},
			}}},
		},
	}

	if got, want := dynamic.dynamicText(), "旧版正文"; got != want {
		t.Fatalf("dynamicText() = %q, want %q", got, want)
	}
	if got, want := dynamic.dynamicImageURLs(), []string{"https://i0.hdslb.com/legacy.png"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("dynamicImageURLs() = %#v, want %#v", got, want)
	}
}

func TestDynamicTextRebuildsFromRichTextNodes(t *testing.T) {
	dynamic := DynamicModules{ModuleDynamic: ModuleDynamic{Major: Major{
		Type: majorTypeOpus,
		Opus: Opus{Summary: OpusSummary{RichTextNodes: []RichTextNode{
			{Text: "前文"},
			{Emoji: &Emoji{Text: "[表情]"}},
			{OrigText: "后文"},
		}}},
	}}}

	if got, want := dynamic.dynamicText(), "前文[表情]后文"; got != want {
		t.Fatalf("dynamicText() = %q, want %q", got, want)
	}
}

func TestCardRichTextNodesTurnsEmojiIntoInlineImage(t *testing.T) {
	nodes := cardRichTextNodes([]RichTextNode{
		{Text: "前文", Type: "RICH_TEXT_NODE_TYPE_TEXT"},
		{Text: "[表情]", Type: richTextNodeTypeEmoji, Emoji: &Emoji{
			IconUrl: "//i0.hdslb.com/test.png",
			Size:    2,
		}},
		{Text: "后文", Type: "RICH_TEXT_NODE_TYPE_TEXT"},
	}, func(url string) (image.Image, error) {
		if got, want := url, "https://i0.hdslb.com/test.png"; got != want {
			t.Fatalf("fetch image URL = %q, want %q", got, want)
		}
		return image.NewRGBA(image.Rect(0, 0, 2, 2)), nil
	})

	if len(nodes) != 3 {
		t.Fatalf("len(cardRichTextNodes()) = %d, want 3", len(nodes))
	}
	if nodes[0].Text != "前文" || nodes[2].Text != "后文" {
		t.Fatalf("普通文本节点顺序不正确: %#v", nodes)
	}
	if !strings.HasPrefix(nodes[1].Image, "data:image/png;base64,") || nodes[1].ImageAlt != "[表情]" || nodes[1].ImageSize != 2 {
		t.Fatalf("表情节点转换结果不正确: %#v", nodes[1])
	}
}

func TestMessageJumpURLMatchesVideoDynamic(t *testing.T) {
	if got, want := messageJumpURL("//www.bilibili.com/opus/1"), "www.bilibili.com/opus/1"; got != want {
		t.Fatalf("messageJumpURL() = %q, want %q", got, want)
	}
}
