package apis

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
	"github.com/xiaoowen/chatgpt-proxy/data"
	"net/http"
	"time"
)

func NewUser(ctx *fiber.Ctx) error {
	user, err := getRequestUserInfo(ctx)
	if err != nil || user == nil {
		return ctx.Status(fiber.StatusBadRequest).Send([]byte(err.Error()))
	}
	if !user.Admin {
		return ctx.SendStatus(fiber.StatusForbidden)
	}

	newUser := &data.User{}
	if err := json.NewDecoder(ctx.Request().BodyStream()).Decode(newUser); err != nil {
		return ctx.SendStatus(fiber.StatusBadRequest)
	}
	if newUser.ExpireTimestamp == 0 {
		newUser.ExpireTimestamp = time.Now().Add(time.Hour * 24 * 7).Unix()
	}

	newUser.Token = data.GenerateUserToken(32)
	if err := newUser.New(); err != nil {
		logrus.Error(err)
		return ctx.SendStatus(fiber.StatusInternalServerError)
	}
	return ctx.JSON(newUser)
}

func EditUser(ctx *fiber.Ctx) error {
	user, err := getRequestUserInfo(ctx)
	if err != nil || user == nil {
		return ctx.Status(fiber.StatusBadRequest).Send([]byte(err.Error()))
	}
	if !user.Admin {
		return ctx.SendStatus(fiber.StatusForbidden)
	}

	editUser := &data.User{}
	if err := json.NewDecoder(ctx.Request().BodyStream()).Decode(editUser); err != nil {
		return ctx.SendStatus(fiber.StatusBadRequest)
	}
	if editUser.Token != user.Token {
		return ctx.SendStatus(http.StatusForbidden)
	}
	if editUser.Admin == !user.Admin {
		user.Admin = editUser.Admin
	}
	if editUser.ExpireTimestamp != user.ExpireTimestamp && editUser.ExpireTimestamp > 0 {
		user.ExpireTimestamp = editUser.ExpireTimestamp
	}
	if editUser.Nickname != user.Nickname {
		user.Nickname = editUser.Nickname
	}
	if editUser.Avatar != editUser.Avatar {
		user.Avatar = editUser.Avatar
	}
	if editUser.Setting != nil {
		user.Setting = editUser.Setting
	}

	if err := user.Update(); err != nil {
		logrus.Error(err)
		return ctx.SendStatus(http.StatusInternalServerError)
	}

	return ctx.JSON(user)
}

func Users(ctx *fiber.Ctx) error {
	user, err := getRequestUserInfo(ctx)
	if err != nil || user == nil {
		return ctx.Status(fiber.StatusBadRequest).Send([]byte(err.Error()))
	}
	if !user.Admin {
		return ctx.SendStatus(fiber.StatusForbidden)
	}
	return ctx.JSON(data.GetAllUser())
}

func Profile(ctx *fiber.Ctx) error {
	user, err := getRequestUserInfo(ctx)
	if err != nil || user == nil {
		return ctx.Status(fiber.StatusBadRequest).Send([]byte(err.Error()))
	}
	return ctx.JSON(user)
}
