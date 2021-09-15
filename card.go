package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	raven "github.com/getsentry/raven-go"
	log "gopkg.in/inconshreveable/log15.v2"
)

const namesFile = "names.json"
const pointsFile = "points.txt"
const scryfallNamesAPIURL = "https://api.scryfall.com/catalog/card-names"
const scryfallFuzzyAPIURL = "https://api.scryfall.com/cards/named?fuzzy=%s"
const scryfallRandomAPIURL = "https://api.scryfall.com/cards/random"
const scryfallSearchAPIURL = "https://api.scryfall.com/cards/search"
const highlanderPointsURL = "http://decklist.mtgpairings.info/js/cards/highlander.txt"

// TODO: Also CardFaces
func (card *Card) getExtraMetadata(inputURL string) {
	log.Debug("Getting Metadata")
	// This is called even for empty Card objects, do don't do anything in that case
	if card.ID == "" {
		return
	}
	fetchURL := card.PrintsSearchURI
	var cm CardMetadata
	// Already have metadata?
	if !reflect.DeepEqual(card.Metadata, cm) {
		return
	}
	// Use parameter over stored, for recursive lists
	if inputURL != "" {
		fetchURL = inputURL
	}
	// Have a url?
	if fetchURL == "" {
		return
	}
	metadataRequests.Add(1)
	log.Debug("GetExtraMetadata: Attempting to fetch", "URL", fetchURL)
	resp, err := http.Get(fetchURL)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Warn("GetExtraMetadata: The HTTP request failed", "Error", err)
		return
	}
	defer resp.Body.Close()
	var list CardList
	if resp.StatusCode == 200 {
		if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
			raven.CaptureError(err, nil)
			return
		}
		if len(list.Warnings) > 0 {
			return
		}
		if list.HasMore {
			defer card.getExtraMetadata(list.NextPage)
		}
		// These are in printing order, since the prints_search_uri includes "order=released"
		for _, c := range list.Data {
			if c.ID == card.ID {
				continue
			}
			if c.FlavourText != "" {
				cm.PreviousFlavourTexts = append(cm.PreviousFlavourTexts, c.FlavourText)
			}
			cm.PreviousPrintings = append(cm.PreviousPrintings, c.formatExpansions())

			if card.getReminderTexts() == "Reminder text not found" && c.getReminderTexts() != "Reminder text not found" {
				cm.PreviousReminderTexts = append(cm.PreviousReminderTexts, c.getReminderTexts())
			}
		}
		card.Metadata = cm
		// Update the Cache ???? Necessary ?
		nameToCardCache.Add(normaliseCardName(card.Name), *card)
		// log.Debug("After metadata extraction", "Card", card)
		return
	}
	log.Info("GetExtraMetadata: Scryfall returned a non-200", "Status Code", resp.StatusCode)
}

// Get all possible most recent reminder texts for a card, \n separated
// TODO/NOTE: This doesn't work, since Scryfall doesn't actually give the printed_text field for each previous printing,
// just the current Oracle text.
func (card Card) getReminderTexts() string {
	reminderRequests.Add(1)
	cardText := card.OracleText

	if len(card.CardFaces) > 0 {
		cardText = ""
		for _, cf := range card.CardFaces {
			cardText += cf.OracleText
		}

	}
	reminders := reminderRegexp.FindAllStringSubmatch(cardText, -1)
	if len(reminders) == 0 {
		if len(card.Metadata.PreviousReminderTexts) > 0 {
			return card.Metadata.PreviousReminderTexts[0]
		}
		return "Reminder text not found"
	}
	var ret []string
	for _, m := range reminders {
		ret = append(ret, m[1])
	}
	return strings.Join(ret, "\n")
}

// Get the most recent Flavour Text that exists
func (card *Card) getFlavourText() string {
	flavourRequests.Add(1)
	if card.FlavourText != "" {
		return card.FlavourText
	}
	if len(card.Metadata.PreviousFlavourTexts) > 0 {
		return card.Metadata.PreviousFlavourTexts[0]
	}
	return "Flavour text not found"
}

