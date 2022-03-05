package config

import (
	"fmt"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
)

type (
	config struct {
		Debug    bool
		Database string
		SteamKey string
		Bot      bot
		Admins   Mentions
		Servers  map[string]Server
		Logger   logger
	}

	bot struct {
		Token   string
		AppID   string
		GuildID string
	}

	Server struct {
		Name     string
		Channel  string
		Mentions Mentions
	}

	Mentions struct {
		Users []string
		Roles []string
	}

	logger struct {
		Enabled    bool
		Filename   string
		MaxSize    int
		MaxAge     int
		MaxBackups int
		LocalTime  bool
		Compress   bool
	}
)

var k = koanf.New(".")
var Config config

func init() {
	if err := k.Load(structs.Provider(config{}, "koanf"), nil); err != nil {
		panic(fmt.Errorf("error loading default config: %w", err))
	}

	if err := k.Load(file.Provider("config.json"), json.Parser()); err != nil {
		panic(fmt.Errorf("error loading config: %w", err))
	}

	if err := k.Unmarshal("", &Config); err != nil {
		panic(fmt.Errorf("error unmarshaling config: %w", err))
	}
}
