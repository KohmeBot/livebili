package livebili

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/kohmebot/livebili/request"
	"github.com/kohmebot/pkg/chain"
	"github.com/kohmebot/pkg/gopool"
	"github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
	"gorm.io/gorm"
)

func (b *biliPlugin) doCheckDynamic() error {
	uids := b.conf.enabledUIDs(func(push PushConfig) bool {
		return push.SendDynamic
	})
	errChan := make(chan error, len(uids))
	defer close(errChan)
	for i, uid := range uids {
		push, _ := b.conf.pushFor(uid)
		groups := b.groupsFor(push)
		gopool.Go(func() {
			errChan <- b.doCheckOneDynamic(uid, groups, push.AtAll)
		})
		if i < len(uids)-1 {
			sDur := time.Duration(b.conf.CheckDuration) * time.Second
			sDur += RandSecond(6)
			time.Sleep(sDur)
		}
	}
	var err error
	for i := 0; i < len(uids); i++ {
		doErr := <-errChan
		if doErr != nil {
			err = errors.Join(err, fmt.Errorf("checkDynamicError %w", doErr))
		}
	}
	return err
}

func (b *biliPlugin) doCheckOneDynamic(uid int64, groups []int64, atAll bool) error {

	resp, err := request.DoGet(fmt.Sprintf("https://api.bilibili.com/x/polymer/web-dynamic/v1/feed/space?features=itemOpusStyle,listOnlyfans,opusBigCover,onlyfansVote,forwardListHidden,decorationCard,commentsNewVersion,onlyfansAssetsV2,ugcDelete,onlyfansQaCard,avatarAutoTheme,sunflowerStyle,cardsEnhance,eva3CardOpus,eva3CardVideo,eva3CardComment,eva3CardUser&host_mid=%d", uid), string(b.conf.Cookies))
	if err != nil {
		return err
	}
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	dynamic := DynamicResp{}
	err = json.Unmarshal(body, &dynamic)
	if err != nil {
		return err
	}
	if dynamic.Code != 0 {
		return fmt.Errorf("code: %d,msg: %s", dynamic.Code, dynamic.Message)
	}

	updates, err := b.updateDynamic(uid, &dynamic)
	if err != nil {
		return err
	}
	if len(updates) <= 0 {
		return nil
	}
	wg := sync.WaitGroup{}

	b.env.UseBot(func(ctx *zero.Ctx) {
		for _, group := range groups {
			wg.Add(1)
			gopool.Go(func() {
				defer wg.Done()
				for _, update := range updates {
					b.sendDynamic(ctx, group, &update, atAll)
					// 发送每个动态间等2s
					time.Sleep(2 * time.Second)
				}
			})
		}

	})
	wg.Wait()
	return nil

}

func (b *biliPlugin) updateDynamic(uid int64, dynamic *DynamicResp) (updates []Dynamic, err error) {
	if len(dynamic.Data.Items) <= 0 {
		return nil, nil
	}
	dynamic.TopReSort()
	db, err := b.env.GetDB()
	if err != nil {
		return nil, err
	}
	latestPubTime := dynamic.Data.Items[0].Modules.PutTs
	record := &DynamicRecord{}
	record.Uid = uid
	err = db.First(&record, uid).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 如果没有记录，插入一条新记录
			record = &DynamicRecord{
				Uid:         uid,
				LastPubTime: latestPubTime,
			}
			err = db.Create(&record).Error
		}
		return nil, err
	}

	// 说明动态未更新
	if record.LastPubTime >= latestPubTime {
		return nil, nil
	}

	// 找出更新的动态
	for _, item := range dynamic.Data.Items {
		if item.Modules.PutTs > record.LastPubTime {
			updates = append(updates, item)
		} else {
			break
		}
	}

	if len(updates) > 0 {
		record.LastPubTime = updates[0].Modules.PutTs
		err = db.Save(&record).Error
	}

	return updates, err

}

func (b *biliPlugin) sendDynamic(ctx *zero.Ctx, group int64, dynamic *Dynamic, atAll bool) {
	var err error
	switch dynamic.Type {
	case "DYNAMIC_TYPE_AV":
		err = b.onAv(ctx, group, &dynamic.Modules, atAll)
	case "DYNAMIC_TYPE_DRAW":
		err = b.onDraw(ctx, group, &dynamic.Modules, atAll)
	case "DYNAMIC_TYPE_WORD":
		err = b.onWord(ctx, group, &dynamic.Modules, atAll)
	default:
		logrus.Warnf("unknown dynamic type: %s", dynamic.Type)
	}

	if err != nil {
		b.env.Error(ctx, err)
	}

}