func (cc CommonCard) getCardOrFaceAsString(mode string) []string {
	slack := (mode == "slack")
	irc := (mode == "irc")
	if !slack && !irc {
		log.Warn("In GCOFAS -- Unknown mode", "Mode", mode)
		return []string{"UNKNOWN MODE"}
	}
	var r []string
	// Mana Cost
	if cc.ManaCost != "" {
		if slack {
			r = append(r, replaceManaCostForSlack(cc.ManaCost))
		} else if irc {
			r = append(r, formatManaCost(cc.ManaCost))
		}
	}
	// Type Line
	r = append(r, fmt.Sprintf("· %s ·", nco(cc.PrintedTypeLine, cc.TypeLine)))
	// P/T
	if cc.Power != "" {
		if slack {
			r = append(r, fmt.Sprintf("%s/%s ·", strings.Replace(cc.Power, "*", "\xC2\xAD*", -1), strings.Replace(cc.Toughness, "*", "\xC2\xAD*", -1)))
		} else if irc {
			r = append(r, fmt.Sprintf("%s/%s ·", cc.Power, cc.Toughness))
		}
	}
	// CIs
	if len(cc.ColorIndicators) > 0 {
		r = append(r, fmt.Sprintf("%s ·", standardiseColorIndicator(cc.ColorIndicators)))
	}
	// Loyalty
	if cc.Loyalty != "" {
		r = append(r, fmt.Sprintf("[%s]", cc.Loyalty))
	}
	// OracleText
	if cc.OracleText != "" {
		var modifiedOracleText string
		if slack {
			modifiedOracleText = strings.Replace(replaceManaCostForSlack(nco(cc.PrintedText, cc.OracleText)), "\n", " \\ ", -1)
			modifiedOracleText = strings.Replace(modifiedOracleText, "(", "_(", -1)
			modifiedOracleText = strings.Replace(modifiedOracleText, ")", ")_", -1)
			modifiedOracleText = strings.Replace(modifiedOracleText, "*", "\xC2\xAD*", -1)
		} else if irc {
			modifiedOracleText = strings.Replace(nco(cc.PrintedText, cc.OracleText), "\n", " \\ ", -1)
			// Change the open/closing parens of reminder text to also start and end italics
			modifiedOracleText = strings.Replace(modifiedOracleText, "(", "\x1D(", -1)
			modifiedOracleText = strings.Replace(modifiedOracleText, ")", ")\x0F", -1)
		}
		r = append(r, modifiedOracleText)
	}
	return r
}

func (card *Card) formatCardForSlack() string {
	var s []string
	if len(card.CardFaces) > 0 {
		for _, cf := range card.CardFaces {
			var r []string
			if len(card.MultiverseIds) == 0 {
				r = append(r, fmt.Sprintf("*<%s|%s>*", card.ScryfallURI, nco(cf.PrintedName, cf.Name)))
			} else {
				r = append(r, fmt.Sprintf("*<http://gatherer.wizards.com/Pages/Card/Details.aspx?multiverseid=%v|%v>*", card.MultiverseIds[0], nco(cf.PrintedName, cf.Name)))
			}
			r = append(r, cf.CommonCard.getCardOrFaceAsString("slack")...)
			s = append(s, strings.Join(r, " "))
		}
		return strings.Join(s, "\n")
	}

	if len(card.MultiverseIds) == 0 {
		s = append(s, fmt.Sprintf("*<%s|%s>*", card.ScryfallURI, nco(card.PrintedName, card.Name)))
	} else {
		s = append(s, fmt.Sprintf("*<http://gatherer.wizards.com/Pages/Card/Details.aspx?multiverseid=%v|%v>*", card.MultiverseIds[0], nco(card.PrintedName, card.Name)))
	}
	s = append(s, card.CommonCard.getCardOrFaceAsString("slack")...)
	if card.Reserved {
		s = append(s, "· [RL] ·")
	}
	if points, ok := highlanderPoints[normaliseCardName(card.Name)]; ok {
		s = append(s, fmt.Sprintf("[:point_right: %d :point_left:]", points))
	}
	return strings.Join(s, " ")
}

