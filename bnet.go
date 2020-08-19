package main

import (
	"fmt"
	"strings"

	"github.com/FuzzyStatic/blizzard/wowgd"
	"github.com/FuzzyStatic/blizzard/wowp"
	raven "github.com/getsentry/raven-go"
)

//TODO: wowchar
//TODO: wow

func retrieveChievesForPlayer(realm, player string) (*wowp.CharacterAchievementsSummary, error) {
	if c, ok := wowPlayerChieveCache.Get(realm + "-" + player); ok {
		return c.(*wowp.CharacterAchievementsSummary), nil
	}
	cas, _, err := bNetClient.WoWCharacterAchievementsSummary(realm, player)
	if err != nil {
		return nil, err
	}
	wowPlayerChieveCache.SetDefault(realm+"-"+player, cas)
	return cas, nil
}

func chieveForPlayer(realm, player, chieveName string) string {
	if bNetClient == nil {
		return "WOW API not available"
	}
	cas, err := retrieveChievesForPlayer(realm, player)
	if err != nil {
		raven.CaptureError(err, nil)
		return "Could not retrieve Chieves for Player"
	}
	return playerSingleChieveStatus(cas, chieveName)
}

// Parses the chieves of a player and returns a Slack formatted string
// as to whether they have received it or not.
func playerSingleChieveStatus(cas *wowp.CharacterAchievementsSummary, chieveName string) string {
	for _, a := range cas.Achievements {
		if a.Achievement.Name == chieveName {
			var ret []string
			if a.Criteria.IsCompleted {
				ret = append(ret, ":fire: Achievement Unlocked! :fire:")
			} else {
				ret = append(ret, ":cry: Chievo not got :cry:")
			}
			/* TODO: SubChieves */
			if len(a.Criteria.ChildCriteria) > 1 {
				ret = append(ret, "Criteria goes here")
			}
			return strings.Join(ret, "\n")
		}
	}
	return "Chieve not found :("
}

func formatChieveForSlack(a *wowgd.Achievement) string {
	if a == nil {
		return "Chieve not found :("
	}
	if len(a.Criteria.ChildCriteria) < 2 {
		return fmt.Sprintf("<http://www.wowhead.com/achievement=%d|%s> - %s", a.ID, a.Name, a.Description)
	}
	var ret []string
	ret = append(ret, fmt.Sprintf("%s - %s\n", a.Name, a.Description))
	for _, c := range dedupeCriteria(a.Criteria.ChildCriteria) {
		tryChieve := chieveFromID(c.ID)
		if tryChieve != nil && tryChieve.ID != 0 {
			ret = append(ret, fmt.Sprintf("<http://www.wowhead.com/achievement=%d|%s> %s", tryChieve.ID, tryChieve.Name, tryChieve.Description))
		} else {
			ret = append(ret, "Unknown Criterion")
		}
	}
	return ret[0] + strings.Join(ret[1:], ", ")
}

type wowDude struct {
	cps *wowp.CharacterProfileSummary
	css *wowp.CharacterStatisticsSummary
	ces *wowp.CharacterEquipmentSummary
}

/*
func retrieveDude(player, realm string) (wowDude, error) {
	if c, ok := wowPlayerCache.Get(realm + "-" + player); ok {
		return c.(wowDude), nil
	}
	aps, _, err := bNetClient.WoWCharacterProfileSummary(realm, player)
	if err != nil {
		raven.CaptureError(err, nil)
		return nil, err
	}
	return aps, nil
}

func getWoWDude(input1, input2 string) string {
	if bNetClient == nil {
		return "WOW API not available"
	}
	realm, player, err := distinguishRealmFromPlayer(input1, input2)
	if err != nil {
		return "Could not distinguish realm"
	}
	aps, err := retrieveDude(player, realm)
	if err != nil {
		return "Problem retrieving player"
	}

}
*/
