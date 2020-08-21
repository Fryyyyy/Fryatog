package main

import (
	"fmt"
	"strings"

	"github.com/FuzzyStatic/blizzard/wowgd"
	"github.com/FuzzyStatic/blizzard/wowp"
	raven "github.com/getsentry/raven-go"
	log "gopkg.in/inconshreveable/log15.v2"
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
	log.Debug("Handling Chieve Player", "Realm", realm, "Player", player, "ChieveName", chieveName)
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
	log.Debug("Handling Chieve Player Status")
	for _, a := range cas.Achievements {
		if strings.ToLower(a.Achievement.Name) == strings.ToLower(chieveName) {
			log.Debug("playerSingleChieveStatus: Found chieve")
			var ret []string
			if a.Criteria.IsCompleted {
				ret = append(ret, ":fire: Achievement Unlocked! :fire:")
			} else {
				ret = append(ret, ":cry: Chievo not got :cry:")
			}
			/* SubChieves */
			ac := chieveFromID(a.Achievement.ID)
			accc := mapCriteriaToName(ac.Criteria.ChildCriteria)
			log.Debug("Retrieved Chieve", "Map", accc)
			if len(a.Criteria.ChildCriteria) > 0 {
				ccret := recursePlayerCriteria(a.Criteria.ChildCriteria)
				log.Debug("Looping CCs", "Map", ccret)
				for k, v := range ccret {
					if ad, ok := accc[k]; ok {
						if strings.Contains("%s", ad) {
							ad = fmt.Sprintf(ad, v.amount)
						}
						if v.completed {
							ret = append(ret, "[:white_check_mark:] "+ad)
						} else {
							ret = append(ret, "[:x:] "+ad)
						}
					}
				}
			}
			return strings.Join(ret, "\n")
		}
	}
	log.Debug("playerSingleChieveStatus: Not found")
	return "Chieve not found :("
}

func formatChieveForSlack(a *wowgd.Achievement) string {
	if a == nil {
		return "Chieve not found :("
	}
	if len(a.Criteria.ChildCriteria) < 2 {
		return fmt.Sprintf("<http://www.wowhead.com/achievement=%d|%s> - %s [:point_right: %d :point_left:]", a.ID, a.Name, a.Description, a.Points)
	}
	var ret []string
	ret = append(ret, fmt.Sprintf("%s - %s\n", a.Name, a.Description))
	for _, v := range mapCriteriaToStrings(a.Criteria.ChildCriteria) {
		ret = append(ret, v)
	}
	if len(a.RewardDescription) > 0 {
		ret = append(ret, fmt.Sprintf(":trophy: %s :trophy:", a.RewardDescription))
	}
	return ret[0] + strings.Join(ret[1:], "\n")
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