func (card *Card) formatCardForIRC() string {
	var s []string
	if len(card.CardFaces) > 0 {
		// DFC and Flip and Split - produce two cards
		for _, cf := range card.CardFaces {
			var r []string
			// Bold card name
			r = append(r, fmt.Sprintf("\x02%s\x0F", nco(cf.PrintedName, cf.Name)))
			r = append(r, cf.CommonCard.getCardOrFaceAsString("irc")...)
			if cf.ManaCost != "" {
				r = append(r, fmt.Sprintf("· %s ·", card.formatExpansions()))
				r = append(r, card.formatLegalities())
			}
			s = append(s, strings.Join(r, " "))
		}
		return strings.Join(s, "\n")
	}

	// Normal card
	s = append(s, fmt.Sprintf("\x02%s\x0F", nco(card.PrintedName, card.Name)))
	s = append(s, card.CommonCard.getCardOrFaceAsString("irc")...)
	s = append(s, fmt.Sprintf("· %s ·", card.formatExpansions()))
	if card.Reserved {
		s = append(s, "[RL] ·")
	}
	s = append(s, card.formatLegalities())

	return strings.Join(s, " ")
}

func (card *Card) getRulings(rulingNumber int) string {
	rulingRequests.Add(1)
	// Do we already have the Rulings?
	if card.Rulings == nil {
		// If we don't, fetch them
		err := (card).fetchRulings()
		if err != nil {
			return "Problem fetching the rulings"
		}
		// Update the Cache ???? Necessary ?
		nameToCardCache.Add(normaliseCardName(card.Name), *card)
	}
	// Now we have them
	var ret []string
	i := 0
	for _, r := range card.Rulings {
		if r.Source == "wotc" {
			i++
			// Do we want a specific ruling?
			if rulingNumber > 0 && i == rulingNumber {
				return r.formatRuling()
			}
			ret = append(ret, r.formatRuling())
		}
	}
	if rulingNumber > 0 || len(ret) == 0 {
		return "Ruling not found"
	}
	if len(ret) > 3 {
		return fmt.Sprintf("Too many rulings (%d), please request a specific one", len(ret))
	}
	return strings.Join(ret, "\n")
}

func (card *Card) fetchRulings() error {
	log.Debug("FetchRulings: Attempting to fetch", "URL", card.RulingsURI)
	resp, err := http.Get(card.RulingsURI)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Warn("FetchRulings: The HTTP request failed", "Error", err)
		return fmt.Errorf("Something went wrong fetching the rulings")
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		var crr CardRulingResult
		if err := json.NewDecoder(resp.Body).Decode(&crr); err != nil {
			raven.CaptureError(err, nil)
			return fmt.Errorf("Something went wrong parsing the rulings")
		}

		card.Rulings = crr.Data
		err = card.sortRulings()
		if err != nil {
			return fmt.Errorf("Error sorting rules")
		}
		return nil
	}

	log.Info("FetchRulings: Scryfall returned a non-200", "Status Code", resp.StatusCode)
	return fmt.Errorf("Unable to fetch rulings from Scryfall")
}

func (card *Card) sortRulings() error {
	if len(card.Rulings) == 0 {
		return nil
	}
	var sortedRulings []CardRuling
	var rulingsChunk []CardRuling

	currentGroupedDate, err := time.Parse("2006-01-02", card.Rulings[0].PublishedAt)
	if err != nil {
		log.Error("Failed to parse date")
		return err
	}

	// Special case for Mairsil, the Pretender
	// Scryfall doesn't acknowledge Gatherer ruling order as canonical,
	// so their "ruling 11" is Gatherer's "ruling 1." Yes, I hate this.
	if card.Name == "Mairsil, the Pretender" {
		// Since append requires a slice as its first arg, so we can't just do card.Rulings[11]
		// We can't use negative indices in Go like we can in Python, so we get a slice that's
		// just the last object in the list.
		card.Rulings = append(card.Rulings[len(card.Rulings)-1:], card.Rulings...)
		// Get a slice that drops the last ruling, since we just moved it to the front of the list
		// and we don't want it duplicated.
		card.Rulings = card.Rulings[:len(card.Rulings)-1]
		return nil
	}

	for _, ruling := range card.Rulings {
		rulingDate, err := time.Parse("2006-01-02", ruling.PublishedAt)
		if err != nil {
			log.Error("Failed to parse date")
			return err
		}
		if rulingDate.Before(currentGroupedDate) {
			currentGroupedDate = rulingDate
			sortedRulings = append(rulingsChunk, sortedRulings...)
			rulingsChunk = nil
		}

		rulingsChunk = append(rulingsChunk, ruling)
	}
	sortedRulings = append(rulingsChunk, sortedRulings...)
	card.Rulings = sortedRulings
	return nil
}

