package apis

import (
	"github.com/gofiber/fiber/v2"
	"github.com/sashabaranov/go-openai"
)

func Hello(ctx *fiber.Ctx) error {
	return ctx.Status(fiber.StatusOK).Send([]byte("hello world"))
}

func Models(ctx *fiber.Ctx) error {
	models := []string{
		openai.GPT432K0314, openai.GPT432K, openai.GPT40314, openai.GPT4,
		openai.GPT3Dot5Turbo0301, openai.GPT3Dot5Turbo,
		openai.GPT3TextDavinci003, openai.GPT3TextDavinci002,
	}
	return ctx.JSON(models)
}
