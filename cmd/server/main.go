package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/ivm97/irbis-api/internal/adapter/authfile"
	"github.com/ivm97/irbis-api/internal/adapter/httpapi"
	"github.com/ivm97/irbis-api/internal/adapter/irbisgw"
	"github.com/ivm97/irbis-api/internal/config"
	"github.com/ivm97/irbis-api/internal/domain"
	"github.com/ivm97/irbis-api/internal/ports"
	"github.com/ivm97/irbis-api/internal/usecase"
)

func main() {
	cfgPath := flag.String("config", "configs/config.example.yaml", "path to config YAML")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	auth, err := authfile.New(cfg.Auth.UsersFile, cfg.Auth.AllowPlaintext)
	if err != nil {
		log.Fatalf("users: %v", err)
	}

	gw := irbisgw.New(irbisgw.Options{
		Host:          cfg.Irbis.Host,
		Port:          cfg.Irbis.Port,
		DatabasesMenu: cfg.Irbis.DatabasesMenu,
		DialTimeout:   cfg.Irbis.DialTimeout,
		IOTimeout:     cfg.Irbis.IOTimeout,
	})

	var irbisSess ports.Session
	if !cfg.Irbis.UseRequestCredentials {
		irbisSess = ports.Session{
			Username: cfg.Irbis.ServiceUsername,
			Password: cfg.Irbis.ServicePassword,
		}
	}

	defaultARM, err := domain.NormalizeWorkstation(cfg.Irbis.Workstation, domain.ARMCataloger)
	if err != nil {
		log.Fatalf("workstation: %v", err)
	}

	svc := usecase.NewCatalogService(auth, gw, irbisSess, defaultARM)

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())
	e.Use(middleware.Logger())
	e.Use(middleware.RequestID())

	httpapi.NewHandler(svc).Register(e)

	addr := cfg.HTTP.Addr
	fmt.Fprintf(os.Stderr, "irbis-api listening on %s (ARM default %s)\n", addr, defaultARM)
	if err := e.Start(addr); err != nil {
		log.Fatal(err)
	}
}