func (card *Card) cardGetLang(lang string) (Card, error) {
	log.Debug("Getting Card Languages")
	var c Card
	// This is called even for empty Card objects, do don't do anything in that case
	if card.ID == "" {
		return c, fmt.Errorf("Empty card")
	}
	fetchURL := card.PrintsSearchURI + "&include_multilingual=true"
	// Have a url?
	if fetchURL == "" {
		return c, fmt.Errorf("No URL")
	}
	log.Debug("cardGetLang: Attempting to fetch", "URL", fetchURL)
	resp, err := http.Get(fetchURL)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Warn("cardGetLang: The HTTP request failed", "Error", err)
		return c, fmt.Errorf("Error fetching results")
	}
	defer resp.Body.Close()
	var list CardList
	if resp.StatusCode == 200 {
		if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
			raven.CaptureError(err, nil)
			return c, fmt.Errorf("Error decoding results")
		}
		if len(list.Warnings) > 0 {
			return c, fmt.Errorf("Warnings in response")
		}
		for _, cs := range list.Data {
			if cs.Lang == lang {
				return cs, nil
			}
		}
	}
	return c, fmt.Errorf("Language not found")
}

func fetchScryfallCardByFuzzyName(input string, isLang bool) (Card, error) {
	var emptyCard Card
	url := fmt.Sprintf(scryfallFuzzyAPIURL, url.QueryEscape(input))
	log.Debug("fetchScryfallCard: Attempting to fetch", "URL", url)
	resp, err := http.Get(url)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Warn("fetchScryfallCard: The HTTP request failed", "Error", err)
		return emptyCard, fmt.Errorf("Something went wrong fetching the card")
	}
	defer resp.Body.Close()
	var card Card
	if resp.StatusCode == 200 {
		if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
			raven.CaptureError(err, nil)
			return card, fmt.Errorf("Something went wrong parsing the card")
		}
		if !isLang && card.Lang != "en" {
			log.Debug("Got back a foreign card when it wasn't requested, let's try again")
			coercedName, err := HandleForeignCardOverlapCases(input)
			if err == nil && coercedName != "" {
				card.Name = coercedName
			}
			return fetchScryfallCardByFuzzyName(card.Name, false)
		}
		if IsDumbCard(card) {
			return emptyCard, fmt.Errorf("Dumb card returned, keep trying")
		}
		return card, nil
	}
	log.Info("fetchScryfallCard: Scryfall returned a non-200", "Status Code", resp.StatusCode)
	return card, fmt.Errorf("Card not found by Scryfall")
}

func HandleForeignCardOverlapCases(input string) (string, error) {
	log.Debug("Handling foreign card overlap", "Card", input)
	if uroRegex.MatchString(input) {
		log.Debug("\tThey probably wanted Uro, Titan of Nature's Wrath")
		return "Uro, Titan of Nature's Wrath", nil
	}
	return "", nil
}

func IsDumbCard(card Card) bool {
	return (card.BorderColor != "black" && card.BorderColor != "white" && card.BorderColor != "borderless") || 
	strings.Contains(card.Layout, "vanguard") || 
	(strings.Contains(card.Layout, "token") && !(strings.Contains(card.TypeLine, "Dungeon"))) || 
	strings.Contains(card.Layout, "art_series") || 
	card.SetType == "funny" || 
	card.Set == "fjmp" ||
	strings.Contains(card.Set, "thp")
}

func fetchDumbScryfallCardByName(input string, isLang bool) (Card, error) {
	var emptyCard Card
	u, _ := url.Parse(scryfallSearchAPIURL)
	q := u.Query()
	q.Add("include_extras", "true")
	queryString := "(border:silver or is:funny or t:scheme or t:vanguard or t:plane or t:phenomenon) " + input
	q.Add("q", queryString)
	u.RawQuery = q.Encode()
	log.Debug("searchScryfallCard: Attempting to fetch", "URL", u)
	resp, err := http.Get(u.String())
	if err != nil {
		raven.CaptureError(err, nil)
		log.Warn("searchDumbScryfallCard: The HTTP request failed", "Error", err)
		return emptyCard, fmt.Errorf("Something went wrong fetching card search results")
	}
	defer resp.Body.Close()
	var csr CardSearchResult
	if resp.StatusCode == 200 {
		if err := json.NewDecoder(resp.Body).Decode(&csr); err != nil {
			raven.CaptureError(err, nil)
			return emptyCard, fmt.Errorf("Something went wrong parsing the card search results")
		}
		log.Debug("searchDumbScryfallCard", "Total cards found", csr.TotalCards)
		if csr.TotalCards != 1 {
			return emptyCard, fmt.Errorf("Too many cards returned (%v > 5)", csr.TotalCards)
		}
		return csr.Data[0], nil
	}
	log.Error("searchDumbScryfallCard: Scryfall returned a non-200", "Status Code", resp.StatusCode)
	return emptyCard, fmt.Errorf("No cards found")
}

