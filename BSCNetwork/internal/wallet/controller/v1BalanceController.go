// v1BalanceController.go

package controllers

import (
	"bsc_network/internal/geth"
	"bsc_network/internal/tools"
	"bsc_network/internal/wallet/repository"
	"net/http"

	hdw "bsc_network/internal/hdwallet"

	"github.com/labstack/echo/v4"
	// ... other imports ...
)

func RegisterHandlers(g *echo.Group, geth geth.Geth, hdwallet *hdw.Wallet, repo repository.Repository) {
	g.GET("/balances", getBalance(geth, hdwallet, repo))
}

func getBalance(geth geth.Geth, hdwallet *hdw.Wallet, repo repository.Repository) echo.HandlerFunc {
	return func(c echo.Context) error {
		derivationPath := tools.GetDerivationPath(tools.HDWalletDefaultIndex)
		rootAccount, err := hdwallet.Derive(hdw.MustParseDerivationPath(derivationPath), false)
		if err != nil {
			c.Logger().Error(err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "rootAccount internal error"})
		}

		rootWallet := rootAccount.Address.Hex()

		BNBAvailableBalance, err := geth.GetBNB(c.Request().Context(), rootWallet)
		if err != nil {
			c.Logger().Error(err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "BNBAvailableBalance Available balance error"})
		}

		USDTAvailableBalance, err := geth.GetUSDT(rootWallet)
		if err != nil {
			c.Logger().Error(err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "USDTAvailableBalance Available balance error"})
		}

		return tools.JSON(c, http.StatusOK, tools.Success, map[string]interface{}{
			"BNB": map[string]string{
				"BNB":      BNBAvailableBalance.String(),
				"USDT-BNB": USDTAvailableBalance.String(),
				"address":  rootWallet,
			},
		})
	}
}
