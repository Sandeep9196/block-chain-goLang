package tools

import (
	"math/big"

	"github.com/shopspring/decimal"
)

const (
	//nolint:gosec // false positive
	USDTBNBToken string = "USDT-BNB"
	BNBCoin      string = "BNB"
)

func CalculateGasCost(gasLimit, gasPrice *big.Float) *big.Float {
	gasCost := new(big.Float)
	return gasCost.Mul(gasLimit, gasPrice)
}

//nolint:gomnd // decimal is base 10
func ToDecimal(ivalue interface{}, decimals int) decimal.Decimal {
	value := new(big.Int)
	switch v := ivalue.(type) {
	case string:
		value.SetString(v, 10)
	case *big.Int:
		value = v
	}

	mul := decimal.NewFromFloat(float64(10)).Pow(decimal.NewFromFloat(float64(decimals)))
	num, _ := decimal.NewFromString(value.String())
	result := num.Div(mul)

	return result
}

// ToWei decimals to wei.
//
//nolint:gomnd // decimal is base 10
func ToWei(iamount interface{}, decimals int) *big.Int {
	amount := decimal.NewFromFloat(0)
	switch v := iamount.(type) {
	case string:
		amount, _ = decimal.NewFromString(v)
	case float64:
		amount = decimal.NewFromFloat(v)
	case int64:
		amount = decimal.NewFromFloat(float64(v))
	case decimal.Decimal:
		amount = v
	case *decimal.Decimal:
		amount = *v
	}

	mul := decimal.NewFromFloat(float64(10)).Pow(decimal.NewFromFloat(float64(decimals)))
	result := amount.Mul(mul)

	wei := new(big.Int)
	wei.SetString(result.String(), 10)

	return wei
}
