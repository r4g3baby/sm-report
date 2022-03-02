package sm_report

import (
	"github.com/bwmarrin/discordgo"
	"github.com/r4g3baby/sm-report/config"
	"regexp"
)

type command struct {
	command *discordgo.ApplicationCommand
	handler func(s *discordgo.Session, i *discordgo.InteractionCreate)
}

var (
	commands        = map[string]command{}
	components      = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){}
	regexComponents = map[*regexp.Regexp]func(s *discordgo.Session, i *discordgo.InteractionCreate, match []string){}
)

func init() {
	handlers = append(handlers, handler{f: func(s *discordgo.Session, _ *discordgo.Ready) {
		if cmds, err := s.ApplicationCommands(config.Config.Bot.AppID, ""); err == nil {
			for _, cmd := range cmds {
				if err := s.ApplicationCommandDelete(config.Config.Bot.AppID, "", cmd.ID); err != nil {
					config.Logger.Errorw("failed to delete application command",
						"command", cmd.Name,
						"error", err,
					)
				}
			}
		}

		if cmds, err := s.ApplicationCommands(config.Config.Bot.AppID, ""); err == nil {
			for _, cmd := range cmds {
				if _, ok := commands[cmd.Name]; !ok {
					if err := s.ApplicationCommandDelete(config.Config.Bot.AppID, "", cmd.ID); err != nil {
						config.Logger.Errorw("failed to delete application command",
							"command", cmd.Name,
							"error", err,
						)
					}
				}
			}
		} else {
			config.Logger.Errorw("failed to list application commands",
				"error", err,
			)
		}

		for name, cmd := range commands {
			cmd.command.Name = name
			if _, err := s.ApplicationCommandCreate(config.Config.Bot.AppID, "", cmd.command); err != nil {
				config.Logger.Errorw("failed to create application command",
					"command", cmd.command.Name,
					"error", err,
				)
			}
		}
	}})

	handlers = append(handlers, handler{f: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		switch i.Type {
		case discordgo.InteractionApplicationCommand, discordgo.InteractionApplicationCommandAutocomplete:
			if command, ok := commands[i.ApplicationCommandData().Name]; ok {
				command.handler(s, i)
			}
		case discordgo.InteractionMessageComponent:
			customID := i.MessageComponentData().CustomID
			if component, ok := components[customID]; ok {
				component(s, i)
			} else {
				for regex, handler := range regexComponents {
					match := regex.FindStringSubmatch(customID)
					if len(match) > 0 {
						handler(s, i, match)
						break
					}
				}
			}
		case discordgo.InteractionModalSubmit:
			customID := i.ModalSubmitData().CustomID
			if component, ok := components[customID]; ok {
				component(s, i)
			} else {
				for regex, handler := range regexComponents {
					match := regex.FindStringSubmatch(customID)
					if len(match) > 0 {
						handler(s, i, match)
						break
					}
				}
			}
		}
	}})
}