const (
	majorTypeOpus         = "MAJOR_TYPE_OPUS"
	richTextNodeTypeEmoji = "RICH_TEXT_NODE_TYPE_EMOJI"
)

func (dynamic *DynamicModules) hasOpus() bool {
	return dynamic.Major.Type == majorTypeOpus ||
		dynamic.Opus.JumpUrl != "" ||
		dynamic.Opus.Title != "" ||
		dynamic.Opus.Summary.Text != "" ||
		len(dynamic.Opus.Summary.RichTextNodes) > 0 ||
		len(dynamic.Opus.Pics) > 0
}

func (dynamic *DynamicModules) dynamicTitle() string {
	if !dynamic.hasOpus() {
		return ""
	}
	return dynamic.Opus.Title
}

func (dynamic *DynamicModules) dynamicText() string {
	if !dynamic.hasOpus() {
		return dynamic.Desc.Text
	}
	if dynamic.Opus.Summary.Text != "" {
		return dynamic.Opus.Summary.Text
	}

	var text strings.Builder
	for _, node := range dynamic.Opus.Summary.RichTextNodes {
		text.WriteString(richTextNodeText(node))
	}
	return text.String()
}

func (dynamic *DynamicModules) dynamicJumpURL() string {
	if !dynamic.hasOpus() {
		return ""
	}
	return dynamic.Opus.JumpUrl
}

func (dynamic *DynamicModules) dynamicImageURLs() []string {
	if dynamic.hasOpus() && len(dynamic.Opus.Pics) > 0 {
		urls := make([]string, 0, len(dynamic.Opus.Pics))
		for _, pic := range dynamic.Opus.Pics {
			if pic.Url != "" {
				urls = append(urls, pic.Url)
			}
		}
		return urls
	}

	urls := make([]string, 0, len(dynamic.Draw.Items))
	for _, item := range dynamic.Draw.Items {
		if item.Src != "" {
			urls = append(urls, item.Src)
		}
	}
	return urls
}

func (dynamic *DynamicModules) cardRichBody() []CardRichTextNode {
	if !dynamic.hasOpus() || len(dynamic.Opus.Summary.RichTextNodes) == 0 {
		return nil
	}
	return cardRichTextNodes(dynamic.Opus.Summary.RichTextNodes, request.FetchImage)
}

func cardRichTextNodes(richTextNodes []RichTextNode, fetchImage func(string) (image.Image, error)) []CardRichTextNode {
	nodes := make([]CardRichTextNode, 0, len(richTextNodes))
	for _, node := range richTextNodes {
		text := richTextNodeText(node)
		if node.Type != richTextNodeTypeEmoji || node.Emoji == nil || node.Emoji.IconUrl == "" {
			if text != "" {
				nodes = append(nodes, CardRichTextNode{Text: text})
			}
			continue
		}

		img, err := fetchImage(absoluteRemoteURL(node.Emoji.IconUrl))
		if err != nil {
			logrus.Warnf("获取动态表情失败: %v", err)
			if text != "" {
				nodes = append(nodes, CardRichTextNode{Text: text})
			}
			continue
		}
		dataURI, err := pngImageDataURI(img)
		if err != nil {
			logrus.Warnf("编码动态表情失败: %v", err)
			if text != "" {
				nodes = append(nodes, CardRichTextNode{Text: text})
			}
			continue
		}
		nodes = append(nodes, CardRichTextNode{
			Image:     dataURI,
			ImageAlt:  text,
			ImageSize: node.Emoji.Size,
		})
	}
	return nodes
}

func richTextNodeText(node RichTextNode) string {
	if node.Text != "" {
		return node.Text
	}
	if node.OrigText != "" {
		return node.OrigText
	}
	if node.Emoji != nil {
		return node.Emoji.Text
	}
	return ""
}

func absoluteRemoteURL(url string) string {
	if strings.HasPrefix(url, "//") {
		return "https:" + url
	}
	return url
}

func messageJumpURL(url string) string {
	return strings.TrimLeft(url, "//")
}