func checkCacheForCard(ncn string) (Card, error) {
	log.Debug("Checking cache for card", "Name", ncn)
	cardCacheQueries.Add(1)
	var emptyCard Card
	if cacheCard, found := nameToCardCache.Get(ncn); found {
		log.Debug("Card was cached")
		cardCacheHits.Add(1)
		cardCacheHitPercentage.Set(int64(math.Round((float64(cardCacheHits.Value()) / float64(cardCacheQueries.Value())) * 100)))
		if cacheCard == nil || reflect.DeepEqual(cacheCard, emptyCard) {
			log.Debug("But cached as nothing")
			return emptyCard, fmt.Errorf("Card not found")
		}
		// Check to see we're returning the canonical card object
		c := cacheCard.(Card)
		cNCN := normaliseCardName(c.Name)
		// It already was the card we wanted.
		if ncn == cNCN {
			log.Debug("It was the Canonical Object")
			return c, nil
		}
		// Do we have the canonical object?
		if cc2, found := nameToCardCache.Get(cNCN); found {
			log.Debug("We have the Canonical Object")
			return cc2.(Card), nil
		}
		// We don't, return what we got
		log.Debug("We don't have the Canonical Object")
		return c, nil
	}
	cardCacheHitPercentage.Set((cardCacheHits.Value() / cardCacheQueries.Value()) * 100)
	log.Debug("Not in cache")
	return emptyCard, fmt.Errorf("Card not found in cache")
}

func getCachedOrStoreCard(card *Card, ncn string) (Card, error) {
	log.Debug("In GCOSC", "Card Name", card.Name, "ncn", ncn)
	cNcn := normaliseCardName(card.Name)
	// If it's a foreign card, store the foreign name
	if card.PrintedName != "" {
		cNcn = normaliseCardName(card.PrintedName)
	}

	card.getExtraMetadata("")
	// Remember what they typed
	nameToCardCache.Add(ncn, *card)

	// What they typed was the real card name, so we're done.
	if ncn == cNcn {
		return *card, nil
	}

	// Else, check to see if we have the real card
	cc, err := checkCacheForCard(cNcn)
	if err == nil {
		// Return the canonical cached object
		log.Debug("Returning existing Canonical object")
		return cc, err
	}

	// We didn't, so store the canonical object if it's not an alternate printing
	if card.PrintedName == "" {
		log.Debug("Storing new Canonical object")
		nameToCardCache.Add(cNcn, *card)
	}
	return *card, nil
}

func getScryfallCard(input string, isLang bool) (Card, error) {
	cardRequests.Add(1)
	var card Card

	// Normalise input to match how we store in the cache:
	// lowercase, no punctuation.
	ncn := normaliseCardName(input)

	// See if it's a known short name of a card.
	if qualifiedName, ok := shortCardNames[ncn]; ok {
		input = qualifiedName
		log.Debug("search for", input, "got", qualifiedName)
	}

	log.Debug("Asked for card", "Name", ncn)
	card, err := checkCacheForCard(ncn)
	if err == nil || err.Error() == "Card not found" {
		return card, err
	}

	log.Debug("Checking Scryfall for card", "Name", ncn)
	// Try fuzzily matching the name
	card, err = fetchScryfallCardByFuzzyName(input, isLang)

	if err == nil {
		return getCachedOrStoreCard(&card, ncn)
	}
	// No luck - try unique prefix
	cardName := lookupUniqueNamePrefix(input)
	if cardName != "" {
		card, err = fetchScryfallCardByFuzzyName(cardName, isLang)
		if err == nil {
			return getCachedOrStoreCard(&card, ncn)
		}
	}
	// Store the empty result
	nameToCardCache.Add(ncn, card)
	return card, fmt.Errorf("No card found")
}

