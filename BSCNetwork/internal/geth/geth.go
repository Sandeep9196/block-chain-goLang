package geth

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math"
	"math/big"

	"bsc_network/internal/config"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"golang.org/x/crypto/sha3"
)

type Geth interface {
	GetBNB(ctx context.Context, address string) (*big.Float, error)
	GetUSDT(address string) (*big.Float, error)
	SuggestGasPrice(ctx context.Context) (*big.Int, error)
	EstimateGasLimit(ctx context.Context, address string, data []byte) (uint64, error)
	GetTransaction(ctx context.Context, hash common.Hash) (*types.Transaction, bool, error)
	GetTransactionInfo(ctx context.Context, hash common.Hash) (*types.Receipt, *types.Transaction, bool, error)
	GetTransactionReceipt(ctx context.Context, hash common.Hash) (*types.Receipt, error)
	GetLatestBlock(ctx context.Context) (*big.Int, error)
	GetTransactionByBlockNum(ctx context.Context, blockNum *big.Int) (types.Transactions, error)
	GetChainID(ctx context.Context) (*big.Int, error)	
	TransferBNB(
		ctx context.Context,
		senderPrivateKey []byte,
		receiverPublicKey string,
		amount *big.Int,
		gasPrice *big.Int,
	) (string, error)

	TransferUSDT(ctx context.Context,
		senderPrivateKey []byte,
		receiverPublicKey string,
		amount *big.Int,
		gasPrice *big.Int,
		tokenType string,
	) (string, error)
	BNBDecimals() int
	BNBUSDTDecimals() int
	LeftPadBytesLength() int
	Bnb20GasLimit() uint64
	BNBGasLimit() uint64
	TransactionNotFoundMsg() string
}

type geth struct {
	client *ethclient.Client
	cfg    config.Config
}

const (
	bnbDecimals         = 18
	bnbUsdtDecimals     = 18
	leftPadBytesLength  = 32
	bnb20GasLimit       = 100000
	bnbGasLimit         = 21000
	transactionNotFound = "not found"
)

var (
	ErrFailToParse = errors.New("fail to parse big float from string")
	ErrCastToECDSA = errors.New("fail to cast ECDSA")
)

func New(cfg config.Config) (Geth, error) {
	client, err := ethclient.Dial("https://data-seed-prebsc-1-s1.binance.org:8545")
	if err != nil {
		return geth{}, err
	}

	return geth{client, cfg}, nil
}
func (g geth) GetBNB(ctx context.Context, address string) (*big.Float, error) {
	account := common.HexToAddress(address)

	weiBalance, err := g.client.BalanceAt(ctx, account, nil)
	if err != nil {
		return &big.Float{}, err
	}

	weiBalanceBigFloat, success := new(big.Float).SetString(weiBalance.String())
	if !success {
		return &big.Float{}, ErrFailToParse
	}

	balance := new(big.Float).Quo(weiBalanceBigFloat, big.NewFloat(math.Pow10(bnbDecimals)))

	return balance, nil
}

func (g geth) GetUSDT(address string) (*big.Float, error) {
	instance, err := NewEthapi(common.HexToAddress(g.cfg.Blockchain.BNBUSDTContract), g.client)
	if err != nil {
		return &big.Float{}, err
	}

	weiBalance, err := instance.BalanceOf(&bind.CallOpts{}, common.HexToAddress(address))
	if err != nil {
		return &big.Float{}, err
	}

	decimals, err := instance.Decimals(&bind.CallOpts{})
	if err != nil {
		return &big.Float{}, err
	}

	balance, success := new(big.Float).SetString(weiBalance.String())
	if !success {
		return &big.Float{}, ErrFailToParse
	}

	balance = new(big.Float).Quo(balance, big.NewFloat(math.Pow10((int(decimals)))))

	return balance, nil
}

func (g geth) SuggestGasPrice(ctx context.Context) (*big.Int, error) {
	gasPrice, err := g.client.SuggestGasPrice(ctx)
	if err != nil {
		return &big.Int{}, err
	}

	return gasPrice, nil
}

func (g geth) EstimateGasLimit(ctx context.Context, address string, data []byte) (uint64, error) {
	a := common.HexToAddress(address)

	gasLimit, err := g.client.EstimateGas(ctx, ethereum.CallMsg{
		To:   &a,
		Data: data,
	})
	if err != nil {
		return 0, err
	}

	return gasLimit, nil
}

