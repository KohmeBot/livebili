package livebili

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/kohmebot/livebili/request"
	"github.com/kohmebot/pkg/chain"
	"github.com/kohmebot/pkg/gopool"
	"slices"
	"time"

	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
	"gorm.io/gorm"
	"io"
)

func (b *biliPlugin) doCheckLive() error {
	live, err := b.checkLive(b.conf.Uids)
	if err != nil {
		return err
	}
	for _, info := range live.Data {
		uid := info.Uid
		var groups []int64
		if _, ok := b.conf.GroupUids[uid]; ok {
			groups = b.conf.GroupUids[uid]
		} else {
			groups = slices.Collect(b.groups.RangeGroup)
		}
		err = b.sendRoomInfo(&info, groups)
		if err != nil {
			return err
		}
	}
	return nil

}

func (b *biliPlugin) sendRoomInfo(info *RoomInfo, groups []int64) error {
	db, err := b.env.GetDB()
	if err != nil {
		return err
	}
	record := &LiveRecord{}
	err = db.First(&record, info.Uid).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 如果没有记录，插入一条新记录并设置状态
			record = &LiveRecord{
				Uid:         int64(info.Uid),
				IsLive:      IsLiving(info.LiveStatus),
				LastOffTime: time.Now(),
			}
			err = db.Create(&record).Error
		}
		return err // 处理其他查询错误
	}
	if err != nil {
		return err
	}

	lastStatus := record.IsLive
	living := IsLiving(info.LiveStatus)
	change := lastStatus != living
	if change {
		// 状态有变化，更新数据库
		record.IsLive = living
		if living {
			// 状态变化且直播，则记录开播时间
			record.LastLiveTime = time.Now()
		} else {
			// 状态变化且下播，则记录下播时间
			record.LastOffTime = time.Now()
		}
		if err := db.Save(&record).Error; err != nil {
			return err
		}
	}
	if !change {
		return nil
	}

	ava, err := request.FetchImage(info.Face)
	if err != nil {
		return err
	}
	cover, err := request.FetchImage(info.CoverFromUser)
	if err != nil {
		return err
	}
	liveImg := NewLiveImg(b.ttfPath, ava, info.Uname)

	if living {
		img, err := liveImg.DrawOnLive(cover, info.Title, record.LastOffTime)
		if err != nil {
			return err
		}
		imgB, err := ImageToBytes(img)
		if err != nil {
			return err
		}
		b.env.RangeBot(func(ctx *zero.Ctx) bool {
			var msgChain chain.MessageChain
			msgChain.Split(
				message.AtAll(),
				message.Text(b.conf.randChoseLiveTips()),
				message.ImageBytes(imgB),
				message.Text(fmt.Sprintf("https://live.bilibili.com/%d", info.RoomId)),
			)
			if b.gn8Iv.IsNowDND() || !b.conf.AtAll {
				// 免打扰状态下去除at全员
				DeleteAtAll(&msgChain)
			}
			for _, group := range groups {
				gopool.Go(func() {
					ctx.SendGroupMessage(group, msgChain)
				})
			}

			return true
		})
		return nil
	}
	if !living && b.conf.SendOff {
		img, err := liveImg.DrawOffLive(b.conf.randChoseOffTips(), record.LastLiveTime)
		if err != nil {
			return err
		}
		imgB, err := ImageToBytes(img)
		if err != nil {
			return err
		}
		b.env.RangeBot(func(ctx *zero.Ctx) bool {
			var msgChain chain.MessageChain
			msgChain.Split(
				message.ImageBytes(imgB),
			)
			for _, group := range groups {
				gopool.Go(func() {
					ctx.SendGroupMessage(group, msgChain)
				})
			}

			return true
		})
		return nil
	}

	return nil
}

func (b *biliPlugin) checkLive(uids []int64) (r LiveResp, err error) {
	data := map[string]interface{}{
		"uids": uids,
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return r, err
	}
	resp, err := request.DoPost("https://api.live.bilibili.com/room/v1/Room/get_status_info_by_uids", "application/json", bytes.NewBuffer(jsonData), b.conf.Cookies)
	if err != nil {
		return r, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return r, err
	}

	r = LiveResp{}
	err = json.Unmarshal(body, &r)
	if err != nil {
		return r, err
	}
	if r.Code != 0 {
		return r, fmt.Errorf("code: %d,msg: %s", r.Code, r.Msg)
	}
	return r, err
}
