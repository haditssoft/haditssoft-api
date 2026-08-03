package main

import (
	"os"

	"github.com/haditssoft/haditssoft-backend/internal/auth"
	"github.com/haditssoft/haditssoft-backend/internal/bookmark"
	"github.com/haditssoft/haditssoft-backend/internal/captcha"
	"github.com/haditssoft/haditssoft-backend/internal/font"
	"github.com/haditssoft/haditssoft-backend/internal/hadithadmin"
	"github.com/haditssoft/haditssoft-backend/internal/hadithdata"
	"github.com/haditssoft/haditssoft-backend/internal/lastread"
	"github.com/haditssoft/haditssoft-backend/internal/note"
	"github.com/haditssoft/haditssoft-backend/internal/opencode"
	"github.com/haditssoft/haditssoft-backend/internal/search"
	"github.com/haditssoft/haditssoft-backend/internal/searchmode"
	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
	"github.com/haditssoft/haditssoft-backend/internal/shared/env"
	"github.com/haditssoft/haditssoft-backend/internal/shared/middleware"
	"github.com/haditssoft/haditssoft-backend/internal/shared/utils"
	"github.com/haditssoft/haditssoft-backend/internal/shared/validator"
	"github.com/haditssoft/haditssoft-backend/internal/theme"
	"github.com/haditssoft/haditssoft-backend/internal/user"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func init() {
	env.LoadEnvVariables()
	database.Init()
	database.RunMigrations()
	validator.RegisterCustomValidations()
}

func main() {
	utils.InitQueryLogger(".")
	app := fiber.New(fiber.Config{
		Prefork: true,
	})
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		ExposeHeaders: "X-Total-Count",
		AllowOrigins:  "*",
		AllowHeaders:  "*",
	}))
	app.Use(compress.New())
	app.Static("/", "./storage")

	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	v1 := app

	hadithDataHandler := hadithdata.NewHandler()
	hadithdata.RegisterRoutes(v1, hadithDataHandler)

	search.RegisterRoutes(v1)

	captcha.RegisterRoutes(v1)
	opencode.RegisterRoutes(v1)

	authRepo := auth.NewRepository()
	authSvc := auth.NewService(authRepo)
	authHandler := auth.NewHandler(authSvc)
	auth.RegisterRoutes(v1, authHandler, middleware.Protected())

	// ADMIN
	v2 := app.Group("/admin")
	authAdminHandler := auth.NewAdminHandler(authSvc)
	auth.RegisterAdminRoutes(v2, authAdminHandler, middleware.Protected(), middleware.IsAdmin)

	userRepo := user.NewRepository()
	userSvc := user.NewService(userRepo)
	userHandler := user.NewHandler(userSvc)
	user.RegisterRoutes(v1, userHandler, middleware.TokenOnly(), middleware.Protected())
	userAdminHandler := user.NewAdminHandler(userSvc)
	user.RegisterAdminRoutes(v2, userAdminHandler, middleware.Protected(), middleware.IsAdmin)

	bookmarkRepo := bookmark.NewRepository()
	bookmarkSvc := bookmark.NewService(bookmarkRepo)
	bookmarkHandler := bookmark.NewHandler(bookmarkSvc)
	bookmark.RegisterRoutes(v1, bookmarkHandler, middleware.Protected())

	noteRepo := note.NewRepository()
	noteSvc := note.NewService(noteRepo)
	noteHandler := note.NewHandler(noteSvc)
	note.RegisterRoutes(v1, noteHandler)

	fontRepo := font.NewRepository()
	fontSvc := font.NewService(fontRepo)
	fontHandler := font.NewHandler(fontSvc)
	font.RegisterRoutes(v1, fontHandler)

	themeRepo := theme.NewRepository()
	themeSvc := theme.NewService(themeRepo)
	themeHandler := theme.NewHandler(themeSvc)
	theme.RegisterRoutes(v1, themeHandler)

	smRepo := searchmode.NewRepository()
	smSvc := searchmode.NewService(smRepo)
	smHandler := searchmode.NewHandler(smSvc)
	searchmode.RegisterRoutes(v1, smHandler)

	lrRepo := lastread.NewRepository()
	lrSvc := lastread.NewService(lrRepo)
	lrHandler := lastread.NewHandler(lrSvc)
	lastread.RegisterRoutes(v1, lrHandler)

	hadithadmin.RegisterRoutes(v2)

	app.Listen(":" + os.Getenv("APP_PORT"))
}
