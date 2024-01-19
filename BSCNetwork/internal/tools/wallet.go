package tools

import "fmt"

type (
	CoinType   string
	ChangeType string
)

const (
	HDWalletRootMnemonic         string     = "ROOT_MNEMONIC"
	HDWalletDerivationPathPrefix string     = "m/44'"
	HDWalletETH                  CoinType   = "/60'"
	HDWalletTRX                  CoinType   = "/195'"
	HDWalletBNB                  CoinType   = "/714'"
	HDWalletExternal             ChangeType = "/0"
	HDWalletDefaultIndexPath     string     = "/0"
	HDWalletInternal             ChangeType = "/1"
	HDWalletDefaultIndex         int64      = 0
)

func GetDerivationPath(index int64) string {
	return fmt.Sprintf(`%s%s/%d'%s%s`,
		HDWalletDerivationPathPrefix,
		string(HDWalletETH),
		index,
		HDWalletExternal,
		HDWalletDefaultIndexPath,
	)
}

func GetTRXDerivationPath(index int64) string {
	return fmt.Sprintf(`%s%s/%d'%s%s`,
		HDWalletDerivationPathPrefix,
		string(HDWalletTRX),
		index,
		HDWalletExternal,
		HDWalletDefaultIndexPath,
	)
}
