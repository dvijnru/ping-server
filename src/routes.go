package main

import (
	"context"
	"fmt"
	"main/src/assets"
	"net/http"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/favicon"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/mcstatus-io/mcutil/v4/options"
	"github.com/mcstatus-io/mcutil/v4/util"
	"github.com/mcstatus-io/mcutil/v4/vote"
)

func init() {
	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
	}))

	app.Use(favicon.New(favicon.Config{
		Data: assets.Favicon,
	}))

	if config.Environment == "development" {
		app.Use(cors.New(cors.Config{
			AllowOrigins:  "*",
			AllowMethods:  "HEAD,OPTIONS,GET,POST",
			ExposeHeaders: "X-Cache-Hit,X-Cache-Time-Remaining",
		}))

		app.Use(logger.New(logger.Config{
			Format:     "${time} ${ip}:${port} -> ${status}: ${method} ${path} (${latency})\n",
			TimeFormat: "2006/01/02 15:04:05",
		}))
	}

	app.Get("/ping", PingHandler)
	app.Get("/status/java/:address", JavaStatusHandler)
	app.Get("/status/bedrock/:address", BedrockStatusHandler)
	app.Get("/icon", DefaultIconHandler)
	app.Get("/icon/:address", IconHandler)
	app.Post("/vote", SendVoteHandler)
}

// PingHandler responds with a 200 OK status for simple health checks.
func PingHandler(ctx *fiber.Ctx) error {
	return ctx.SendStatus(http.StatusOK)
}

// JavaStatusHandler returns the status of the Java edition Minecraft server specified in the address parameter.
func JavaStatusHandler(ctx *fiber.Ctx) error {
	opts, err := GetStatusOptions(ctx)

	if err != nil {
		return err
	}

	hostname, port, err := ParseAddress(strings.ToLower(ctx.Params("address")), util.DefaultJavaPort)

	if err != nil {
		return ctx.Status(http.StatusBadRequest).SendString("Invalid address value")
	}

	authorized, err := Authenticate(ctx)

	// This check should work for both scenarios, because nil should be returned if the user
	// is unauthorized, and err will be nil in that case.
	if err != nil || !authorized {
		return err
	}

	if err = r.Increment(fmt.Sprintf("java-hits:%s", fmt.Sprintf("%s:%d", hostname, port))); err != nil {
		return err
	}

	response, expiresAt, err := GetJavaStatus(hostname, port, opts)

	if err != nil {
		return err
	}

	ctx.Set("X-Cache-Hit", strconv.FormatBool(expiresAt != 0))

	if expiresAt != 0 {
		ctx.Set("X-Cache-Time-Remaining", strconv.Itoa(int(expiresAt.Seconds())))
	}

	return ctx.JSON(response)
}

// BedrockStatusHandler returns the status of the Bedrock edition Minecraft server specified in the address parameter.
func BedrockStatusHandler(ctx *fiber.Ctx) error {
	opts, err := GetStatusOptions(ctx)

	if err != nil {
		return err
	}

	hostname, port, err := ParseAddress(strings.ToLower(ctx.Params("address")), util.DefaultBedrockPort)

	if err != nil {
		return ctx.Status(http.StatusBadRequest).SendString("Invalid address value")
	}

	if err = r.Increment(fmt.Sprintf("bedrock-hits:%s", fmt.Sprintf("%s:%d", hostname, port))); err != nil {
		return err
	}

	response, expiresAt, err := GetBedrockStatus(hostname, port, opts)

	if err != nil {
		return err
	}

	ctx.Set("X-Cache-Hit", strconv.FormatBool(expiresAt != 0))

	if expiresAt != 0 {
		ctx.Set("X-Cache-Time-Remaining", strconv.Itoa(int(expiresAt.Seconds())))
	}

	return ctx.JSON(response)
}

// IconHandler returns the server icon for the specified Java edition Minecraft server.
func IconHandler(ctx *fiber.Ctx) error {
	opts, err := GetStatusOptions(ctx)

	if err != nil {
		return err
	}

	hostname, port, err := ParseAddress(strings.ToLower(ctx.Params("address")), util.DefaultJavaPort)

	if err != nil {
		return ctx.Status(http.StatusBadRequest).SendString("Invalid address value")
	}

	icon, expiresAt, err := GetServerIcon(hostname, port, opts)

	if err != nil {
		return err
	}

	ctx.Set("X-Cache-Hit", strconv.FormatBool(expiresAt != 0))

	if expiresAt != 0 {
		ctx.Set("X-Cache-Time-Remaining", strconv.Itoa(int(expiresAt.Seconds())))
	}

	return ctx.Type("png").Send(icon)
}

// DefaultIconHandler returns the default server icon.
func DefaultIconHandler(ctx *fiber.Ctx) error {
	return ctx.Type("png").Send(assets.DefaultIcon)
}

// SendVoteHandler allows sending of Votifier votes to the specified server.
func SendVoteHandler(ctx *fiber.Ctx) error {
	opts, err := GetVoteOptions(ctx)

	if err != nil {
		return ctx.Status(http.StatusBadRequest).SendString(err.Error())
	}

	c, cancel := context.WithTimeout(context.Background(), opts.Timeout)

	defer cancel()

	if err = vote.SendVote(c, opts.Host, opts.Port, options.Vote{
		PublicKey:   opts.PublicKey,
		Token:       opts.Token,
		ServiceName: opts.ServiceName,
		Username:    opts.Username,
		IPAddress:   opts.IPAddress,
		Timestamp:   opts.Timestamp,
		Timeout:     opts.Timeout,
	}); err != nil {
		return ctx.Status(http.StatusBadRequest).SendString(err.Error())
	}

	return ctx.Status(http.StatusOK).SendString("The vote was successfully sent to the server")
}
