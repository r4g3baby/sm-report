package sm_report

import (
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/r4g3baby/sm-report/config"
	"github.com/r4g3baby/sm-report/database"
	"gorm.io/gorm"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func init() {
	handlers = append(handlers, handler{f: func(s *discordgo.Session, _ *discordgo.Ready) {
		if _, err := scheduler.CronWithSeconds("*/3 * * * * *").SingletonMode().StartImmediately().Do(func() {
			var pendingReports []database.Report
			if result := database.DB.Preload("Comments").Where("message_id IS NULL").Find(&pendingReports); result.Error != nil {
				config.Logger.Errorw("failed to query database",
					"error", result.Error,
				)
				return
			}

			for _, report := range pendingReports {
				serverConfig, ok := config.Config.Servers[report.Config]
				if !ok {
					config.Logger.Errorw("config does not exist",
						"report", report.ID,
						"config", report.Config,
					)
					return
				}

				var msg *discordgo.Message
				if embed, err := getReportEmbed(report, serverConfig); err == nil {
					if msg, err = s.ChannelMessageSendComplex(serverConfig.Channel, &discordgo.MessageSend{
						Content: getReportMentions(serverConfig.Mentions),
						Embed:   embed,
						Components: []discordgo.MessageComponent{
							discordgo.ActionsRow{
								Components: getReportButtons(report),
							},
						},
					}); err != nil {
						config.Logger.Errorw("failed to send report message",
							"report", report.ID,
							"channel", serverConfig.Channel,
							"error", err,
						)
						return
					}
				} else {
					config.Logger.Errorw("failed to create report embed",
						"report", report.ID,
						"error", err,
					)
					return
				}

				channelID, _ := strconv.ParseUint(msg.ChannelID, 10, 64)
				messageID, _ := strconv.ParseUint(msg.ID, 10, 64)
				if result := database.DB.Model(&report).Updates(database.Report{
					ChannelID: &channelID, MessageID: &messageID,
				}); result.Error != nil {
					_ = s.ChannelMessageDelete(msg.ChannelID, msg.ID)
					config.Logger.Errorw("failed to update report channel and message id",
						"error", result.Error,
					)
				}
			}
		}); err != nil {
			config.Logger.Errorw("failed to create reports finder job",
				"error", err,
			)
		}

		if _, err := scheduler.Cron("0 * * * *").SingletonMode().StartImmediately().Do(func() {
			var pendingReports []database.Report
			query := "message_id IS NOT NULL AND admin_id IS NULL AND status = ? AND DATEDIFF(current_time, created_at) >= 1"
			if result := database.DB.Preload("Comments").Where(query, database.StatusUnknown).Find(&pendingReports); result.Error != nil {
				config.Logger.Errorw("failed to query database",
					"error", result.Error,
				)
				return
			}

			for _, report := range pendingReports {
				if result := database.DB.Model(&report).Updates(database.Report{Status: database.StatusAutoClosed}); result.Error != nil {
					config.Logger.Errorw("failed to update report message status",
						"error", result.Error,
					)
				}

				serverConfig, ok := config.Config.Servers[report.Config]
				if !ok {
					config.Logger.Errorw("config does not exist",
						"report", report.ID,
						"config", report.Config,
					)
					return
				}

				if err := updateReportMessage(report, serverConfig); err != nil {
					config.Logger.Errorw("failed to update report message",
						"report", report.ID,
						"error", err,
					)
				}
			}
		}); err != nil {
			config.Logger.Errorw("failed to create reports auto close job",
				"error", err,
			)
		}
	}, once: true})

	commands["reports"] = command{
		command: &discordgo.ApplicationCommand{
			Type:        discordgo.ChatApplicationCommand,
			Description: "Interact with the reports system",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "view",
					Description: "View a report",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionInteger,
							Name:        "report",
							Description: "Report ID",
							Required:    true,
						},
					},
				},
			},
		},
		handler: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			var applicationCommandData = i.ApplicationCommandData().Options[0]
			switch applicationCommandData.Name {
			case "view":
				if !isServerAdmin(i.Member) {
					if err := simpleInteractionResponse(i.Interaction, "You must be an admin in order to use this command."); err != nil {
						config.Logger.Errorw("failed to respond to interaction",
							"error", err,
						)
					}
					return
				}

				var report database.Report
				if result := database.DB.Preload("Comments").First(&report, applicationCommandData.Options[0].UintValue()); result.Error != nil {
					if errors.Is(result.Error, gorm.ErrRecordNotFound) {
						if err := simpleInteractionResponse(i.Interaction, "Report does not exist."); err != nil {
							config.Logger.Errorw("failed to respond to interaction",
								"error", err,
							)
						}
					} else {
						config.Logger.Errorw("failed to query database",
							"error", result.Error,
						)
					}

					return
				}

				serverConfig, ok := config.Config.Servers[report.Config]
				if !ok {
					config.Logger.Errorw("config does not exist",
						"report", report.ID,
						"config", report.Config,
					)
					return
				}

				if embed, err := getReportEmbed(report, serverConfig); err == nil {
					var embeds = []*discordgo.MessageEmbed{embed}

					if len(report.Comments) > 0 {
						embeds = append(embeds, getCommentsEmbed(report))
					}

					if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Embeds: embeds,
							Flags:  1 << 6,
						},
					}); err != nil {
						config.Logger.Errorw("failed to respond to interaction",
							"error", err,
						)
					}
				} else {
					config.Logger.Errorw("failed to create report embed",
						"report", report.ID,
						"error", err,
					)
				}
			default:
				if err := simpleInteractionResponse(i.Interaction, "Oops, something gone wrong.\nHol' up, you aren't supposed to see this message."); err != nil {
					config.Logger.Errorw("failed to respond to interaction",
						"error", err,
					)
				}
			}
		},
	}

	components["report_handle"] = func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		var report database.Report
		query := "channel_id = ? AND message_id = ?"
		if result := database.DB.Preload("Comments").First(&report, query, i.Message.ChannelID, i.Message.ID); result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				if err := simpleInteractionResponse(i.Interaction, "Report does not exist."); err != nil {
					config.Logger.Errorw("failed to respond to interaction",
						"error", err,
					)
				}
			} else {
				config.Logger.Errorw("failed to query database",
					"error", result.Error,
				)
			}
			return
		}

		serverConfig, ok := config.Config.Servers[report.Config]
		if !ok {
			config.Logger.Errorw("config does not exist",
				"report", report.ID,
				"config", report.Config,
			)
			return
		}

		if report.AdminID != nil {
			// Handle button shouldn't be enabled, so we update the message to make sure
			if err := updateReportMessage(report, serverConfig); err != nil {
				config.Logger.Errorw("failed to update report message",
					"report", report.ID,
					"error", err,
				)
			}

			var msg = "This report already belongs to someone else."
			if strconv.FormatUint(*report.AdminID, 10) == i.Member.User.ID {
				msg = "This report already belongs to you."
			}

			if err := simpleInteractionResponse(i.Interaction, msg); err != nil {
				config.Logger.Errorw("failed to respond to interaction",
					"error", err,
				)
			}
			return
		}

		if !canHandleReport(serverConfig, i.Member) {
			if err := simpleInteractionResponse(i.Interaction, "You are not allowed to handle this report."); err != nil {
				config.Logger.Errorw("failed to respond to interaction",
					"error", err,
				)
			}
			return
		}

		adminID, _ := strconv.ParseUint(i.Member.User.ID, 10, 64)
		report.AdminID = &adminID

		if result := database.DB.Model(&report).Updates(database.Report{AdminID: report.AdminID}); result.Error != nil {
			config.Logger.Errorw("failed to update report admin id",
				"error", result.Error,
			)
			return
		}

		if err := updateReportMessage(report, serverConfig); err != nil {
			config.Logger.Errorw("failed to update report message",
				"report", report.ID,
				"error", err,
			)
		}

		if err := simpleInteractionResponse(i.Interaction, "You are now responsible for this report."); err != nil {
			config.Logger.Errorw("failed to respond to interaction",
				"error", err,
			)
		}
	}

	components["report_verify"] = func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		var report database.Report
		query := "channel_id = ? AND message_id = ?"
		if result := database.DB.Preload("Comments").First(&report, query, i.Message.ChannelID, i.Message.ID); result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				if err := simpleInteractionResponse(i.Interaction, "Report does not exist."); err != nil {
					config.Logger.Errorw("failed to respond to interaction",
						"error", err,
					)
				}
			} else {
				config.Logger.Errorw("failed to query database",
					"error", result.Error,
				)
			}
			return
		}

		serverConfig, ok := config.Config.Servers[report.Config]
		if !ok {
			config.Logger.Errorw("config does not exist",
				"report", report.ID,
				"config", report.Config,
			)
			return
		}

		if report.AdminID == nil || strconv.FormatUint(*report.AdminID, 10) != i.Member.User.ID {
			// Update the report message either way just because why not
			if err := updateReportMessage(report, serverConfig); err != nil {
				config.Logger.Errorw("failed to update report message",
					"report", report.ID,
					"error", err,
				)
			}

			if err := simpleInteractionResponse(i.Interaction, "This report does not belong to you."); err != nil {
				config.Logger.Errorw("failed to respond to interaction",
					"error", err,
				)
			}
			return
		}

		report.Status = database.StatusVerified
		if result := database.DB.Model(&report).Updates(database.Report{Status: report.Status}); result.Error != nil {
			config.Logger.Errorw("failed to update report status",
				"error", result.Error,
			)
			return
		}

		if err := updateReportMessage(report, serverConfig); err != nil {
			config.Logger.Errorw("failed to update report message",
				"report", report.ID,
				"error", err,
			)
		}

		if err := simpleInteractionResponse(i.Interaction, "This report has been marked as verified."); err != nil {
			config.Logger.Errorw("failed to respond to interaction",
				"error", err,
			)
		}
	}

	components["report_falsify"] = func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		var report database.Report
		query := "channel_id = ? AND message_id = ?"
		if result := database.DB.Preload("Comments").First(&report, query, i.Message.ChannelID, i.Message.ID); result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				if err := simpleInteractionResponse(i.Interaction, "Report does not exist."); err != nil {
					config.Logger.Errorw("failed to respond to interaction",
						"error", err,
					)
				}
			} else {
				config.Logger.Errorw("failed to query database",
					"error", result.Error,
				)
			}
			return
		}

		serverConfig, ok := config.Config.Servers[report.Config]
		if !ok {
			config.Logger.Errorw("config does not exist",
				"report", report.ID,
				"config", report.Config,
			)
			return
		}

		if report.AdminID == nil || strconv.FormatUint(*report.AdminID, 10) != i.Member.User.ID {
			// Update the report message either way just because why not
			if err := updateReportMessage(report, serverConfig); err != nil {
				config.Logger.Errorw("failed to update report message",
					"report", report.ID,
					"error", err,
				)
			}

			if err := simpleInteractionResponse(i.Interaction, "This report does not belong to you."); err != nil {
				config.Logger.Errorw("failed to respond to interaction",
					"error", err,
				)
			}
			return
		}

		report.Status = database.StatusFalsified
		if result := database.DB.Model(&report).Updates(database.Report{Status: report.Status}); result.Error != nil {
			config.Logger.Errorw("failed to update report status",
				"error", result.Error,
			)
			return
		}

		if err := updateReportMessage(report, serverConfig); err != nil {
			config.Logger.Errorw("failed to update report message",
				"report", report.ID,
				"error", err,
			)
		}

		if err := simpleInteractionResponse(i.Interaction, "This report has been marked as falsified."); err != nil {
			config.Logger.Errorw("failed to respond to interaction",
				"error", err,
			)
		}
	}

	components["report_comment"] = func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		var report database.Report
		query := "channel_id = ? AND message_id = ?"
		if result := database.DB.Preload("Comments").First(&report, query, i.Message.ChannelID, i.Message.ID); result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				if err := simpleInteractionResponse(i.Interaction, "Report does not exist."); err != nil {
					config.Logger.Errorw("failed to respond to interaction",
						"error", err,
					)
				}
			} else {
				config.Logger.Errorw("failed to query database",
					"error", result.Error,
				)
			}
			return
		}

		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseModal,
			Data: &discordgo.InteractionResponseData{
				CustomID: fmt.Sprintf("report_comment_%d", report.ID),
				Title:    "Add a Comment",
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.TextInput{
								CustomID:  "comment",
								Label:     "What's your comment?",
								Style:     discordgo.TextInputParagraph,
								Required:  true,
								MaxLength: 256,
							},
						},
					},
				},
			},
		}); err != nil {
			config.Logger.Errorw("failed to respond to interaction",
				"error", err,
			)
		}
	}

	regexComponents[regexp.MustCompile("report_comment_(\\d+)")] = func(s *discordgo.Session, i *discordgo.InteractionCreate, match []string) {
		var report database.Report
		if result := database.DB.Preload("Comments").First(&report, "id = ?", match[1]); result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				if err := simpleInteractionResponse(i.Interaction, "Report does not exist."); err != nil {
					config.Logger.Errorw("failed to respond to interaction",
						"error", err,
					)
				}
			} else {
				config.Logger.Errorw("failed to query database",
					"error", result.Error,
				)
			}
			return
		}

		serverConfig, ok := config.Config.Servers[report.Config]
		if !ok {
			config.Logger.Errorw("config does not exist",
				"report", report.ID,
				"config", report.Config,
			)
			return
		}

		adminID, _ := strconv.ParseInt(i.Member.User.ID, 10, 64)
		comment := database.Comment{
			ReportID: report.ID,
			AdminID:  uint64(adminID),
			Text:     i.ModalSubmitData().Components[0].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value,
		}

		if result := database.DB.Create(&comment); result.Error != nil {
			config.Logger.Errorw("failed to query database",
				"error", result.Error,
			)
			return
		}
		report.Comments = append(report.Comments, comment)

		if err := updateReportMessage(report, serverConfig); err != nil {
			config.Logger.Errorw("failed to update report message",
				"report", report.ID,
				"error", err,
			)
		}

		if err := simpleInteractionResponse(i.Interaction, "Comment added."); err != nil {
			config.Logger.Errorw("failed to respond to interaction",
				"error", err,
			)
		}
	}

	components["report_more_info"] = func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		var report database.Report
		query := "channel_id = ? AND message_id = ?"
		if result := database.DB.Preload("Comments").First(&report, query, i.Message.ChannelID, i.Message.ID); result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				if err := simpleInteractionResponse(i.Interaction, "Report does not exist."); err != nil {
					config.Logger.Errorw("failed to respond to interaction",
						"error", err,
					)
					return
				}
			} else {
				config.Logger.Errorw("failed to query database",
					"error", result.Error,
				)
			}
			return
		}

		serverConfig, ok := config.Config.Servers[report.Config]
		if !ok {
			config.Logger.Errorw("config does not exist",
				"report", report.ID,
				"config", report.Config,
			)
			return
		}

		// Update the report message either way just because why not
		if err := updateReportMessage(report, serverConfig); err != nil {
			config.Logger.Errorw("failed to update report message",
				"report", report.ID,
				"error", err,
			)
		}

		var embeds []*discordgo.MessageEmbed

		if clientInfoEmbed, err := getClientMoreInfo(report); err == nil {
			embeds = append(embeds, clientInfoEmbed)
		} else {
			config.Logger.Errorw("failed to create report client more information embed",
				"report", report.ID,
				"error", err,
			)
		}

		if report.TargetSteamID != nil {
			if targetInfoEmbed, err := getTargetMoreInfo(report); err == nil {
				embeds = append(embeds, targetInfoEmbed)
			} else {
				config.Logger.Errorw("failed to create report target more information embed",
					"report", report.ID,
					"error", err,
				)
			}
		}

		if len(report.Comments) > 0 {
			embeds = append(embeds, getCommentsEmbed(report))
		}

		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: embeds,
				Flags:  1 << 6,
			},
		}); err != nil {
			config.Logger.Errorw("failed to respond to interaction",
				"error", err,
			)
		}
	}
}

