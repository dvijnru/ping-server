package main

import (
	"fmt"
	"main/src/assets"
	"net/http"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/mcstatus-io/mcutil"
	"github.com/mcstatus-io/mcutil/options"
)

func init() {
	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
	}))

	if conf.Environment == "development" {
		app.Use(cors.New(cors.Config{
			AllowOrigins:  "*",
			AllowMethods:  "HEAD,OPTIONS,GET",
			ExposeHeaders: "X-Cache-Hit,X-Cache-Time-Remaining",
		}))

		app.Use(logger.New(logger.Config{
			Format:     "${time} ${ip}:${port} -> ${status}: ${method} ${path} (${latency})\n",
			TimeFormat: "2006/01/02 15:04:05",
		}))
	}

	app.Get("/ping", PingHandler)
	app.Get("/favicon.ico", FaviconHandler)
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

// FaviconHandler serves the favicon.ico file to any users that visit the API using a browser.
func FaviconHandler(ctx *fiber.Ctx) error {
	return ctx.Type("ico").Send(assets.Favicon)
}

// JavaStatusHandler returns the status of the Java edition Minecraft server specified in the address parameter.
func JavaStatusHandler(ctx *fiber.Ctx) error {
	host, port, err := ParseAddress(ctx.Params("address"), 25565)

	if err != nil {
		return ctx.Status(http.StatusBadRequest).SendString("Invalid address value")
	}

	if err = r.Increment(fmt.Sprintf("java-hits:%s-%d", host, port)); err != nil {
		return err
	}

	response, expiresAt, err := GetJavaStatus(host, port, ctx.QueryBool("query", true))

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
	host, port, err := ParseAddress(ctx.Params("address"), 19132)

	if err != nil {
		return ctx.Status(http.StatusBadRequest).SendString("Invalid address value")
	}

	if err = r.Increment(fmt.Sprintf("bedrock-hits:%s-%d", host, port)); err != nil {
		return err
	}

	response, expiresAt, err := GetBedrockStatus(host, port)

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
	host, port, err := ParseAddress(ctx.Params("address"), 25565)

	if err != nil {
		return ctx.Status(http.StatusBadRequest).SendString("Invalid address value")
	}

	icon, expiresAt, err := GetServerIcon(host, port)

	if err != nil {
		return err
	}

	ctx.Set("X-Cache-Hit", strconv.FormatBool(expiresAt != 0))

	if expiresAt != 0 {
		ctx.Set("X-Cache-Time-Remaining", strconv.Itoa(int(expiresAt.Seconds())))
	}

	return ctx.Type("png").Send(icon)
}

// SendVoteHandler allows sending of Votifier votes to the specified server.
func SendVoteHandler(ctx *fiber.Ctx) error {
	opts, err := ParseVoteOptions(ctx)

	if err != nil {
		return ctx.Status(http.StatusBadRequest).SendString(err.Error())
	}

	switch opts.Version {
	case 1:
		{
			if err = mcutil.SendLegacyVote(opts.Host, opts.Port, options.LegacyVote{
				PublicKey:   opts.PublicKey,
				ServiceName: opts.ServiceName,
				Username:    opts.Username,
				IPAddress:   opts.IPAddress,
				Timestamp:   opts.Timestamp,
				Timeout:     time.Second * 5,
			}); err != nil {
				return ctx.Status(http.StatusBadRequest).SendString(err.Error())
			}
		}
	case 2:
		{
			if err = mcutil.SendVote(opts.Host, opts.Port, options.Vote{
				ServiceName: opts.ServiceName,
				Username:    opts.Username,
				Token:       opts.Token,
				UUID:        opts.UUID,
				Timestamp:   opts.Timestamp,
				Timeout:     time.Second * 5,
			}); err != nil {
				return ctx.Status(http.StatusBadRequest).SendString(err.Error())
			}
		}
	}

	return ctx.Status(http.StatusOK).SendString("The vote was successfully sent to the server")
}

// DefaultIconHandler returns the default server icon.
func DefaultIconHandler(ctx *fiber.Ctx) error {
	return ctx.Type("png").Send(assets.DefaultIcon)
}
