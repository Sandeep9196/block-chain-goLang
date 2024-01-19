package v1

import (
	"fmt"
	"net/http"

	"bsc_network/internal/config/log"
	"bsc_network/internal/randx"
	"bsc_network/internal/tools"
	"bsc_network/internal/wallet/service"

	"github.com/labstack/echo/v4"
)

type resource struct {
	logger  log.Logger
	service service.Service
}

func RegisterHandlers(g *echo.Group, service service.Service, logger log.Logger) {
	r := &resource{logger, service}

	wallet := g.Group("/wallet")
	{
		wallet.GET("/get_unique_id", r.GetUniqueId)
		wallet.GET("/list/member/:member_id", r.ListWallet)
		wallet.GET("/bnb/member/:member_id", r.GetBNBWallet)
		wallet.POST("/transfer", r.TransferCrypto)
	}
}

func (r resource) GetBNBWallet(c echo.Context) error {
	fmt.Printf("GetBNBWallet    is ")
	var req service.GetWalletRequest
	if err := tools.BindValidate(c, &req); err != nil {
		return err
	}

	address, err := r.service.GetBNBWallet(c.Request().Context(), req)
	if err != nil {
		return err
	}

	fmt.Printf("address    is " + address)

	return tools.JSON(
		c,
		http.StatusOK,
		tools.Success,
		struct {
			Address string `json:"address"`
		}{
			Address: address,
		},
	)
}

func (r resource) ListWallet(c echo.Context) error {
	var req service.GetWalletRequest
	if err := tools.BindValidate(c, &req); err != nil {
		return err
	}

	wallets, err := r.service.ListWallet(c.Request().Context(), req)
	if err != nil {
		return err
	}

	return tools.JSON(
		c,
		http.StatusOK,
		tools.Success,
		struct {
			BNBAddress string `json:"bnb_address"`
		}{
			BNBAddress: wallets[0],
		},
	)
}

func (r resource) TransferCrypto(c echo.Context) error {
	var req service.TransferCryptoRequest
	if err := tools.BindValidate(c, &req); err != nil {
		return err
	}

	if err := r.service.TransferCrypto(c.Request().Context(), req); err != nil {
		return err
	}

	return tools.JSON(c, http.StatusOK, tools.Success, nil)
}

func (r resource) GetUniqueId(c echo.Context) error {
	return tools.JSON(c, http.StatusOK, tools.Success, randx.GenUniqueId())
}