func updateReportMessage(report database.Report, serverConfig config.Server) error {
	if embed, err := getReportEmbed(report, serverConfig); err == nil {
		var content = getReportMentions(serverConfig.Mentions)
		if _, err := session.ChannelMessageEditComplex(&discordgo.MessageEdit{
			Channel: strconv.FormatUint(*report.ChannelID, 10),
			ID:      strconv.FormatUint(*report.MessageID, 10),
			Content: &content,
			Embed:   embed,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: getReportButtons(report),
				},
			},
		}); err != nil {
			return err
		}
	} else {
		return err
	}
	return nil
}

func getReportMentions(mentions config.Mentions) string {
	var mentionsList []string
	for _, role := range mentions.Roles {
		mentionsList = append(mentionsList, fmt.Sprintf("<@&%s>", role))
	}
	for _, admin := range mentions.Users {
		mentionsList = append(mentionsList, fmt.Sprintf("<@%s>", admin))
	}
	if len(mentionsList) > 0 {
		return fmt.Sprintf("**⇓ %s ⇓**", strings.Join(mentionsList, " "))
	}
	return ""
}

func getReportEmbed(report database.Report, server config.Server) (*discordgo.MessageEmbed, error) {
	if report.TargetSteamID != nil {
		return userReportEmbed(report, server)
	}
	return serverReportEmbed(report, server)
}