func (g geth) TransferBNB(
	ctx context.Context,
	senderPrivateKey []byte,
	receiverPublicKey string,
	amount *big.Int,
	gasPrice *big.Int,
) (string, error) {
	fmt.Printf("start transfer ETH %s\n\n", amount)
	fmt.Printf("start transfer Gas Price %s\n\n", gasPrice)
	spk := crypto.ToECDSAUnsafe(senderPrivateKey)
	defer g.zeroKey(spk)

	publicKey := spk.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return "", ErrCastToECDSA
	}

	senderAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	nonce, err := g.client.PendingNonceAt(ctx, senderAddress)
	if err != nil {
		return "", err
	}

	chainID, err := g.GetChainID(ctx)
	if err != nil {
		return "", err
	}

	userAddressPk := common.HexToAddress(receiverPublicKey)
	fmt.Printf("userAddresPk %s\n\n", userAddressPk)
	tx := types.NewTransaction(nonce, userAddressPk, amount, bnbGasLimit, gasPrice, nil)
	fmt.Printf("\n\n Bnb Transfer to user account for gas fee %s", tx)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), spk)

	if err != nil {
		return "", err
	}

	//TODO: got error SendTransaction
	err = g.client.SendTransaction(ctx, signedTx)
	if err != nil {
		return "", err
	}
	fmt.Printf("\n\n err   Bnb Transfer to user account for gas fee %s \n", err)
	fmt.Printf("\n\n  signedTxe %s \n", signedTx.Hash().Hex())

	return signedTx.Hash().Hex(), nil
}

func (g geth) TransferUSDT(
	ctx context.Context,
	senderPrivateKey []byte,
	receiverPublicKey string,
	amount *big.Int,
	gasPrice *big.Int,
	tokenType string,
) (string, error) {
	if amount.Cmp(big.NewInt(0)) > 0 {
		spk := crypto.ToECDSAUnsafe(senderPrivateKey)
		defer g.zeroKey(spk)

		publicKey := spk.Public()
		publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
		if !ok {
			return "", ErrCastToECDSA
		}

		senderAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
		nonce, err := g.client.PendingNonceAt(ctx, senderAddress)
		if err != nil {
			return "", err
		}

		receiverAddress := common.HexToAddress(receiverPublicKey)

		hash := sha3.NewLegacyKeccak256()
		hash.Write([]byte("transfer(address,uint256)"))
		methodID := hash.Sum(nil)[:4]

		paddedAddress := common.LeftPadBytes(receiverAddress.Bytes(), leftPadBytesLength)
		paddedAmount := common.LeftPadBytes(amount.Bytes(), leftPadBytesLength)

		var data []byte
		data = append(data, methodID...)
		data = append(data, paddedAddress...)
		data = append(data, paddedAmount...)

		chainID, err := g.GetChainID(ctx)
		if err != nil {
			return "", err
		}
		contract := common.HexToAddress(g.cfg.Blockchain.BNBUSDTContract)

		tx := types.NewTransaction(nonce, contract, big.NewInt(0), bnb20GasLimit, gasPrice, data)

		signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), spk)
		if err != nil {
			return "", err
		}

		err = g.client.SendTransaction(ctx, signedTx)
		if err != nil {
			return "", err
		}

		return signedTx.Hash().Hex(), nil
	}

	return "", nil

}

func (g geth) GetTransaction(ctx context.Context, hash common.Hash) (*types.Transaction, bool, error) {
	tx, isPending, err := g.client.TransactionByHash(ctx, hash)
	if err != nil {
		return &types.Transaction{}, isPending, err
	}

	return tx, isPending, nil
}

func (g geth) GetTransactionInfo(ctx context.Context, hash common.Hash) (
	*types.Receipt,
	*types.Transaction,
	bool,
	error,
) {
	tx, isPending, err := g.GetTransaction(ctx, hash)
	if err != nil {
		return &types.Receipt{}, &types.Transaction{}, isPending, err
	}

	receipt, err := g.GetTransactionReceipt(ctx, hash)
	if err != nil {
		return &types.Receipt{}, &types.Transaction{}, isPending, err
	}

	return receipt, tx, isPending, nil
}

func (g geth) GetTransactionReceipt(ctx context.Context, hash common.Hash) (*types.Receipt, error) {
	receipt, err := g.client.TransactionReceipt(ctx, hash)
	if err != nil {
		return &types.Receipt{}, err
	}

	return receipt, nil
}

func (g geth) GetLatestBlock(ctx context.Context) (*big.Int, error) {
	header, err := g.client.HeaderByNumber(ctx, nil)
	if err != nil {
		return &big.Int{}, err
	}

	return header.Number, nil
}

func (g geth) GetTransactionByBlockNum(ctx context.Context, blockNum *big.Int) (types.Transactions, error) {
	block, err := g.client.BlockByNumber(ctx, blockNum)
	if err != nil {
		return nil, err
	}

	return block.Transactions(), nil
}

func (g geth) GetChainID(ctx context.Context) (*big.Int, error) {
	chainID, err := g.client.NetworkID(ctx)
	if err != nil {
		return &big.Int{}, err
	}

	return chainID, nil
}

// zeroKey zeroes a private key in memory.
func (g geth) zeroKey(k *ecdsa.PrivateKey) {
	b := k.D.Bits()
	for i := range b {
		b[i] = 0
	}
}

func (g geth) BNBDecimals() int {
	return bnbDecimals
}

func (g geth) BNBUSDTDecimals() int {
	return bnbUsdtDecimals
}

func (g geth) LeftPadBytesLength() int {
	return leftPadBytesLength
}


func (g geth) Bnb20GasLimit() uint64 {
	return bnb20GasLimit
}

func (g geth) BNBGasLimit() uint64 {
	return bnbGasLimit
}

func (g geth) TransactionNotFoundMsg() string {
	return transactionNotFound
}