// 投稿了视频
func (b *biliPlugin) onAv(ctx *zero.Ctx, group int64, dynamic *DynamicModules, atAll bool) error {
	userName := dynamic.ModuleAuthor.Name
	pubTime := dynamic.ModuleAuthor.PubTime
	face := dynamic.ModuleAuthor.Face

	title := dynamic.Archive.Title
	cover := dynamic.Archive.Cover
	url := dynamic.Archive.JumpUrl
	bv := dynamic.Archive.BvID
	duration := dynamic.Archive.DurationText

	ava, err := request.FetchImage(face)
	if err != nil {
		return err
	}
	coverImg, err := request.FetchImage(cover)
	if err != nil {
		return err
	}

	var msgChain chain.MessageChain
	avatarData, err := imageDataURI(ava)
	if err != nil {
		return err
	}
	coverData, err := imageDataURI(coverImg)
	if err != nil {
		return err
	}
	imgB, err := renderCardImage(CardData{
		Theme:     "coral",
		Icon:      "▶",
		Label:     "视频投稿",
		Badge:     bv,
		Avatar:    avatarData,
		Author:    userName,
		Meta:      fmt.Sprintf("%s投稿了视频", pubTime),
		Title:     title,
		Cover:     coverData,
		StatLabel: "视频时长",
		StatValue: duration,
		Footer:    "视频更新",
	}, b.conf.ChromeAddr())

	if err != nil {
		b.env.Error(ctx, err)
		// Chrome 生成失败时仍发送原有文字消息，避免漏掉动态。
		msgChain.Split(
			message.AtAll(),
			message.Text(fmt.Sprintf("@%s", userName)),
			message.Text(fmt.Sprintf("%s投稿了视频", pubTime)),
			message.Text(fmt.Sprintf("【%s】", title)),
			message.Image(cover),
			message.Text(messageJumpURL(url)),
		)
	} else {
		msgChain.Split(
			message.AtAll(),
			message.Text(fmt.Sprintf("@%s", userName)),
			message.Text(fmt.Sprintf("%s投稿了视频", pubTime)),
			message.ImageBytes(imgB),
			message.Text(messageJumpURL(url)),
		)
	}

	if b.gn8Iv.IsNowDND() || !atAll {
		// 免打扰状态下去除at全员
		DeleteAtAll(&msgChain)
	}
	ctx.SendGroupMessage(group, msgChain)
	return nil
}

// 带图动态
func (b *biliPlugin) onDraw(ctx *zero.Ctx, group int64, dynamic *DynamicModules, atAll bool) error {
	userName := dynamic.ModuleAuthor.Name
	pubTime := dynamic.ModuleAuthor.PubTime
	title := dynamic.dynamicTitle()
	text := dynamic.dynamicText()
	richBody := dynamic.cardRichBody()

	avatarData := ""
	if dynamic.ModuleAuthor.Face != "" {
		ava, err := request.FetchImage(dynamic.ModuleAuthor.Face)
		if err == nil {
			avatarData, err = imageDataURI(ava)
		}
		if err != nil {
			logrus.Warnf("获取动态头像失败: %v", err)
		}
	}

	imageURLs := dynamic.dynamicImageURLs()
	gallery := make([]CardGalleryImage, 0, len(imageURLs))
	for _, imageURL := range imageURLs {
		img, err := request.FetchImage(absoluteRemoteURL(imageURL))
		if err != nil {
			b.env.Error(ctx, err)
			b.sendDrawFallback(ctx, group, dynamic, atAll)
			return nil
		}
		dataURI, err := imageDataURI(img)
		if err != nil {
			b.env.Error(ctx, err)
			b.sendDrawFallback(ctx, group, dynamic, atAll)
			return nil
		}
		gallery = append(gallery, newCardGalleryImage(dataURI, img))
	}

	imgBytes, err := renderCardImage(CardData{
		Theme:        "pink",
		Icon:         "✎",
		Label:        "发布动态",
		Avatar:       avatarData,
		Author:       userName,
		Meta:         pubTime,
		Title:        title,
		Body:         text,
		RichBody:     richBody,
		GalleryItems: gallery,
		Footer:       "动态",
	}, b.conf.ChromeAddr())
	if err != nil {
		b.env.Error(ctx, err)
		b.sendDrawFallback(ctx, group, dynamic, atAll)
		return nil
	}

	var msgChain chain.MessageChain
	segments := []message.Segment{
		message.AtAll(),
		message.ImageBytes(imgBytes),
	}
	if url := dynamic.dynamicJumpURL(); url != "" {
		segments = append(segments, message.Text(messageJumpURL(url)))
	}
	msgChain.Split(segments...)
	if b.gn8Iv.IsNowDND() || !atAll {
		DeleteAtAll(&msgChain)
	}
	ctx.SendGroupMessage(group, msgChain)
	return nil
}