func userReportEmbed(report database.Report, server config.Server) (*discordgo.MessageEmbed, error) {
	client, err := getSteamUser(report.ClientSteamID)
	if err != nil {
		return nil, err
	}

	target, err := getSteamUser(*report.TargetSteamID)
	if err != nil {
		return nil, err
	}

	var admin = "Press the **handle** button."
	if report.Status == database.StatusAutoClosed {
		admin = "None"
	} else if report.AdminID != nil {
		admin = fmt.Sprintf("<@%d>", *report.AdminID)
	}

	return &discordgo.MessageEmbed{
		Color: 0xc33131,
		Author: &discordgo.MessageEmbedAuthor{
			Name:    fmt.Sprintf("%s [%s]", client.Name, client.SteamID.ToID()),
			URL:     client.ProfileURL,
			IconURL: client.AvatarURL,
		},
		Title: fmt.Sprintf("Reported %s [%s]", target.Name, target.SteamID.ToID()),
		URL:   target.ProfileURL,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: target.AvatarURL,
		},
		Description: fmt.Sprintf("```%s```", report.Reason),
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Reported SteamID", Value: target.SteamID.ToID().String(), Inline: true},
			{Name: "Reported IP", Value: *report.TargetIP, Inline: true},
			{Name: "Comments", Value: strconv.Itoa(len(report.Comments)), Inline: true},
			{Name: "Admin Handling Report", Value: admin, Inline: true},
			{Name: "Status", Value: report.Status.String(), Inline: true},
			{Name: fmt.Sprintf("Join %s", server.Name), Value: fmt.Sprintf("steam://connect/%s", report.ServerIP), Inline: false},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Report #%d", report.ID),
		},
		Timestamp: report.CreatedAt.Format(time.RFC3339),
	}, nil
}

