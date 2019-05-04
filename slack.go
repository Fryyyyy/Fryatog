package main

import (
	"fmt"
	"strings"

	"github.com/nlopes/slack"
)

func runSlack(rtm *slack.RTM, api *slack.Client) {
	for msg := range rtm.IncomingEvents {
		fmt.Print("Event Received: ")
		switch ev := msg.Data.(type) {
		case *slack.HelloEvent:
			// Ignore hello

		case *slack.ConnectedEvent:
			fmt.Println("Infos:", ev.Info)
			fmt.Println("Connection counter:", ev.ConnectionCount)
			// Replace C2147483705 with your Channel ID
			// rtm.SendMessage(rtm.NewOutgoingMessage("Hello world", "C2147483705"))

		case *slack.MessageEvent:
			// fmt.Printf("Message: %v\n", ev)
			// Event Received: Message: &{{message G2BT7Q8H4 U1X8R7KNH !mountain 1556862421.206500  false [] [] <nil>  false 0  false  1556862421.206500   <nil>      [] 0 []  [] false <nil>  0 T1X8RK4K1 []  fals                  e false []} <nil> <nil>}
			text := ev.Msg.Text
			if !(strings.Contains(text, "!") || strings.Contains(text, "[[")) {
				continue
			}
			// fmt.Printf("Message: %v\n", text)
			user, err := api.GetUserInfo(ev.Msg.User)
			if err != nil {
				fmt.Printf("%s\n", err)
				return
			}
			// rtm.SendMessage(rtm.NewOutgoingMessage(fmt.Sprintf("Got %v from %v", text, user.Name), ev.Msg.Channel))
			toPrint := tokeniseAndDispatchInput(&fryatogParams{slackm: text}, getScryfallCard, getRandomScryfallCard)
			for _, s := range sliceUniqMap(toPrint) {
				rtm.SendMessage(rtm.NewOutgoingMessage(fmt.Sprintf("<@%v>: %v", user.ID, s), ev.Msg.Channel))
			}

		case *slack.PresenceChangeEvent:
			// fmt.Printf("Presence Change: %v\n", ev)

		case *slack.LatencyReport:
			fmt.Printf("Current latency: %v\n", ev.Value)

		case *slack.RTMError:
			fmt.Printf("Error: %s\n", ev.Error())

		case *slack.InvalidAuthEvent:
			fmt.Printf("Invalid credentials")
			return

		default:
			// Ignore other events..
			// fmt.Printf("Unexpected: %v\n", msg.Data)
		}
	}
}
