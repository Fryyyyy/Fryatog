package main

import (
	"encoding/json"
	"os"
	"strings"

	log "gopkg.in/inconshreveable/log15.v2"
)

// Configuration lists the configurable parameters, stored in config.json
type configuration struct {
	DSN      string `json:"DSN"`
	Password string `json:"Password"`
	DevMode  bool   `json:"DevMode"`
	Server   struct {
		SSL    string `json:"SSL"`
		NonSSL string `json:"NonSSL"`
	}
	ProdChannels []string `json:"ProdChannels"`
	DevChannels  []string `json:"DevChannels"`
	Ops          []string `json:"Ops"`
	ProdNick     string   `json:"ProdNick"`
	DevNick      string   `json:"DevNick"`
	SlackTokens  []string `json:"SlackTokens"`
	Hearthstone  struct {
		AppID     string `json:"AppID"`
		APIToken  string `json:"APIToken"`
		IndexName string `json:"IndexName"`
	} `json:"Hearthstone"`
	BattleNet struct {
		ClientID         string   `json:"ClientID"`
		ClientSecret     string   `json:"ClientSecret"`
		CurrentExpansion string   `json:"CurrentExpansion"`
		CurrentRaidTier  string   `json:"CurrentRaidTier"`
		Reputations      []string `json:"Reputations"`
	} `json:"BattleNet"`
	PoE struct {
		League           string   `json:"League"`
		WantedCurrencies []string `json:"WantedCurrencies"`
	} `json:"PoE"`
	IRC   bool `json:"IRC"`
	Slack bool `json:"Slack"`
}

const (
	defaultConfigFilePath  = "config.json"
	configPathEnvVariable  = "CONFIG_PATH"
	passwordEnvVariable    = "IRC_PASSWORD"
	dsnEnvVariable         = "SENTRY_DSN"
	slackTokensEnvVariable = "SLACK_TOKENS"
)

func readConfig() configuration {
	file, err := os.Open(getConfigFilePath())
	if err != nil {
		panic(err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	conf := configuration{}
	err = decoder.Decode(&conf)
	if err != nil {
		panic(err)
	}
	log.Debug("Conf", "Parsed as", conf)
	return conf
}

func getConfigFilePath() string {
	path := os.Getenv(configPathEnvVariable)
	if path == "" {
		path = defaultConfigFilePath
	}
	return path
}

func getPassword(conf *configuration) string {
	password := os.Getenv(passwordEnvVariable)
	if password == "" {
		password = conf.Password
	}
	return password
}

func getDSN(conf *configuration) string {
	dsn := os.Getenv(dsnEnvVariable)
	if dsn == "" {
		dsn = conf.DSN
	}
	return dsn
}

func getSlackTokens(conf *configuration) []string {
	tokenStr := os.Getenv(slackTokensEnvVariable)
	if tokenStr == "" {
		return conf.SlackTokens
	}
	return strings.Split(tokenStr, ",")
}
