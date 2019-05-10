package main

import (
	"fmt"
	"strings"

	"github.com/algolia/algoliasearch-client-go/algolia/search"
)

func formatHSCard(i map[string]interface{}) string {
	var r []string
	if i["hearthpwnUrl"] != "" {
		r = append(r, fmt.Sprintf("*<%v|%v>*", i["hearthpwnUrl"], i["name"]))
	} else {
		r = append(r, fmt.Sprintf("*%v*", i["name"]))
	}
	r = append(r, fmt.Sprintf("· {%v} ·", i["cost"]))
	r = append(r, fmt.Sprintf("%v ·", i["type"]))
	if i["type"] == "Minion" {
		r = append(r, fmt.Sprintf("%v/%v ·", i["attack"], i["health"]))
	} else if i["type"] == "Weapon" {
		r = append(r, fmt.Sprintf("%v/%v ·", i["attack"], i["durability"]))
	}
	if i["text"] != nil {
		text, ok := i["text"].(string)
		if ok {
			modifiedRuleText := strings.Replace(text, "<b>", "*", -1)
			modifiedRuleText = strings.Replace(modifiedRuleText, "</b>", "*", -1)
			modifiedRuleText = strings.Replace(modifiedRuleText, "<i>", "_", -1)
			modifiedRuleText = strings.Replace(modifiedRuleText, "</i>", "_", -1)
			modifiedRuleText = strings.Replace(modifiedRuleText, "\n", " ", -1)
			r = append(r, fmt.Sprintf("%v ·", modifiedRuleText))
		}
	}
	r = append(r, fmt.Sprintf("_%v_ ·", i["flavor"]))
	r = append(r, fmt.Sprintf("%v-%v", i["set"], (i["rarity"]).(string)[0:1]))
	return strings.Join(r, " ")
}

func searchHSCard(input string) (string, error) {
	client := search.NewClient(conf.Hearthstone.AppID, conf.Hearthstone.APIToken)
	index := client.InitIndex(conf.Hearthstone.IndexName)
	res, err := index.Search(input, nil)
	if err != nil {
		return "", err
	}
	if len(res.Hits) > 0 {
		return formatHSCard(res.Hits[0]), nil
	}
	return "", fmt.Errorf("Card not found")
}
