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

/* REALM UTILITIES */

// Given two strings, if either one is the name of a realm (case insensitive)
// returns realm, player (, error).
func distinguishRealmFromPlayer(input1, input2 string) (string, string, error) {
	for _, r := range wowRealms.Realms {
		if strings.ToLower(input1) == strings.ToLower(r.Name) {
			return r.Slug, input2, nil
		}
		if strings.ToLower(input2) == strings.ToLower(r.Name) {
			return r.Slug, input1, nil
		}
	}
	return "", "", fmt.Errorf("Realm not found")
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
