package main

import (
	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

type (
	Config struct {
		DSN      string
		BotToken string
		SteamKey string

		Servers map[string]Server
	}

	Server struct {
		ServerName string   `json:"serverName"`
		ServerHost string   `json:"serverHost"`
		Channels   []string `json:"channels"`
		Roles      []string `json:"roles"`
		Admins     []string `json:"admins"`
	}
)

var config Config

func setUpConfig() {
	viper.AddConfigPath(".")
	viper.SetConfigName("config")
	viper.SetConfigType("json")

	viper.SetDefault("DSN", "username:password@protocol+hostspec/database?parseTime=true")
	viper.SetDefault("BotToken", "")
	viper.SetDefault("SteamKey", "")
	viper.SetDefault("Servers", map[string]Server{
		"default": {
			ServerName: "Default",
			ServerHost: "127.0.0.1:27015",
			Channels:   []string{},
			Roles:      []string{},
			Admins:     []string{},
		},
	})

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			if err := viper.SafeWriteConfig(); err != nil {
				log.Fatal().Err(err).Msg("got error while writing configuration")
			}
		} else {
			log.Fatal().Err(err).Msg("got error while reading configuration")
		}
	}

	if err := viper.Unmarshal(&config); err != nil {
		log.Fatal().Err(err).Msg("unable to decode config into struct")
	}

	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		log.Info().Msg("detected config file change")
		if err := viper.Unmarshal(&config); err != nil {
			log.Error().Err(err).Msg("unable to decode config into struct")
		}
	})
}
