package http

import (
	"context"
	"fmt"
	"log/slog"
	"mzhn/auth/internal/config"
	"mzhn/auth/internal/domain/entity"
	"mzhn/auth/internal/services/authservice"
	"mzhn/auth/internal/transport/http/handlers"
	"mzhn/auth/internal/transport/http/middleware"
	"mzhn/auth/pkg/sl"
	"strings"

	"github.com/labstack/echo/v4"
	emw "github.com/labstack/echo/v4/middleware"
)

type Server struct {
	*echo.Echo

	cfg    *config.Config
	logger *slog.Logger

	as *authservice.AuthService
}

func New(cfg *config.Config, as *authservice.AuthService) *Server {
	return &Server{
		Echo:   echo.New(),
		logger: slog.Default().With(sl.Module("http")),
		cfg:    cfg,
		as:     as,
	}
}

func (h *Server) setup() {

	h.Use(emw.Logger())
	h.Use(emw.CORSWithConfig(emw.CORSConfig{
		AllowOrigins:     strings.Split(h.cfg.Http.Cors.AllowedOrigins, ","),
		AllowMethods:     []string{echo.GET, echo.POST, echo.PUT, echo.PATCH, echo.DELETE},
		AllowCredentials: true,
	}))

	tokguard := middleware.Token()
	authguard := middleware.RequireAuth(h.as, h.cfg)

	h.POST("/register", handlers.Register(h.as))
	h.POST("/login", handlers.Login(h.as))
	h.POST("/refresh", handlers.Refresh(h.as), tokguard())
	h.POST("/authenticate", handlers.Authenticate(h.as), tokguard())
	h.GET("/profile", handlers.Profile(h.as), tokguard(), authguard())
	h.POST("/logout", handlers.Logout(h.as), tokguard(), authguard())
	h.POST("/roles/add", handlers.AddRoles(h.as), tokguard(), authguard(entity.RoleAdmin))
}

func (h *Server) Run(ctx context.Context) error {
	h.setup()

	host := h.cfg.Http.Host
	port := h.cfg.Http.Port
	addr := fmt.Sprintf("%s:%d", host, port)
	slog.Info("running http server", slog.String("addr", addr))

	go func() {
		if err := h.Start(addr); err != nil {
			return
		}
	}()

	<-ctx.Done()
	if err := h.Shutdown(ctx); err != nil {
		return err
	}

	slog.Info("shutting down http server\n")
	return nil
}
