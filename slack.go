package main

import (
	"fmt"
	"strings"

	"github.com/slack-go/slack"
	log "gopkg.in/inconshreveable/log15.v2"
)

func runSlack(rtm *slack.RTM, api *slack.Client) {
	for msg := range rtm.IncomingEvents {
		// log.Debug("New Slack Event", msg.Data)
		switch ev := msg.Data.(type) {
		case *slack.HelloEvent:
			// Ignore hello

		case *slack.ConnectedEvent:
			log.Debug("Slack ConnectedEvent", "Infos", ev.Info, "Connection counter", ev.ConnectionCount)

		case *slack.MessageEvent:
			//log.Debug("New Slack MessageEvent", "Event", ev)
			log.Debug("New Slack MessageEvent", "Channel", ev.Channel, "User", ev.User, "Text", ev.Text, "Ts", ev.Timestamp, "Thread TS", ev.ThreadTimestamp, "Message", ev.Msg)
			if ev.User == "" && ev.Text == "" {
				log.Info("NSM: Empty Message")
				continue
			}
			text := strings.Replace(ev.Msg.Text, "&amp;", "&", -1)
			text = strings.Replace(text, "’", "'", -1)

			var isIM bool
			channel, _, _, err := api.OpenConversation(&slack.OpenConversationParameters{ChannelID: ev.Channel})
			if err != nil {
				log.Debug("New SlackMessage", "Error getting conversation", err)
			} else {
				isIM = channel.IsIM
			}

			if !(strings.Contains(text, "!") || strings.Contains(text, "[[")) && !isIM {
				continue
			}
			if strings.HasPrefix(text, "<!") && strings.Contains(text, ">") {
				log.Debug("Slack Message", "Skipping Meta Ping", text)
				continue
			}
			totalLines.Add(1)
			slackLines.Add(1)
			user, err := api.GetUserInfo(ev.Msg.User)
			if err != nil {
				log.Warn("New SlackMessage", "Error getting user info", err)
				continue
			}
			var options []slack.RTMsgOption
			if ev.ThreadTimestamp != "" {
				options = append(options, slack.RTMsgOptionTS(ev.ThreadTimestamp))
			}
			toPrint := tokeniseAndDispatchInput(&fryatogParams{slackm: text}, getScryfallCard, getDumbScryfallCard, getRandomScryfallCard, searchScryfallCard)
			for _, s := range sliceUniqMap(toPrint) {
				if s != "" {
					rtm.SendMessage(rtm.NewOutgoingMessage(fmt.Sprintf("<@%v>: %v", user.ID, s), ev.Msg.Channel, options...))
				}
			}

		case *slack.PresenceChangeEvent:
			// fmt.Printf("Presence Change: %v\n", ev)

		case *slack.LatencyReport:
			//fmt.Printf("Current latency: %v\n", ev.Value)

		case *slack.RTMError:
			log.Error("Slack RTMError", "Error", ev.Error())

		case *slack.InvalidAuthEvent:
			log.Error("Slack InvalidAuthEvent", "event", ev)
			return

		default:
			// Ignore other events..
			// log.Debug("Unexpected: %v\n", msg.Data)
		}
	}
}