func serverReportEmbed(report database.Report, server config.Server) (*discordgo.MessageEmbed, error) {
	client, err := getSteamUser(report.ClientSteamID)
	if err != nil {
		return nil, err
	}

	var admin = "Press the **handle** button."
	if report.Status == database.StatusAutoClosed {
		admin = "None"
	} else if report.AdminID != nil {
		admin = fmt.Sprintf("<@%d>", *report.AdminID)
	}

	return &discordgo.MessageEmbed{
		Color: 0xc33131,
		Author: &discordgo.MessageEmbedAuthor{
			Name:    fmt.Sprintf("%s [%s]", client.Name, client.SteamID.ToID()),
			URL:     client.ProfileURL,
			IconURL: client.AvatarURL,
		},
		Title: "Reported a Server Issue",
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: "https://cdn.discordapp.com/icons/224706012697985025/a_24d5284d77886a56da2d9525915f7bee.webp",
		},
		Description: fmt.Sprintf("```%s```", report.Reason),
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Comments", Value: strconv.Itoa(len(report.Comments)), Inline: true},
			{Name: "Admin Handling Report", Value: admin, Inline: true},
			{Name: "Status", Value: report.Status.String(), Inline: true},
			{Name: fmt.Sprintf("Join %s", server.Name), Value: fmt.Sprintf("steam://connect/%s", report.ServerIP), Inline: false},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Report #%d", report.ID),
		},
		Timestamp: report.CreatedAt.Format(time.RFC3339),
	}, nil
}

