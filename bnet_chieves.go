package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/FuzzyStatic/blizzard/wowgd"
	"github.com/FuzzyStatic/blizzard/wowp"
	raven "github.com/getsentry/raven-go"
	log "gopkg.in/inconshreveable/log15.v2"
)

//TODO: wow
//TODO: Multi-rank "Got My Mind On My Money"
//TODO: For chieve, fuzzy. Fuzzy for realm too.

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
			log.Debug("playerSingleChieveStatus: Found chieve", "Chievo", a)
			var ret []string
			if a.Criteria.IsCompleted {
				ret = append(ret, ":fire: Achievement Unlocked! :fire:")
			} else {
				ret = append(ret, ":cry: Chievo not got :cry:")
			}
			/* SubChieves */
			ac := chieveFromID(a.Achievement.ID)
			accc := mapCriteriaToName(ac.Criteria.ChildCriteria)
			// A bare chievo with a single child criterion
			if len(ac.Criteria.ChildCriteria) == 1 && len(ac.Criteria.ChildCriteria[0].ChildCriteria) == 0 {
				accc = singleBareChievoCriterion(ac)
			}
			log.Debug("Retrieved Chieve", "C", ac, "Map", accc)
			if len(a.Criteria.ChildCriteria) > 0 {
				ccret := recursePlayerCriteria(a.Criteria.ChildCriteria)
				log.Debug("Looping CCs", "Map", ccret)
				for k, v := range ccret {
					if ad, ok := accc[k]; ok {
						if strings.Contains(ad, "%s") {
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

/* CHIEVE UTILITIES */
type playerCriteriaStuff struct {
	completed bool
	amount    string
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

func handleChieveInput(input string) string {
	cardTokens := strings.SplitN(input, " ", 3)
	log.Debug("Handling Chieve Input", "Input", input, "Tokens", cardTokens)
	realm, player, err := distinguishRealmFromPlayer(cardTokens[0], cardTokens[1])
	if err != nil {
		return formatChieveForSlack(chieveFromID(chieveNameToID(input)))
	}
	return chieveForPlayer(realm, player, cardTokens[2])

}

func chieveNameToID(chieveName string) int {
	if bNetClient == nil || len(wowChieves.Achievements) == 0 {
		log.Debug("Chieve Name to ID - no client or chieves")
		return 0
	}

	for _, a := range wowChieves.Achievements {
		if strings.ToLower(a.Name) == strings.ToLower(chieveName) {
			log.Debug("Chieve Name to ID", "Name", chieveName, "Chievo Found", a.ID)
			return a.ID
		}
	}

	log.Debug("Chieve Name to ID -- not found")
	// Not found
	return 0
}

// Little wrapper to make the format function hermetic.
func chieveFromID(chieveID int) *wowgd.Achievement {
	log.Debug("Chieve from ID", "ID", chieveID)
	if chieveID == 0 {
		return nil
	}
	c, _, err := bNetClient.WoWAchievement(chieveID)
	if err != nil {
		raven.CaptureError(err, nil)
		return nil
	}
	log.Debug("Chieve from ID", "ID", chieveID, "Chievo Found", c.Name)
	return c
}

func mapCriteriaToStrings(cc wowgd.ChildCriteria) []string {
	var ret []string
	for _, c := range cc {
		if c.Achievement.ID == 0 {
			if len(c.ChildCriteria) == 0 {
				continue
			}
			if c.Operator.Name != "" && len(c.ChildCriteria) > 1 {
				if c.Amount > 0 {
					ret = append(ret, fmt.Sprintf("%s %d of:", c.Operator.Name, c.Amount))
				} else {
					ret = append(ret, fmt.Sprintf("%s of:", c.Operator.Name))
				}
			}
		} else {
			tryChieve := chieveFromID(c.Achievement.ID)
			if tryChieve != nil && tryChieve.ID != 0 {
				var faction string
				if len(c.Faction.Name) > 1 {
					faction = fmt.Sprintf(" [%s]", string(c.Faction.Name[0]))
				}
				ret = append(ret, fmt.Sprintf("<http://www.wowhead.com/achievement=%d|%s>%s - %s", tryChieve.ID, tryChieve.Name, faction, tryChieve.Description))
			}
		}
		if len(c.ChildCriteria) > 0 {
			ret = append(ret, mapCriteriaToStrings(c.ChildCriteria)...)
		}
	}
	return ret
}

func singleBareChievoCriterion(c *wowgd.Achievement) map[int]string {
	log.Debug("SBCC")
	ret := make(map[int]string)
	ret[c.Criteria.ChildCriteria[0].ID] = fmt.Sprintf("<http://www.wowhead.com/achievement=%d|%s> - %s", c.ID, c.Name, c.Description)
	if c.Criteria.Amount > 1 {
		ret[c.Criteria.ChildCriteria[0].ID] = ret[c.Criteria.ChildCriteria[0].ID] + fmt.Sprintf(" [%%s/%d]", c.Criteria.Amount)
	}
	return ret
}

func mapCriteriaToName(cc wowgd.ChildCriteria) map[int]string {
	log.Debug("Recursing into Mapping Criteria to Name")
	ret := make(map[int]string)
	for _, c := range cc {
		if c.Achievement.ID == 0 {
			continue
		}
		tryChieve := chieveFromID(c.Achievement.ID)
		if tryChieve != nil && tryChieve.ID != 0 {
			ret[c.ID] = fmt.Sprintf("<http://www.wowhead.com/achievement=%d|%s> - %s", tryChieve.ID, tryChieve.Name, tryChieve.Description)
			if c.Amount > 1 {
				ret[c.ID] = ret[c.ID] + fmt.Sprintf(" [%%s/%d]", c.Amount)
			}
		}
		if len(c.ChildCriteria) > 0 {
			ret = mergeIntStringMaps(mapCriteriaToName(c.ChildCriteria), ret)
		}
	}
	log.Debug("Recursing into Mapping Criteria to Name", "Ret", ret)
	return ret
}

func recursePlayerCriteria(cc wowp.ChildCriteria) map[int]playerCriteriaStuff {
	log.Debug("Recursing into Player Criteria")
	ret := make(map[int]playerCriteriaStuff)
	for _, c := range cc {
		ret[c.ID] = playerCriteriaStuff{c.IsCompleted, strconv.FormatInt(int64(c.Amount), 10)}
		if len(c.ChildCriteria) > 0 {
			ret = mergeIntStuffMaps(recursePlayerCriteria(c.ChildCriteria), ret)
		}
	}
	log.Debug("Recursing into Player Criteria", "Ret", ret)
	return ret
}

func mergeIntStuffMaps(new map[int]playerCriteriaStuff, existing map[int]playerCriteriaStuff) map[int]playerCriteriaStuff {
	for k, v := range new {
		existing[k] = v
	}
	return existing
}
