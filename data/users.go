package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
)

type CostUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type Cost struct {
	Usage *CostUsage `json:"usage"`
	Cost  float64    `json:"cost"` // 根据 total_tokens 计算
}

type UserSetting struct {
	APIKey             string            `json:"api_key,omitempty"`
	Model              string            `json:"model,omitempty"`
	MaxHistoryMessages int               `json:"max_history_messages,omitempty"`
	Custom             map[string]string `json:"custom,omitempty"`
}

type User struct {
	Token           string       `json:"token"`
	Nickname        string       `json:"nickname"`
	Avatar          string       `json:"avatar"`
	Admin           bool         `json:"is_admin,omitempty"`
	ExpireTimestamp int64        `json:"expire_timestamp"`
	Cost            *Cost        `json:"cost,omitempty"`
	Setting         *UserSetting `json:"setting,omitempty"`
}

func (u *User) New() error {
	if u.Token == "" {
		return errors.New("undefined token")
	}
	u.Cost = &Cost{Cost: 0, Usage: &CostUsage{PromptTokens: 0, CompletionTokens: 0, TotalTokens: 0}}
	u.Setting = &UserSetting{}
	return storeUser(u)
}

func (u *User) Update() error {
	if u.Token == "" {
		return errors.New("undefined token")
	}
	return storeUser(u)
}

func (u *User) UpdateCostUsage(usage *openai.Usage) error {
	if u.Cost == nil {
		u.Cost = &Cost{
			Usage: &CostUsage{PromptTokens: 0, CompletionTokens: 0, TotalTokens: 0},
			Cost:  0,
		}
	}
	if usage.PromptTokens > 0 {
		u.Cost.Usage.PromptTokens += usage.PromptTokens
	}
	if usage.CompletionTokens > 0 {
		u.Cost.Usage.CompletionTokens += usage.CompletionTokens
	}
	if usage.TotalTokens > 0 {
		u.Cost.Usage.TotalTokens += usage.TotalTokens
	}

	u.Cost.Cost = float64(u.Cost.Usage.TotalTokens) / 1000 * 0.002 // 每 1000 tokens $0.002
	return storeUser(u)
}

type UserStore struct {
	sync.RWMutex
	m map[string]*User
}

var users = &UserStore{
	m: make(map[string]*User),
}

func GetAllUser() []*User {
	users.Lock()
	defer users.Unlock()

	data := make([]*User, 0)
	for _, v := range users.m {
		data = append(data, v)
	}
	return data
}

func GetUser(token string) *User {
	if token == "" {
		return nil
	}
	users.Lock()
	defer users.Unlock()
	if _, ok := users.m[token]; !ok {
		return nil
	}
	return users.m[token]
}

func InitUsers() error {
	data, err := rd.SMembers(context.Background(), RedisUsersKey).Result()
	if err != nil {
		return err
	}

	users.Lock()
	defer users.Unlock()
	logrus.Infof("init access users:")
	for _, v := range data {
		if _, ok := users.m[v]; ok {
			continue
		}
		if val := rd.Get(context.Background(), fmt.Sprintf(RedisUserItemKey, v)).Val(); val != "" {
			user := &User{}
			if err := json.Unmarshal([]byte(val), user); err != nil {
				logrus.Error(err)
			}
			logrus.Infof("load user: %v", user)
			users.m[v] = user
		}
	}
	logrus.Infof("load all user: %d", len(users.m))
	return nil
}

func storeUser(u *User) error {
	users.Lock()
	defer users.Unlock()

	users.m[u.Token] = u
	b, err := json.Marshal(u)
	if err != nil {
		logrus.Error(err)
		return err
	}
	key := fmt.Sprintf(RedisUserItemKey, u.Token)
	if err := rd.Set(context.Background(), key, string(b), redis.KeepTTL).Err(); err != nil {
		logrus.Error(err)
		return err
	}
	return nil
}

func GenerateUserToken(n int) string {
	bytes := make([]byte, n/2)
	rand.New(rand.NewSource(time.Now().UnixNano())).Read(bytes)
	return fmt.Sprintf("%x", bytes)
}
