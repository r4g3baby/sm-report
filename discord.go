package sm_report

import (
	"github.com/bwmarrin/discordgo"
	"github.com/go-co-op/gocron"
	"github.com/r4g3baby/sm-report/config"
	"time"
)

type handler struct {
	f    interface{}
	once bool
}

var (
	scheduler = gocron.NewScheduler(time.UTC)

	session  *discordgo.Session
	handlers []handler
)

func StartBot() {
	discordgo.Logger = func(msgL, caller int, format string, a ...interface{}) {
		switch msgL {
		case discordgo.LogError:
			config.Logger.Errorf(format, a)
		case discordgo.LogWarning:
			config.Logger.Warnf(format, a)
		case discordgo.LogInformational:
			config.Logger.Infof(format, a)
		case discordgo.LogDebug:
			config.Logger.Debugf(format, a)
		}
	}

	var err error
	if session, err = discordgo.New("Bot " + config.Config.Bot.Token); err != nil {
		config.Logger.Fatalw("failed to create bot session",
			"error", err,
		)
	}

	session.AddHandler(func(s *discordgo.Session, _ *discordgo.Ready) {
		if err := s.UpdateStatusComplex(discordgo.UpdateStatusData{
			Status: "dnd",
			Activities: []*discordgo.Activity{{
				Type: 3, // Watching
				Name: "for reports (・－・)",
			}},
		}); err != nil {
			config.Logger.Errorw("failed to update bot status",
				"error", err,
			)
		}
	})

	for _, handler := range handlers {
		if handler.once {
			session.AddHandlerOnce(handler.f)
		} else {
			session.AddHandler(handler.f)
		}
	}

	if err := session.Open(); err != nil {
		config.Logger.Fatalw("failed to open bot connection",
			"error", err,
		)
	}

	scheduler.StartAsync()

	config.Logger.Info("bot is now running")
}

func ShutdownBot() {
	scheduler.Stop()

	if err := session.Close(); err != nil {
		config.Logger.Errorw("failed to close bot session",
			"error", err,
		)
	}
}
