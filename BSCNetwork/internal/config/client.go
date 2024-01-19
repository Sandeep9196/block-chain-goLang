package config

import (
	client "github.com/bnb-chain/go-sdk/client/rpc"
	"github.com/bnb-chain/go-sdk/common/types"
	"github.com/bnb-chain/go-sdk/keys"
)

func BNBConnect(configurations Config, keyManager keys.KeyManager) (*client.HTTP) {
	bnbConnect := client.NewRPCClient(configurations.Blockchain.BNBNetwork, types.TestNetwork)
	return bnbConnect
}
