package config

import (
	"github.com/spf13/viper"
)

type PhraseConfig struct {
	Phrase               string `mapstructure:"mnemonicPhrase"`
	BnbReconcileAddress  string `mapstructure:"bnb_reconcile_address"`
}

func PhraseConfigLoad() (PhraseConfig, error) {
	file := "phrase"
	viper.SetConfigName(file)
	viper.SetConfigType("yml")
	viper.AddConfigPath("./")
	viper.AddConfigPath("../../")
	viper.AddConfigPath("../../../")
	viper.AddConfigPath("../../../../")

	conf := new(PhraseConfig)

	if err := viper.ReadInConfig(); err != nil {
		return *conf, err
	}

	if err := viper.Unmarshal(&conf); err != nil {
		return *conf, err
	}

	return *conf, nil
}