func getClientMoreInfo(report database.Report) (*discordgo.MessageEmbed, error) {
	steamUser, err := getSteamUser(report.ClientSteamID)
	if err != nil {
		return nil, err
	}

	var clientReports []database.Report
	if result := database.DB.Find(&clientReports, "client_steam_id = ?", report.ClientSteamID); result.Error != nil {
		return nil, result.Error
	}

	var verifiedReports int
	var falsifiedReports int
	for _, report := range clientReports {
		if report.Status == database.StatusVerified {
			verifiedReports++
		} else if report.Status == database.StatusFalsified {
			falsifiedReports++
		}
	}

	var totalReports = verifiedReports + falsifiedReports
	var verifiedPercent, falsifiedPercent = 0, 0
	if totalReports > 0 {
		verifiedPercent = (verifiedReports * 100) / totalReports
		falsifiedPercent = (falsifiedReports * 100) / totalReports
	}

	return &discordgo.MessageEmbed{
		Color: 0xc33131,
		Title: "Reporter Information",
		URL:   steamUser.ProfileURL,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: steamUser.AvatarURL,
		},
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Name", Value: steamUser.Name, Inline: true},
			{Name: "SteamID", Value: steamUser.SteamID.ToID().String(), Inline: true},
			{Name: "IP", Value: report.ClientIP, Inline: true},
			{Name: "Filled Reports", Value: strconv.Itoa(len(clientReports)), Inline: true},
			{Name: "Verified", Value: fmt.Sprintf("%d [%d%%]", verifiedReports, verifiedPercent), Inline: true},
			{Name: "Falsified", Value: fmt.Sprintf("%d [%d%%]", falsifiedReports, falsifiedPercent), Inline: true},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Report #%d", report.ID),
		},
		Timestamp: report.CreatedAt.Format(time.RFC3339),
	}, nil
}