func getDumbScryfallCard(input string, isLang bool) (Card, error) {
	dumbCardRequests.Add(1)
	var card Card
	ncn := normaliseCardName(input)
	log.Debug("Asked for dumb card", "Name", ncn)

	card, err := checkCacheForCard(ncn)
	if err == nil {
		return card, err
	}

	log.Debug("Checking Scryfall for card", "Name", ncn)
	card, err = fetchDumbScryfallCardByName(input, isLang)
	if err == nil {
		return card, nil
	}
	return card, fmt.Errorf("No card found")
}

func getRandomScryfallCard() (Card, error) {
	randomRequests.Add(1)
	var card Card
	log.Debug("GetRandomScryfallCard: Attempting to fetch", "URL", scryfallRandomAPIURL)
	resp, err := http.Get(scryfallRandomAPIURL)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Error("getRandomScryfallCard: The HTTP request failed", "Error", err)
		return card, fmt.Errorf("Something went wrong fetching a random card")
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
			raven.CaptureError(err, nil)
			return card, fmt.Errorf("Something went wrong parsing the card")
		}
		card.getExtraMetadata("")
		nameToCardCache.Add(normaliseCardName(card.Name), card)
		return card, nil
	}
	log.Error("fetchRandomScryfallCard: Scryfall returned a non-200", "Status Code", resp.StatusCode)
	return card, fmt.Errorf("Error retrieving card")
}

func searchScryfallCard(cardTokens []string) ([]Card, error) {
	searchRequests.Add(1)
	// TODO: Validate Search Parameters
	u, _ := url.Parse(scryfallSearchAPIURL)
	q := u.Query()
	q.Add("q", strings.Join(cardTokens, " "))
	u.RawQuery = q.Encode()
	log.Debug("searchScryfallCard: Attempting to fetch", "URL", u)
	resp, err := http.Get(u.String())
	if err != nil {
		raven.CaptureError(err, nil)
		log.Warn("searchScryfallCard: The HTTP request failed", "Error", err)
		return []Card{}, fmt.Errorf("Something went wrong fetching card search results")
	}
	defer resp.Body.Close()

	var csr CardSearchResult
	if resp.StatusCode == 200 {
		if err := json.NewDecoder(resp.Body).Decode(&csr); err != nil {
			raven.CaptureError(err, nil)
			return []Card{}, fmt.Errorf("Something went wrong parsing the card search results")
		}
		log.Debug("searchScryfallCard", "Total cards found", csr.TotalCards)
		return ParseAndFormatSearchResults(csr)
	}
	// This is the general "please learn scryfall syntax" reply
	if resp.StatusCode == 400 {
		if err := json.NewDecoder(resp.Body).Decode(&csr); err != nil {
			raven.CaptureError(err, nil)
			return []Card{}, fmt.Errorf("Something went wrong parsing the card search results")
		}
		log.Error("searchScryfallCard: Scryfall returned 400, handling")
		return []Card{}, fmt.Errorf("%v (%v)", csr.Details, strings.Join(csr.Warnings, " "))
	}

	log.Error("searchScryfallCard: Scryfall returned a non-200, non-400", "Status Code", resp.StatusCode)
	return []Card{}, fmt.Errorf("No cards found")
}

func ParseAndFormatSearchResults(csr CardSearchResult) ([]Card, error) {
	if len(csr.Data) <= 5 {
		for _, c := range csr.Data {
			x := c
			cNcn := normaliseCardName(c.Name)
			// Sneakily add all these to the Cache
			if _, ok := nameToCardCache.Peek(cNcn); !ok {
				go func(cp *Card, cNcn string) {
					getCachedOrStoreCard(cp, cNcn)
				}(&x, cNcn)
			}
		}
	}
	if (csr.Warnings != nil) {
		return []Card{}, fmt.Errorf(strings.Join(csr.Warnings, " "))
	}
	switch {
	case csr.TotalCards == 0:
		return []Card{}, fmt.Errorf("No cards found")
	case csr.TotalCards <= 2:
		minLen := min(2, len(csr.Data))
		return csr.Data[0:minLen], nil
	case csr.TotalCards > 5:
		return []Card{}, fmt.Errorf("Too many cards returned (%v > 5)", csr.TotalCards)
	default:
		// Between 3 and 5 cards
		var names []string
		for _, c := range csr.Data {
			names = append(names, c.Name)
		}
		return []Card{}, fmt.Errorf("[" + strings.Join(names, "], [") + "]")
	}
}

