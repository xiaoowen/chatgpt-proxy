package apis

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/xiaoowen/chatgpt-proxy/data"
)

func Setting(ctx *fiber.Ctx) error {
	user, err := getRequestUserInfo(ctx)
	if err != nil || user == nil {
		return ctx.Status(fiber.StatusBadRequest).Send([]byte(err.Error()))
	}
	if !user.Admin {
		return ctx.SendStatus(fiber.StatusForbidden)
	}
	return ctx.JSON(data.GetGPTSetting())
}

func EditSetting(ctx *fiber.Ctx) error {
	user, err := getRequestUserInfo(ctx)
	if err != nil || user == nil {
		return ctx.Status(fiber.StatusBadRequest).Send([]byte(err.Error()))
	}
	if !user.Admin {
		return ctx.SendStatus(fiber.StatusForbidden)
	}
	setting := &data.GPTSetting{}
	if err := json.NewDecoder(ctx.Request().BodyStream()).Decode(setting); err != nil {
		return ctx.SendStatus(fiber.StatusBadRequest)
	}
	setting.Update()
	return ctx.SendStatus(fiber.StatusOK)
}
