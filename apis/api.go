package apis

import (
	"errors"
	"github.com/gofiber/fiber/v2"
	"github.com/sashabaranov/go-openai"
	"github.com/xiaoowen/chatgpt-proxy/data"
	"time"
)

type chatInstance struct {
	user    *data.User
	client  *openai.Client
	payload openai.ChatCompletionRequest
}

type chatResponse struct {
	Done        bool       `json:"done,omitempty"`
	Created     int64      `json:"created,omitempty"`
	Content     string     `json:"content"`
	TotalTokens int        `json:"total_tokens,omitempty"`
	UserCost    *data.Cost `json:"user_cost,omitempty"`
}

var sseHeaders = map[string]string{
	"Content-Type":      "text/event-stream",
	"Cache-Control":     "no-cache",
	"Connection":        "keep-alive",
	"Transfer-Encoding": "chunked",
}

func getRequestUserInfo(ctx *fiber.Ctx) (user *data.User, err error) {
	token := ctx.Get("Access-Token", "")
	if token == "" {
		return nil, errors.New("invalid user token")
	}
	user = data.GetUser(token)
	if user == nil {
		return nil, errors.New("invalid user token, user info not found")
	}
	if user.ExpireTimestamp > 0 && user.ExpireTimestamp < time.Now().Unix() {
		return nil, errors.New("license expires")
	}
	return user, nil
}
