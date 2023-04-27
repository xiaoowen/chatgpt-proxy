package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
)

type GPTSetting struct {
	APIKey             string `json:"api_key"`
	Model              string `json:"model"`
	MaxHistoryMessages int    `json:"max_history_messages"`
}

func (s *GPTSetting) Update() {
	if gptSetting.Model != s.Model {
		gptSetting.Model = s.Model
	}
	if gptSetting.APIKey != s.APIKey {
		gptSetting.APIKey = s.APIKey
	}
	if gptSetting.MaxHistoryMessages != s.MaxHistoryMessages {
		gptSetting.MaxHistoryMessages = s.MaxHistoryMessages
	}
	storeSetting(gptSetting)
}

var gptSetting *GPTSetting

func GetSettingWithUser(setting *UserSetting) *GPTSetting {
	if gptSetting == nil {
		if err := initGPTSetting(); err != nil {
			logrus.Fatalf("init setting failed. err: %s", err.Error())
		}
	}
	if setting != nil {
		if setting.APIKey != "" {
			gptSetting.APIKey = setting.APIKey
			// 只有用户自己定义了使用自有API Key，才可以支持选择模型
			if setting.Model != "" {
				gptSetting.Model = setting.Model
			}
		}
		if setting.MaxHistoryMessages > 0 {
			gptSetting.MaxHistoryMessages = setting.MaxHistoryMessages
		}
	}
	return gptSetting
}

func storeSetting(s *GPTSetting) {
	b, err := json.Marshal(s)
	if err != nil {
		logrus.Errorf("store setting failed. err: %s", err)
		return
	}
	if err := rd.Set(context.Background(), RedisSetting, string(b), redis.KeepTTL).Err(); err != nil {
		logrus.Errorf("store setting failed, err: %s", err)
	}
}

func initGPTSetting() error {
	data := rd.Get(context.Background(), RedisSetting).Val()
	if data == "" {
		return errors.New("undefined setting")
	}
	logrus.Infof("get setting data: %s", data)
	gptSetting = &GPTSetting{}
	return json.Unmarshal([]byte(data), gptSetting)
}
