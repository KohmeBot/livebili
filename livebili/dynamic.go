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
	"slices"
	"strings"
	"sync"
	"time"
)

func (b *biliPlugin) doCheckDynamic() error {
	var uids []int64
	for _, uid := range b.conf.Uids {
		uids = append(uids, uid)
	}
	errChan := make(chan error, len(uids))
	defer close(errChan)
	for i, uid := range uids {
		var groups []int64
		if _, ok := b.conf.GroupUids[uid]; ok {
			groups = b.conf.GroupUids[uid]
		} else {
			groups = slices.Collect(b.groups.RangeGroup())
		}
		gopool.Go(func() {
			errChan <- b.doCheckOneDynamic(uid, groups)
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

func (b *biliPlugin) doCheckOneDynamic(uid int64, groups []int64) error {

	resp, err := request.DoGet(fmt.Sprintf("https://api.bilibili.com/x/polymer/web-dynamic/v1/feed/space?host_mid=%d", uid), b.conf.Cookies)
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
					b.sendDynamic(ctx, group, &update)
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

func (b *biliPlugin) sendDynamic(ctx *zero.Ctx, group int64, dynamic *Dynamic) {
	switch dynamic.Type {
	case "DYNAMIC_TYPE_AV":
		b.onAv(ctx, group, &dynamic.Modules)
	case "DYNAMIC_TYPE_DRAW":
		b.onDraw(ctx, group, &dynamic.Modules)
	case "DYNAMIC_TYPE_WORD":
		b.onWord(ctx, group, &dynamic.Modules)
	default:
		logrus.Warnf("unknown dynamic type: %s", dynamic.Type)
	}
}

// 投稿了视频
func (b *biliPlugin) onAv(ctx *zero.Ctx, group int64, dynamic *DynamicModules) {
	userName := dynamic.ModuleAuthor.Name
	pubTime := dynamic.ModuleAuthor.PubTime

	title := dynamic.Archive.Title
	cover := dynamic.Archive.Cover
	url := dynamic.Archive.JumpUrl

	var msgChain chain.MessageChain
	msgChain.Split(
		message.AtAll(),
		message.Text(fmt.Sprintf("@%s", userName)),
		message.Text(fmt.Sprintf("%s投稿了视频", pubTime)),
		message.Text(fmt.Sprintf("【%s】", title)),
		message.Image(cover),
		message.Text(strings.TrimLeft(url, "//")),
	)
	if b.gn8Iv.IsNowDND() || !b.conf.AtAll {
		// 免打扰状态下去除at全员
		DeleteAtAll(&msgChain)
	}
	ctx.SendGroupMessage(group, msgChain)

}

// 带图动态
func (b *biliPlugin) onDraw(ctx *zero.Ctx, group int64, dynamic *DynamicModules) {
	userName := dynamic.ModuleAuthor.Name
	pubTime := dynamic.ModuleAuthor.PubTime

	text := dynamic.Desc.Text

	var imgMsg []message.Segment
	for _, item := range dynamic.Draw.Items {
		imgMsg = append(imgMsg, message.Image(item.Src))
	}

	var msgChain chain.MessageChain
	msgChain.Split(
		message.AtAll(),
		message.Text(fmt.Sprintf("@%s", userName)),
		message.Text(fmt.Sprintf("%s发布了动态", pubTime)),
		message.Text(text),
	)
	if b.gn8Iv.IsNowDND() || !b.conf.AtAll {
		DeleteAtAll(&msgChain)
	}
	if len(imgMsg) > 0 {
		msgChain.Line()
		msgChain.Split(imgMsg...)
	}
	ctx.SendGroupMessage(group, msgChain)

}

// 纯文字动态
func (b *biliPlugin) onWord(ctx *zero.Ctx, group int64, dynamic *DynamicModules) {
	userName := dynamic.ModuleAuthor.Name
	pubTime := dynamic.ModuleAuthor.PubTime

	text := dynamic.Desc.Text

	var msgChain chain.MessageChain
	msgChain.Split(
		message.AtAll(),
		message.Text(fmt.Sprintf("@%s", userName)),
		message.Text(fmt.Sprintf("%s发布了动态", pubTime)),
		message.Text(text),
	)
	if b.gn8Iv.IsNowDND() || !b.conf.AtAll {
		DeleteAtAll(&msgChain)
	}
	ctx.SendGroupMessage(group, msgChain)
}
