package main

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
	"strings"
	"time"
)

var session *discordgo.Session

func setUpDiscord() {
	discordgo.Logger = func(msgL, caller int, format string, a ...interface{}) {
		switch msgL {
		case discordgo.LogError:
			log.Error().Msgf(format, a)
		case discordgo.LogWarning:
			log.Warn().Msgf(format, a)
		case discordgo.LogInformational:
			log.Info().Msgf(format, a)
		case discordgo.LogDebug:
			log.Debug().Msgf(format, a)
		}
	}

	dg, err := discordgo.New("Bot " + config.BotToken)
	if err != nil {
		log.Fatal().Err(err).Msg("error creating discord session")
	}

	session = dg
	session.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsNone)

	session.AddHandler(ready)

	if err := dg.Open(); err != nil {
		log.Fatal().Err(err).Msg("error opening discord connection")
	}
}

func ready(dg *discordgo.Session, _ *discordgo.Ready) {
	if err := dg.UpdateStatusComplex(discordgo.UpdateStatusData{
		Status: "dnd",
		Game: &discordgo.Game{
			Name: "for new reports",
			Type: discordgo.GameTypeWatching,
		},
	}); err != nil {
		log.Error().Err(err).Msg("failed to update bot status")
	}

	go func() {
		for {
			reports, err := getPendingReports()
			if err != nil {
				log.Error().Err(err).Msg("failed to get pending reports")
			} else if len(reports) > 0 {
				log.Info().Msgf("sending %d new report(s)", len(reports))
				for _, report := range reports {
					go sendReport(report)
				}
			}

			time.Sleep(3 * time.Second)
		}
	}()
}

func sendReport(report Report) {
	serverConfig, ok := config.Servers[report.Config]
	if !ok {
		log.Error().Int("report", report.ID).Str("config", report.Config).Msg("config does not exist")
		return
	}

	client, err := GetSteamUser(report.ClientSteamID)
	if err != nil {
		log.Error().Err(err).Int("report", report.ID).Uint64("steamID", report.ClientSteamID).Msg("failed to get steam user")
		return
	}
	target, err := GetSteamUser(report.TargetSteamID)
	if err != nil {
		log.Error().Err(err).Int("report", report.ID).Uint64("steamID", report.TargetSteamID).Msg("failed to get steam user")
		return
	}

	var embed discordgo.MessageEmbed
	embed.Color = 12792113
	embed.Author = &discordgo.MessageEmbedAuthor{
		Name:    fmt.Sprintf("%s [%s]", client.Name, client.SteamID.ToID()),
		URL:     client.ProfileURL,
		IconURL: client.AvatarURL,
	}
	embed.Title = fmt.Sprintf("Reported %s [%s]", target.Name, target.SteamID.ToID())
	embed.URL = target.ProfileURL
	embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
		URL: target.AvatarURL,
	}
	embed.Description = fmt.Sprintf("```%s```This player has had %d previous reports.", report.Reason, len(report.PreviousReports))
	embed.Fields = []*discordgo.MessageEmbedField{
		{Name: "Server", Value: serverConfig.ServerName, Inline: true},
		{Name: "Join Game", Value: fmt.Sprintf("steam://connect/%s", serverConfig.ServerHost), Inline: true},
	}
	embed.Footer = &discordgo.MessageEmbedFooter{
		Text: fmt.Sprintf("Report #%d", report.ID),
	}
	embed.Timestamp = report.Created.Format(time.RFC3339)

	var mentions []string
	for _, role := range serverConfig.Roles {
		mentions = append(mentions, fmt.Sprintf("<@&%s>", role))
	}
	for _, admin := range serverConfig.Admins {
		mentions = append(mentions, fmt.Sprintf("<@%s>", admin))
	}

	var content string
	if len(mentions) > 0 {
		content = fmt.Sprintf("**⇓ %s ⇓**", strings.Join(mentions, " "))
	}

	for _, channel := range serverConfig.Channels {
		if _, err := session.ChannelMessageSendComplex(channel, &discordgo.MessageSend{
			Content: content,
			Embed:   &embed,
		}); err != nil {
			log.Error().Err(err).Int("report", report.ID).Str("channel", channel).Msg("failed to send report")
		}
	}
}

func closeDiscord() {
	if err := session.Close(); err != nil {
		log.Error().Err(err).Msg("failed to safely close discord connection")
	}
}
