package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	raven "github.com/getsentry/raven-go"
	log "gopkg.in/inconshreveable/log15.v2"
)

const snapAPI = "https://marvelsnap.io/api/search.php?database&n=%s&desc=%s&sort=name&limit=1&offset=0"

type SnapCard struct {
	ID        int     `json:"id"`
	Name      string  `json:"name"`
	Type      string  `json:"type"`
	Cost      *int    `json:"cost"`
	Power     *int    `json:"power"`
	Ability   string  `json:"ability"`
	DateAdded string  `json:"date_added"`
	Status    *string `json:"status"`
	Variants  string  `json:"variants"`
	PrettyURL string  `json:"pretty_url"`
	Method    *string `json:"method"`
	Slug      string  `json:"slug"`
}

type SnapResponse struct {
	Card   []SnapCard `json:"card"`
	Paging struct {
		TotalItems int `json:"total_items"`
		TotalPages int `json:"total_pages"`
	} `json:"paging"`
}

func handleSnapQuery(cardTokens []string) string {
	for _, rc := range reduceCardSentence(cardTokens) {
		card, err := searchSnapCard(rc)
		log.Debug("Snap Card Func gave us", "CardID", card, "Err", err)
		if err == nil {
			return card
		}
	}
	return ""
}

func formatSnapCard(c SnapCard) string {
	var r []string
	r = append(r, fmt.Sprintf("*<https://marvelsnap.io/card/%v|%v>* -", c.PrettyURL, c.Name))
	if c.Cost != nil {
		r = append(r, replaceManaCostForSlack(fmt.Sprintf(" {%d}", *c.Cost)))
	}
	if c.Power != nil {
		r = append(r, fmt.Sprintf(" · ⚔️ %v ⚔️ ·", *c.Power))
	}
	r = append(r, fmt.Sprintf(" %v", c.Ability))
	if c.Method != nil {
		r = append(r, fmt.Sprintf(" · (_%v_)", *c.Method))
	}

	return strings.Join(r, "")
}

func searchSnapCard(input string) (string, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf(snapAPI, input, input), nil)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Warn("searchSnapCard: The HTTP request could not be created", "Error", err)
		return "", err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Warn("searchSnapCard: The HTTP request failed", "Error", err)
		return "", err
	}
	defer resp.Body.Close()

	var sc SnapResponse
	if resp.StatusCode == 200 {
		if err := json.NewDecoder(resp.Body).Decode(&sc); err != nil {
			raven.CaptureError(err, nil)
			return "", err
		}
		if len(sc.Card) == 1 {
			return formatSnapCard(sc.Card[0]), nil
		}
	}
	return "", fmt.Errorf("Card not found")
}
