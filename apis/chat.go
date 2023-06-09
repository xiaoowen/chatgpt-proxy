package apis

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
	"github.com/xiaoowen/chatgpt-proxy/data"
)

func Chat(ctx *fiber.Ctx) error {
	instance, err := getChatCompletionInstance(ctx, false)
	if err != nil {
		logrus.Error(err)
		return ctx.Status(fiber.StatusBadRequest).Send([]byte(err.Error()))
	}

	instance.payload.Stream = false

	resp, err := instance.client.CreateChatCompletion(ctx.Context(), instance.payload)
	if err != nil {
		logrus.Error(err)
		return ctx.SendStatus(fiber.StatusInternalServerError)
	}

	if err := instance.user.UpdateCostUsage(&resp.Usage); err != nil {
		logrus.Error(err)
		return ctx.SendStatus(fiber.StatusInternalServerError)
	}

	ret := &chatResponse{
		Done: true, Created: resp.Created,
		Content:     resp.Choices[0].Message.Content,
		TotalTokens: resp.Usage.TotalTokens,
		UserCost:    instance.user.Cost,
	}
	return ctx.JSON(ret)
}

func StreamChat(ctx *fiber.Ctx) error {
	instance, err := getChatCompletionInstance(ctx, true)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).Send([]byte(err.Error()))
	}
	instance.payload.Stream = true

	stream, err := instance.client.CreateChatCompletionStream(ctx.Context(), instance.payload)
	if err != nil {
		logrus.Error(err)
		return ctx.SendStatus(fiber.StatusInternalServerError)
	}
	defer stream.Close()

	for k, v := range sseHeaders {
		ctx.Set(k, v)
	}

	usage := &openai.Usage{
		PromptTokens: data.GetTokensFromMessages(instance.payload.Messages, instance.payload.Model),
	}

	if _, err := ctx.WriteString("retry: 10000\n"); err != nil {
		return err
	}
	if _, err := ctx.WriteString("event: chat-completion-stream\n"); err != nil {
		return err
	}

	responseText := ""

	flush := func(ret *chatResponse) {
		if retBytes, err := json.Marshal(ret); err != nil {
			logrus.Error(err)
		} else {
			if _, err := ctx.WriteString(fmt.Sprintf("data: %s\n\n", string(retBytes))); err != nil {
				logrus.Error(err)
			}
		}
	}

	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			usage.CompletionTokens = data.GetTokensByModel(responseText, instance.payload.Model)
			usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
			if err := instance.user.UpdateCostUsage(usage); err != nil {
				logrus.Infof("update user cost fialed. err: %s, usage: %#v", err.Error(), usage)
			}
			ret := &chatResponse{
				Done: true, Content: "", TotalTokens: usage.TotalTokens, UserCost: instance.user.Cost,
			}
			flush(ret)
			return nil
		} else if err != nil {
			logrus.Errorf("stream recv error: %s", err.Error())
			return err
		}
		ret := &chatResponse{Content: resp.Choices[0].Delta.Content}
		responseText += ret.Content
		flush(ret)
		if ctx.Response().ConnectionClose() {
			return nil
		}
	}
}

func getChatCompletionInstance(ctx *fiber.Ctx, usingStream bool) (instance *chatInstance, err error) {
	instance = &chatInstance{}
	instance.payload = openai.ChatCompletionRequest{}
	var reader io.Reader
	if usingStream {
		reader = strings.NewReader(ctx.Query("data", ""))
	} else {
		reader = bytes.NewReader(ctx.Body())
	}
	if reader == nil {
		return nil, errors.New("io.reader undefined")
	}
	if err := json.NewDecoder(reader).Decode(&instance.payload); err != nil {
		return nil, err
	}

	messageLen := len(instance.payload.Messages)
	if messageLen == 0 {
		return nil, errors.New("content is empty")
	}

	instance.user, err = getRequestUserInfo(ctx)
	if err != nil || instance.user == nil {
		return nil, err
	}
	logrus.Infof("received content: %#v", instance.payload.Messages)

	gptSetting := data.GetSettingWithUser(instance.user.Setting)
	if gptSetting.MaxHistoryMessages == 0 {
		gptSetting.MaxHistoryMessages = 1
	}
	if gptSetting.MaxHistoryMessages > messageLen {
		gptSetting.MaxHistoryMessages = messageLen
	}
	instance.payload.Messages = instance.payload.Messages[messageLen-gptSetting.MaxHistoryMessages:]
	instance.payload.Model = gptSetting.Model
	instance.client = openai.NewClient(gptSetting.APIKey)
	return instance, nil
}
