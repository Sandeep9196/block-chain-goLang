package config

import (
	"github.com/spf13/viper"
)

var GlobalConfig Config

type Config struct {
	App struct {
		Name string `mapstructure:"name"`
		Port int    `mapstructure:"port"`
	} `mapstructure:"app"`

	Context struct {
		Timeout int `mapstructure:"timeout"`
	} `mapstructure:"context"`

	Security struct {
		Key   string `mapstructure:"key"`
		Salt  string `mapstructure:"salt"`
		Token string `mapstructure:"token"`
	} `mapstructure:"security"`

	Dsn string `mapstructure:"dsn"`

	Redis struct {
		Host     string `mapstructure:"host"`
		Port     int    `mapstructure:"port"`
		Password string `mapstructure:"password"`
		DB       int    `mapstructure:"db"`
		PoolSize int    `mapstructure:"pool_size"`
		Timeout  int    `mapstructure:"timeout"`
	} `mapstructure:"redis"`

	Blockchain struct {
		BNBNetwork             string `mapstructure:"bnb_network"`
		BNBNet                 int    `mapstructure:"bnb_network_net"`
		BNBUSDTContract        string `mapstructure:"bnb_usdt_contract"`
		USDTContract           string `mapstructure:"usdt_contract"`
		BlockInterval          int64  `mapstructure:"block_interval"`
		USDTReconcileThreshold string `mapstructure:"usdt_reconcile_threshold"`
		BNBReconcileThreshold  string `mapstructure:"bnb_reconcile_threshold"`
		ReconcileAddress       string `mapstructure:"bnb_reconcile_address"`
	} `mapstructure:"blockchain"`

	TelegramBot struct {
		BotToken  string `mapstructure:"bot_token"`
		ChannelId int64  `mapstructure:"channel_id"`
	} `mapstructure:"telegram_bot"`
}

func Load(env string) (Config, error) {
	file := env
	viper.SetConfigName(file)
	viper.SetConfigType("yml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("../../config")
	viper.AddConfigPath("../../../config")

	conf := new(Config)

	if err := viper.ReadInConfig(); err != nil {
		return *conf, err
	}

	if err := viper.Unmarshal(&conf); err != nil {
		return *conf, err
	}

	if err := viper.Unmarshal(&GlobalConfig); err != nil {
		return Config{}, err
	}

	return *conf, nil
}