func fetchCardNames() error {
	// Fetch it
	out, err := os.Create(namesFile)
	if err != nil {
		raven.CaptureError(err, nil)
		return err
	}
	log.Debug("FetchCardNames: Attempting to fetch", "URL", scryfallNamesAPIURL)
	resp, err := http.Get(scryfallNamesAPIURL)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Warn("FetchCardNames: The HTTP request failed", "Error", err)
		return fmt.Errorf("Something went wrong fetching the cardname catalog")
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		_, err = io.Copy(out, resp.Body)
		if err != nil {
			log.Warn("FetchCardNames: Error writing to cardNames file", "Error", err)
			return err
		}
		out.Close()
		return nil
	}
	log.Warn("FetchCardNames: Scryfall returned a non-200", "Status Code", resp.StatusCode)
	return fmt.Errorf("Scryfall returned a non-200")
}

func importCardNames(forceFetch bool) ([]string, error) {
	log.Debug("In importCardNames")
	if forceFetch {
		if err := fetchCardNames(); err != nil {
			log.Warn("Error fetching card names", "Error", err)
			return []string{}, err
		}
	}
	if _, err := os.Stat(namesFile); err != nil {
		if err := fetchCardNames(); err != nil {
			log.Warn("Error fetching card names", "Error", err)
			return []string{}, err
		}
	}
	// Parse it.
	f, err := os.Open(namesFile)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Warn("Error opening cardNames file", "Error", err)
		return []string{}, err
	}
	defer f.Close()
	var catalog CardCatalog
	if err := json.NewDecoder(f).Decode(&catalog); err != nil {
		raven.CaptureError(err, nil)
		log.Warn("Error parsing cardnames file", "Error", err)
		return []string{}, fmt.Errorf("Something went wrong parsing the cardname catalog")
	}
	log.Debug("Finished importing", "Length", len(catalog.Data))
	return catalog.Data, nil
}

func fetchHighlanderPoints() error {
	out, err := os.Create(pointsFile)
	if err != nil {
		return err
	}
	log.Debug("FetchHighlanderPoints: Attempting to fetch", "URL", highlanderPointsURL)
	resp, err := http.Get(highlanderPointsURL)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Warn("FetchHighlanderPoints: The HTTP request failed", "Error", err)
		return fmt.Errorf("Something went wrong fetching the points")
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		_, err = io.Copy(out, resp.Body)
		if err != nil {
			raven.CaptureError(err, nil)
			log.Warn("FetchHighlanderPoints: Error writing to points file", "Error", err)
			return err
		}
		out.Close()
		return nil
	}
	log.Warn("FetchHighlanderPoints: The site returned a non-200", "Status Code", resp.StatusCode)
	return fmt.Errorf("Points file returned a non-200")
}

func importHighlanderPoints(forceFetch bool) error {
	log.Debug("In importHighlanderPoints", "Forced?", forceFetch)
	if forceFetch {
		if err := fetchHighlanderPoints(); err != nil {
			raven.CaptureError(err, nil)
			log.Warn("Error fetching points", "Error", err)
			return err
		}
	}
	if _, err := os.Stat(pointsFile); err != nil {
		if err := fetchCardNames(); err != nil {
			raven.CaptureError(err, nil)
			log.Warn("Error fetching points", "Error", err)
			return err
		}
	}
	// Parse it.
	f, err := os.Open(pointsFile)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Warn("Error opening points file", "Error", err)
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lineFields := strings.Fields(scanner.Text())
		cardName := normaliseCardName(strings.Join(lineFields[0:len(lineFields)-1], " "))
		points, err := strconv.Atoi(lineFields[len(lineFields)-1])
		if err != nil {
			log.Warn("Unable to convert points line", "Error", err)
			continue
		}
		if conf.DevMode {
			log.Debug("ImportPoints", "Cardname", cardName, "Points", points)
		}
		highlanderPoints[cardName] = points
	}
	if err := scanner.Err(); err != nil {
		raven.CaptureError(err, nil)
		log.Warn("Error reading points file", "Error", err)
		return err
	}
	return nil
}