func (b *biliPlugin) sendDrawFallback(ctx *zero.Ctx, group int64, dynamic *DynamicModules, atAll bool) {
	var imgMsg []message.Segment
	for _, imageURL := range dynamic.dynamicImageURLs() {
		imgMsg = append(imgMsg, message.Image(absoluteRemoteURL(imageURL)))
	}

	var msgChain chain.MessageChain
	segments := []message.Segment{
		message.AtAll(),
		message.Text(fmt.Sprintf("@%s", dynamic.ModuleAuthor.Name)),
		message.Text(fmt.Sprintf("%s发布了动态", dynamic.ModuleAuthor.PubTime)),
	}
	if title := dynamic.dynamicTitle(); title != "" {
		segments = append(segments, message.Text(fmt.Sprintf("【%s】", title)))
	}
	if text := dynamic.dynamicText(); text != "" {
		segments = append(segments, message.Text(text))
	}
	msgChain.Split(segments...)
	if b.gn8Iv.IsNowDND() || !atAll {
		DeleteAtAll(&msgChain)
	}
	if len(imgMsg) > 0 {
		msgChain.Line()
		msgChain.Split(imgMsg...)
	}
	if url := dynamic.dynamicJumpURL(); url != "" {
		msgChain.Split(message.Text(messageJumpURL(url)))
	}
	ctx.SendGroupMessage(group, msgChain)
}

// 纯文字动态
func (b *biliPlugin) onWord(ctx *zero.Ctx, group int64, dynamic *DynamicModules, atAll bool) error {
	userName := dynamic.ModuleAuthor.Name
	pubTime := dynamic.ModuleAuthor.PubTime
	title := dynamic.dynamicTitle()
	text := dynamic.dynamicText()
	richBody := dynamic.cardRichBody()
	avatarData := ""
	if dynamic.ModuleAuthor.Face != "" {
		ava, err := request.FetchImage(dynamic.ModuleAuthor.Face)
		if err == nil {
			avatarData, err = imageDataURI(ava)
		}
		if err != nil {
			logrus.Warnf("获取动态头像失败: %v", err)
		}
	}

	imgBytes, err := renderCardImage(CardData{
		Theme:    "grape",
		Icon:     "✎",
		Label:    "发布动态",
		Avatar:   avatarData,
		Author:   userName,
		Meta:     pubTime,
		Title:    title,
		Body:     text,
		RichBody: richBody,
		Footer:   "动态",
	}, b.conf.ChromeAddr())
	if err != nil {
		b.env.Error(ctx, err)
		b.sendWordFallback(ctx, group, dynamic, atAll)
		return nil
	}

	var msgChain chain.MessageChain
	segments := []message.Segment{
		message.AtAll(),
		message.ImageBytes(imgBytes),
	}
	if url := dynamic.dynamicJumpURL(); url != "" {
		segments = append(segments, message.Text(messageJumpURL(url)))
	}
	msgChain.Split(segments...)
	if b.gn8Iv.IsNowDND() || !atAll {
		DeleteAtAll(&msgChain)
	}
	ctx.SendGroupMessage(group, msgChain)
	return nil
}

func (b *biliPlugin) sendWordFallback(ctx *zero.Ctx, group int64, dynamic *DynamicModules, atAll bool) {
	var msgChain chain.MessageChain
	segments := []message.Segment{
		message.AtAll(),
		message.Text(fmt.Sprintf("@%s", dynamic.ModuleAuthor.Name)),
		message.Text(fmt.Sprintf("%s发布了动态", dynamic.ModuleAuthor.PubTime)),
	}
	if title := dynamic.dynamicTitle(); title != "" {
		segments = append(segments, message.Text(fmt.Sprintf("【%s】", title)))
	}
	if text := dynamic.dynamicText(); text != "" {
		segments = append(segments, message.Text(text))
	}
	if url := dynamic.dynamicJumpURL(); url != "" {
		segments = append(segments, message.Text(messageJumpURL(url)))
	}
	msgChain.Split(segments...)
	if b.gn8Iv.IsNowDND() || !atAll {
		DeleteAtAll(&msgChain)
	}
	ctx.SendGroupMessage(group, msgChain)
}
