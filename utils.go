package sm_report

import (
	"github.com/bwmarrin/discordgo"
	"github.com/r4g3baby/sm-report/config"
)

func isServerAdmin(member *discordgo.Member) bool {
	for _, mRole := range member.Roles {
		for _, aRole := range config.Config.Admins.Roles {
			if mRole == aRole {
				return true
			}
		}

		for _, server := range config.Config.Servers {
			for _, sRole := range server.Mentions.Roles {
				if mRole == sRole {
					return true
				}
			}
		}
	}

	for _, aUser := range config.Config.Admins.Users {
		if aUser == member.User.ID {
			return true
		}
	}
	for _, server := range config.Config.Servers {
		for _, sUser := range server.Mentions.Users {
			if sUser == member.User.ID {
				return true
			}
		}
	}
	return false
}

func canHandleReport(server config.Server, member *discordgo.Member) bool {
	for _, mRole := range member.Roles {
		for _, aRole := range config.Config.Admins.Roles {
			if mRole == aRole {
				return true
			}
		}

		for _, sRole := range server.Mentions.Roles {
			if mRole == sRole {
				return true
			}
		}
	}

	for _, aUser := range config.Config.Admins.Users {
		if aUser == member.User.ID {
			return true
		}
	}
	for _, sUser := range server.Mentions.Users {
		if sUser == member.User.ID {
			return true
		}
	}
	return false
}

func simpleInteractionResponse(i *discordgo.Interaction, message string) error {
	return session.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Flags:   1 << 6,
		},
	})
}