func getTargetMoreInfo(report database.Report) (*discordgo.MessageEmbed, error) {
	steamUser, err := getSteamUser(*report.TargetSteamID)
	if err != nil {
		return nil, err
	}

	var targetReports []database.Report
	if result := database.DB.Find(&targetReports, "target_steam_id = ?", report.TargetSteamID); result.Error != nil {
		return nil, result.Error
	}

	var verifiedReports int
	var falsifiedReports int
	for _, report := range targetReports {
		if report.Status == database.StatusVerified {
			verifiedReports++
		} else if report.Status == database.StatusFalsified {
			falsifiedReports++
		}
	}

	var totalReports = verifiedReports + falsifiedReports
	var verifiedPercent, falsifiedPercent = 0, 0
	if totalReports > 0 {
		verifiedPercent = (verifiedReports * 100) / totalReports
		falsifiedPercent = (falsifiedReports * 100) / totalReports
	}

	return &discordgo.MessageEmbed{
		Color: 0xc33131,
		Title: "Reported Information",
		URL:   steamUser.ProfileURL,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: steamUser.AvatarURL,
		},
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Name", Value: steamUser.Name, Inline: true},
			{Name: "SteamID", Value: steamUser.SteamID.ToID().String(), Inline: true},
			{Name: "IP", Value: *report.TargetIP, Inline: true},
			{Name: "Times Reported", Value: strconv.Itoa(len(targetReports)), Inline: true},
			{Name: "Verified", Value: fmt.Sprintf("%d [%d%%]", verifiedReports, verifiedPercent), Inline: true},
			{Name: "Falsified", Value: fmt.Sprintf("%d [%d%%]", falsifiedReports, falsifiedPercent), Inline: true},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Report #%d", report.ID),
		},
		Timestamp: report.CreatedAt.Format(time.RFC3339),
	}, nil
}

func getCommentsEmbed(report database.Report) *discordgo.MessageEmbed {
	var commentsList []string
	for _, comment := range report.Comments {
		admin := fmt.Sprintf("<@%d>", comment.AdminID)
		when := fmt.Sprintf("<t:%d>", comment.CreatedAt.Unix())
		commentsList = append(commentsList, fmt.Sprintf("%s - %s\n%s", admin, when, comment.Text))
	}
	return &discordgo.MessageEmbed{
		Color:       0xc33131,
		Title:       "List of Comments",
		Description: strings.Join(commentsList, "\n"),
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Report #%d", report.ID),
		},
		Timestamp: report.CreatedAt.Format(time.RFC3339),
	}
}

func getReportButtons(report database.Report) []discordgo.MessageComponent {
	var (
		handleBtn = discordgo.Button{
			CustomID: "report_handle",
			Label:    "Handle",
			Style:    discordgo.PrimaryButton,
		}
		verifyBtn = discordgo.Button{
			CustomID: "report_verify",
			Label:    "Verify",
			Style:    discordgo.SuccessButton,
		}
		falsifyBtn = discordgo.Button{
			CustomID: "report_falsify",
			Label:    "Falsify",
			Style:    discordgo.DangerButton,
		}
		commentBtn = discordgo.Button{
			CustomID: "report_comment",
			Label:    "Comment",
			Style:    discordgo.PrimaryButton,
		}
		moreInfoBtn = discordgo.Button{
			CustomID: "report_more_info",
			Label:    "More Information",
			Style:    discordgo.SecondaryButton,
		}
	)

	if report.Status == database.StatusAutoClosed {
		return []discordgo.MessageComponent{
			commentBtn, moreInfoBtn,
		}
	}

	if report.AdminID != nil {
		if report.Status == database.StatusUnknown {
			return []discordgo.MessageComponent{
				verifyBtn, falsifyBtn, commentBtn, moreInfoBtn,
			}
		}

		return []discordgo.MessageComponent{
			commentBtn, moreInfoBtn,
		}
	}

	return []discordgo.MessageComponent{
		handleBtn, commentBtn, moreInfoBtn,
	}
}
