package livebili

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/kohmebot/livebili/request"
	"github.com/kohmebot/pkg/chain"
	"github.com/kohmebot/pkg/gopool"
	"github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
	"gorm.io/gorm"
	"io"
	"strings"
	"sync"
	"time"
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
			time.Sleep(time.Duration(b.conf.CheckDuration) * time.Second)
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

	resp, err := request.DoGet(fmt.Sprintf("https://api.bilibili.com/x/polymer/web-dynamic/v1/feed/space?host_mid=%d", uid), string(b.conf.Cookies))
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
		Footer:    "哔哩哔哩 · 视频更新",
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
			message.Text(strings.TrimLeft(url, "//")),
		)
	} else {
		msgChain.Split(
			message.AtAll(),
			message.Text(fmt.Sprintf("@%s", userName)),
			message.Text(fmt.Sprintf("%s投稿了视频", pubTime)),
			message.ImageBytes(imgB),
			message.Text(strings.TrimLeft(url, "//")),
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
	text := dynamic.Desc.Text

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

	gallery := make([]string, 0, len(dynamic.Draw.Items))
	for _, item := range dynamic.Draw.Items {
		img, err := request.FetchImage(item.Src)
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
		gallery = append(gallery, dataURI)
	}

	imgBytes, err := renderCardImage(CardData{
		Theme:   "pink",
		Icon:    "✎",
		Label:   "发布动态",
		Avatar:  avatarData,
		Author:  userName,
		Meta:    pubTime,
		Body:    text,
		Gallery: gallery,
		Footer:  "哔哩哔哩 · 图文动态",
	}, b.conf.ChromeAddr())
	if err != nil {
		b.env.Error(ctx, err)
		b.sendDrawFallback(ctx, group, dynamic, atAll)
		return nil
	}

	var msgChain chain.MessageChain
	msgChain.Split(
		message.AtAll(),
		message.ImageBytes(imgBytes),
	)
	if b.gn8Iv.IsNowDND() || !atAll {
		DeleteAtAll(&msgChain)
	}
	ctx.SendGroupMessage(group, msgChain)
	return nil
}

func (b *biliPlugin) sendDrawFallback(ctx *zero.Ctx, group int64, dynamic *DynamicModules, atAll bool) {
	var imgMsg []message.Segment
	for _, item := range dynamic.Draw.Items {
		imgMsg = append(imgMsg, message.Image(item.Src))
	}

	var msgChain chain.MessageChain
	msgChain.Split(
		message.AtAll(),
		message.Text(fmt.Sprintf("@%s", dynamic.ModuleAuthor.Name)),
		message.Text(fmt.Sprintf("%s发布了动态", dynamic.ModuleAuthor.PubTime)),
		message.Text(dynamic.Desc.Text),
	)
	if b.gn8Iv.IsNowDND() || !atAll {
		DeleteAtAll(&msgChain)
	}
	if len(imgMsg) > 0 {
		msgChain.Line()
		msgChain.Split(imgMsg...)
	}
	ctx.SendGroupMessage(group, msgChain)
}

// 纯文字动态
func (b *biliPlugin) onWord(ctx *zero.Ctx, group int64, dynamic *DynamicModules, atAll bool) error {
	userName := dynamic.ModuleAuthor.Name
	pubTime := dynamic.ModuleAuthor.PubTime
	text := dynamic.Desc.Text
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
		Theme:  "grape",
		Icon:   "✎",
		Label:  "发布动态",
		Avatar: avatarData,
		Author: userName,
		Meta:   pubTime,
		Body:   text,
		Footer: "哔哩哔哩 · 文字动态",
	}, b.conf.ChromeAddr())
	if err != nil {
		b.env.Error(ctx, err)
		b.sendWordFallback(ctx, group, dynamic, atAll)
		return nil
	}

	var msgChain chain.MessageChain
	msgChain.Split(
		message.AtAll(),
		message.ImageBytes(imgBytes),
	)
	if b.gn8Iv.IsNowDND() || !atAll {
		DeleteAtAll(&msgChain)
	}
	ctx.SendGroupMessage(group, msgChain)
	return nil
}

func (b *biliPlugin) sendWordFallback(ctx *zero.Ctx, group int64, dynamic *DynamicModules, atAll bool) {
	var msgChain chain.MessageChain
	msgChain.Split(
		message.AtAll(),
		message.Text(fmt.Sprintf("@%s", dynamic.ModuleAuthor.Name)),
		message.Text(fmt.Sprintf("%s发布了动态", dynamic.ModuleAuthor.PubTime)),
		message.Text(dynamic.Desc.Text),
	)
	if b.gn8Iv.IsNowDND() || !atAll {
		DeleteAtAll(&msgChain)
	}
	ctx.SendGroupMessage(group, msgChain)
}
