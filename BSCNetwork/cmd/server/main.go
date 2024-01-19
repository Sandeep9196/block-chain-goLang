package main

import (
	"bsc_network/internal/config"
	"bsc_network/internal/config/db"
	"bsc_network/internal/config/log"

	"bsc_network/internal/cron"
	bnbGath "bsc_network/internal/geth"
	"bsc_network/internal/hdwallet"
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"

	"golang.org/x/crypto/pbkdf2"

	errs "bsc_network/internal/errors"
	// hcController "bsc_network/internal/healthcheck/controller/http"
	m "bsc_network/internal/middleware"
	v1BalanceController "bsc_network/internal/wallet/controller"
	v1WalletController "bsc_network/internal/wallet/controller/http/v1"
	walletRepo "bsc_network/internal/wallet/repository"
	walletService "bsc_network/internal/wallet/service"

	"github.com/go-playground/validator/v10"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

var (
	Version = "1.0.0"
	flagEnv = flag.String("env", "local", "environment")
)

const (
	gracefulTimeout   = 10 * time.Second
	readHeaderTimeout = 2 * time.Second
	mnemonicPhrase    = "MNEMONIC_PHRASE"
)

func main() {
	flag.Parse()
	ctx := context.Background()

	logger := log.NewWithZap(log.New(*flagEnv, log.ErrorLog)).With(ctx, "version", Version)

	cfg, err := config.Load(*flagEnv)
	if err != nil {
		fmt.Errorf("\n%v\n", err)
	}

	// fmt.Printf("\n\n%v\n\n", cfg)
	// connect to database
	db, err := db.Connect(ctx, cfg)
	if err != nil {
		fmt.Errorf("\n%v\n", err)
	}

	// connect to redis
	rds, err := config.RedisConnect(ctx, cfg)
	if err != nil {
		fmt.Errorf("\n%v\n", err)
	}

	phrase := ""
	phcfg, err := config.PhraseConfigLoad()
	if phrase == "" {

		if err != nil {
			fmt.Errorf("\n%v\n", err)
		}
		phrase = phcfg.Phrase
		// deletePhase()
	}

	// connect to blockchain node
	bnbGath, err := bnbGath.New(cfg)
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\n\n phrase is \n\n %s \n\n", phrase)

	hdw, err := hdwallet.NewFromMnemonic(phrase)
	if err != nil {
		fmt.Errorf("\n%v\n", err)
	}

	bnbCronLogger := log.NewWithZap(log.New(*flagEnv, log.BNBCronLog)).With(ctx, "version", Version)

	// run bnb blockCrawler cron
	bnbblockCrawler, err := cron.BNBBNBNewBlockCrawler(ctx, *flagEnv, bnbGath, hdw, cfg, phcfg, db, rds, bnbCronLogger)
	if err != nil {
		fmt.Errorf("\n%v\n", err)
	}
	bnbblockCrawler.Run(ctx)

	txProcessCronLogger := log.NewWithZap(log.New(*flagEnv, log.TxProcessLog)).With(ctx, "version", Version)

	// run tx process cron
	txProcessCron, err := cron.NewTxProcessor(ctx, db, txProcessCronLogger)
	if err != nil {
		logger.Fatal(err)
	}
	txProcessCron.Run(ctx)

	// run tx reconciliation cron
	bnbReconcile, err := cron.NewBNBReconcile(ctx, bnbGath, hdw, cfg, phcfg, db, bnbCronLogger)
	if err != nil {
		logger.Fatal(err)
	}
	bnbReconcile.Run(ctx)

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.App.Port),
		Handler:           buildHandler(hdw, bnbGath, logger, rds, db, &cfg),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	fmt.Printf("Server listening on %s", server.Addr)

	go func() {
		if err = server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Errorf("\n%v\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Printf("\nServer shutting down\n")

	ctx, cancel := context.WithTimeout(ctx, gracefulTimeout)
	defer cancel()

	if err = server.Shutdown(ctx); err != nil {
		fmt.Errorf("\n%v\n", err)
	}

	fmt.Printf("\nServer exiting\n")
}

func deletePhase() {
	filePath := "./config/phrase.yml" // Update this path if needed

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		fmt.Println("The file does not exist!")
		return
	}
	err := os.Remove(filePath)
	if err != nil {
		fmt.Printf("Error deleting file: %v\n", err)
		return
	}
}

func decrypt(encrypted string, secretKey string, salt string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}

	key := pbkdf2.Key([]byte(secretKey), []byte(salt), 65536, 32, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	if len(data) < aes.BlockSize {
		return "", fmt.Errorf("cipher text too short")
	}

	iv := make([]byte, aes.BlockSize) // all zeros IV
	cipherText := data

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(cipherText, cipherText)

	// Remove PKCS5 padding
	padding := cipherText[len(cipherText)-1]
	cipherText = cipherText[:len(cipherText)-int(padding)]

	return string(cipherText), nil
}

// buildMiddleware sets up the middlewre logic and builds a handler.
func buildMiddleware() []echo.MiddlewareFunc {
	var middlewares []echo.MiddlewareFunc
	logger := log.NewWithZap(log.New(*flagEnv, log.AccessLog)).With(context.TODO(), "version", Version)

	middlewares = append(middlewares,

		// Echo built-in middleware
		middleware.Recover(),

		middleware.Secure(),

		middleware.RequestIDWithConfig(middleware.RequestIDConfig{
			Generator: func() string {
				return uuid.New().String()
			},
		}),

		// Api access log
		m.AccessLogHandler(logger),
	)

	return middlewares
}

func tokenValidationMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		fmt.Println("tokenValidationMiddleware")
		token := c.Request().Header.Get("X-Secret")

		if token == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "X-Secret is required"})
		}

		decToken, err := decrypt(token, config.GlobalConfig.Security.Key, config.GlobalConfig.Security.Salt)

		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid X-Secret Provided"})
		}

		if decToken != config.GlobalConfig.Security.Token {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid X-Secret Provided"})
		}

		return next(c)
	}
}

// buildHandler sets up the HTTP routing and builds an HTTP handler.
func buildHandler(
	hdw *hdwallet.Wallet,
	geth bnbGath.Geth,
	logger log.Logger,
	rds redis.Client,
	db *sqlx.DB,
	cfg *config.Config,
) *echo.Echo {
	t := time.Duration(cfg.Context.Timeout) * time.Second

	e := echo.New()
	e.HTTPErrorHandler = m.NewHTTPErrorHandler(errs.GetStatusCodeMap()).Handler(logger)
	e.Validator = &m.CustomValidator{Validator: validator.New()}
	e.Use(buildMiddleware()...)
	// checking the token validation
	// e.Use(tokenValidationMiddleware)

	dg := e.Group("")

	v1WalletController.RegisterHandlers(
		dg.Group("/v1/bnb-crypto"),
		walletService.New(hdw, geth, db, walletRepo.New(db, logger), logger, t),
		logger,
	)

	v1BalanceController.RegisterHandlers(
		dg.Group("/v1/bnb-crypto"),
		geth,
		hdw,
		walletRepo.New(db, logger),
	)

	return e
}
