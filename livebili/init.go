package livebili

import (
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"gorm.io/gorm"
	"time"
)

func (b *biliPlugin) init() error {
	conf := Config{}
	db, err := b.env.GetDB()
	if err != nil {
		return err
	}

	err = b.env.GetConf(&conf)
	if err != nil {
		return err
	}
	if conf.UIDs == nil {
		conf.UIDs = make(map[int64]PushConfig)
	}
	b.conf = conf
	err = b.initData(db)
	if err != nil {
		return err
	}
	go b.tickerLive()
	go b.tickerDynamic()
	go b.tickerFollower()

	return nil
}

func (b *biliPlugin) initData(db *gorm.DB) error {
	err := db.AutoMigrate(&LiveRecord{})
	if err != nil {
		return err
	}
	for _, uid := range b.conf.allUIDs() {
		record := &LiveRecord{Uid: uid}
		if err := db.Where("uid = ?", uid).First(record).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				record.IsLive = false
				if err := db.Create(record).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		}

	}

	return errors.Join(
		db.AutoMigrate(&DynamicRecord{}),
		db.AutoMigrate(&FollowerRecord{}),
	)
}

func (b *biliPlugin) tickerLive() {
	dur := time.Second * time.Duration(b.conf.CheckLiveDuration)
	t := time.NewTicker(dur)
	s := newErrorSender(b.sendError)
	for range t.C {
		logrus.Debugf("tock...%.2f sec", dur.Seconds())
		err := b.doCheckLive()
		if err != nil {
			err = fmt.Errorf("check live error: %w", err)
			logrus.Error(err.Error())
		}
		t.Reset(dur)
		s.Error(err)
	}

}

func (b *biliPlugin) tickerDynamic() {
	dur := time.Second * time.Duration(b.conf.CheckDynamicDuration)
	t := time.NewTicker(dur)
	s := newErrorSender(b.sendError)
	for range t.C {
		logrus.Debugf("tock...%.2f sec", dur.Seconds())
		err := b.doCheckDynamic()
		if err != nil {
			err = fmt.Errorf("check dynamic error: %w", err)
			logrus.Error(err.Error())
		}
		t.Reset(dur)
		s.Error(err)
	}
}

func (b *biliPlugin) tickerFollower() {
	dur := time.Second * time.Duration(b.conf.CheckFollowerDuration)
	t := time.NewTicker(dur)
	s := newErrorSender(b.sendError)
	for range t.C {
		logrus.Debugf("tock...%.2f sec", dur.Seconds())
		err := b.doCheckFollower()
		if err != nil {
			err = fmt.Errorf("check follower error: %w", err)
			logrus.Error(err.Error())
		}
		t.Reset(dur)
		s.Error(err)
	}
}

func (b *biliPlugin) sendError(err error) {
	b.env.UseBot(func(ctx *zero.Ctx) {
		b.env.Error(ctx, err)
	})
}
