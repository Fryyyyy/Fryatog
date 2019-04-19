package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	raven "github.com/getsentry/raven-go"
	log "gopkg.in/inconshreveable/log15.v2"
)

const namesFile = "names.json"
const scryfallNamesAPIURL = "https://api.scryfall.com/catalog/card-names"
const scryfallFuzzyAPIURL = "https://api.scryfall.com/cards/named?fuzzy=%s"
const scryfallRandomAPIURL = "https://api.scryfall.com/cards/random"

// CardList represents the Scryfall List API when retrieving multiple cards
type CardList struct {
	Object     string   `json:"object"`
	TotalCards int      `json:"total_cards"`
	Warnings   []string `json:"warnings"`
	HasMore    bool     `json:"has_more"`
	NextPage   string   `json:"next_page"`
	Data       []Card   `json:"data"`
}

// CardRuling contains an individual ruling on a card
type CardRuling struct {
	Object      string `json:"object"`
	OracleID    string `json:"oracle_id"`
	Source      string `json:"source"`
	PublishedAt string `json:"published_at"`
	Comment     string `json:"comment"`
}

func (ruling *CardRuling) formatRuling() string {
	return fmt.Sprintf("%v: %v", ruling.PublishedAt, ruling.Comment)
}

// CardMetadata contains some extraneous extra information we sometimes retrieve
type CardMetadata struct {
	PreviousPrintings     []string
	PreviousFlavourTexts  []string
	PreviousReminderTexts []string
}

// CardRulingResult represents the JSON returned by the /cards/{}/rulings Scryfall API
type CardRulingResult struct {
	Object  string       `json:"object"`
	HasMore bool         `json:"has_more"`
	Data    []CardRuling `json:"data"`
}

// CardFace represents the individual information for each face of a DFC
type CardFace struct {
	Object          string   `json:"object"`
	Name            string   `json:"name"`
	ManaCost        string   `json:"mana_cost"`
	TypeLine        string   `json:"type_line"`
	ColorIndicators []string `json:"color_indicator"`
	OracleText      string   `json:"oracle_text"`
	Power           string   `json:"power"`
	Toughness       string   `json:"toughness"`
	Watermark       string   `json:"watermark"`
	Artist          string   `json:"artist"`
	IllustrationID  string   `json:"illustration_id,omitempty"`
}

// Card represents the JSON returned by the /cards Scryfall API
type Card struct {
	Object        string `json:"object"`
	ID            string `json:"id"`
	OracleID      string `json:"oracle_id"`
	MultiverseIds []int  `json:"multiverse_ids"`
	MtgoID        int    `json:"mtgo_id"`
	MtgoFoilID    int    `json:"mtgo_foil_id"`
	TcgplayerID   int    `json:"tcgplayer_id"`
	Name          string `json:"name"`
	Lang          string `json:"lang"`
	ReleasedAt    string `json:"released_at"`
	URI           string `json:"uri"`
	ScryfallURI   string `json:"scryfall_uri"`
	Layout        string `json:"layout"`
	HighresImage  bool   `json:"highres_image"`
	ImageUris     struct {
		Small      string `json:"small"`
		Normal     string `json:"normal"`
		Large      string `json:"large"`
		Png        string `json:"png"`
		ArtCrop    string `json:"art_crop"`
		BorderCrop string `json:"border_crop"`
	} `json:"image_uris"`
	ManaCost        string     `json:"mana_cost"`
	Cmc             float32    `json:"cmc"`
	TypeLine        string     `json:"type_line"`
	OracleText      string     `json:"oracle_text"`
	Power           string     `json:"power"`
	Toughness       string     `json:"toughness"`
	Loyalty         string     `json:"loyalty"`
	Colors          []string   `json:"colors"`
	ColorIndicators []string   `json:"color_indicator"`
	ColorIdentity   []string   `json:"color_identity"`
	CardFaces       []CardFace `json:"card_faces"`
	Legalities      struct {
		Standard  string `json:"standard"`
		Future    string `json:"future"`
		Frontier  string `json:"frontier"`
		Modern    string `json:"modern"`
		Legacy    string `json:"legacy"`
		Pauper    string `json:"pauper"`
		Vintage   string `json:"vintage"`
		Penny     string `json:"penny"`
		Commander string `json:"commander"`
		OneV1     string `json:"1v1"`
		Duel      string `json:"duel"`
		Brawl     string `json:"brawl"`
	} `json:"legalities"`
	Games           []string `json:"games"`
	Reserved        bool     `json:"reserved"`
	Foil            bool     `json:"foil"`
	Nonfoil         bool     `json:"nonfoil"`
	Oversized       bool     `json:"oversized"`
	Promo           bool     `json:"promo"`
	Reprint         bool     `json:"reprint"`
	Set             string   `json:"set"`
	SetName         string   `json:"set_name"`
	SetURI          string   `json:"set_uri"`
	SetSearchURI    string   `json:"set_search_uri"`
	ScryfallSetURI  string   `json:"scryfall_set_uri"`
	RulingsURI      string   `json:"rulings_uri"`
	PrintsSearchURI string   `json:"prints_search_uri"`
	CollectorNumber string   `json:"collector_number"`
	Digital         bool     `json:"digital"`
	Rarity          string   `json:"rarity"`
	FlavourText     string   `json:"flavor_text"`
	IllustrationID  string   `json:"illustration_id"`
	Artist          string   `json:"artist"`
	BorderColor     string   `json:"border_color"`
	Frame           string   `json:"frame"`
	FrameEffect     string   `json:"frame_effect"`
	FullArt         bool     `json:"full_art"`
	Timeshifted     bool     `json:"timeshifted"`
	Colorshifted    bool     `json:"colorshifted"`
	Futureshifted   bool     `json:"futureshifted"`
	StorySpotlight  bool     `json:"story_spotlight"`
	EdhrecRank      int      `json:"edhrec_rank"`
	Usd             string   `json:"usd"`
	Eur             string   `json:"eur"`
	Tix             string   `json:"tix"`
	RelatedUris     struct {
		Gatherer       string `json:"gatherer"`
		TcgplayerDecks string `json:"tcgplayer_decks"`
		Edhrec         string `json:"edhrec"`
		Mtgtop8        string `json:"mtgtop8"`
	} `json:"related_uris"`
	PurchaseUris struct {
		Tcgplayer   string `json:"tcgplayer"`
		Cardmarket  string `json:"cardmarket"`
		Cardhoarder string `json:"cardhoarder"`
	} `json:"purchase_uris"`
	Rulings  []CardRuling
	Metadata CardMetadata
}

// TODO: Also CardFaces
func (card *Card) getExtraMetadata(inputURL string) {
	log.Debug("Getting Metadata")
	// This is called even for empty Card objects, do don't do anything in that case
	if card.ID == "" {
		return
	}
	fetchURL := card.PrintsSearchURI
	var cm CardMetadata
	// Aready have metadata?
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
		log.Debug("After metadata extraction", "Card", card)
		return
	}
	log.Info("GetExtraMetadata: Scryfall returned a non-200", "Status Code", resp.StatusCode)
	return
}

func formatManaCost(input string) string {
	// return fmt.Sprintf("{%s}", strings.Replace(strings.Replace(input, "{", "", -1), "}", "", -1))
	return input
}

// TODO: Have a command to see all printing information
func (card *Card) formatExpansions() string {
	ret := ""
	if card.Name != "Plains" && card.Name != "Island" && card.Name != "Swamp" && card.Name != "Mountain" && card.Name != "Forest" {
		if len(card.Metadata.PreviousPrintings) > 0 {
			if len(card.Metadata.PreviousPrintings) < 10 {
				ret = fmt.Sprintf("%s,", strings.Join(card.Metadata.PreviousPrintings, ","))
			} else {
				ret = fmt.Sprintf("%s,[...],", strings.Join(card.Metadata.PreviousPrintings[:5], ","))
			}
		}
	}
	return ret + fmt.Sprintf("%s-%s", strings.ToUpper(card.Set), strings.ToUpper(card.Rarity[0:1]))
}

// Get all possible most recent reminder texts for a card, \n separated
// TODO/NOTE: This doesn't work, since Scryfall doesn't actually give the printed_text field for each previous printing,
// just the current Oracle text.
func (card Card) getReminderTexts() string {
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
	if card.FlavourText != "" {
		return card.FlavourText
	}
	if len(card.Metadata.PreviousFlavourTexts) > 0 {
		return card.Metadata.PreviousFlavourTexts[0]
	}
	return "Flavour text not found"
}

func (card *Card) formatLegalities() string {
	var ret []string
	switch card.Legalities.Vintage {
	case "legal":
		ret = append(ret, "Vin")
	case "restricted":
		ret = append(ret, "VinRes")
	case "banned":
		ret = append(ret, "VinBan")
	}
	switch card.Legalities.Legacy {
	case "legal":
		ret = append(ret, "Leg")
	case "restricted":
		ret = append(ret, "LegRes")
	case "banned":
		ret = append(ret, "LegBan")
	}
	switch card.Legalities.Modern {
	case "legal":
		ret = append(ret, "Mod")
	case "restricted":
		ret = append(ret, "ModRes")
	case "banned":
		ret = append(ret, "ModBan")
	}
	switch card.Legalities.Standard {
	case "legal":
		ret = append(ret, "Std")
	case "restricted":
		ret = append(ret, "StdRes")
	case "banned":
		ret = append(ret, "StdBan")
	}
	return strings.Join(ret, ",")
}

func (card *Card) formatCard() string {
	var s []string
	if len(card.CardFaces) > 0 {
		// DFC and Flip and Split - produce two cards
		for _, cf := range card.CardFaces {
			var r []string
			// Bold card name
			r = append(r, fmt.Sprintf("\x02%s\x0F", cf.Name))
			if cf.ManaCost != "" {
				r = append(r, formatManaCost(cf.ManaCost))
			}
			r = append(r, fmt.Sprintf("· %s ·", cf.TypeLine))
			if cf.Power != "" {
				r = append(r, fmt.Sprintf("%s/%s ·", cf.Power, cf.Toughness))
			}
			if len(cf.ColorIndicators) > 0 {
				formattedColorIndicator := standardiseColorIndicator(cf.ColorIndicators)
				r = append(r, fmt.Sprintf("%s ·", formattedColorIndicator))
			}
			r = append(r, strings.Replace(cf.OracleText, "\n", " \\ ", -1))
			if cf.ManaCost != "" {
				r = append(r, fmt.Sprintf("· %s ·", card.formatExpansions()))
				r = append(r, card.formatLegalities())
			}

			s = append(s, strings.Join(r, " "))
		}
		return strings.Join(s, "\n")
	}
	// Normal card
	s = append(s, fmt.Sprintf("\x02%s\x0F", card.Name))
	if card.ManaCost != "" {
		s = append(s, formatManaCost(card.ManaCost))
	}
	s = append(s, fmt.Sprintf("· %s ·", card.TypeLine))
	// P/T or Loyalty or Nothing
	if card.Power != "" {
		s = append(s, fmt.Sprintf("%s/%s ·", card.Power, card.Toughness))
	}
	if len(card.ColorIndicators) > 0 {
		formattedColorIndicator := standardiseColorIndicator(card.ColorIndicators)
		s = append(s, fmt.Sprintf("%s ·", formattedColorIndicator))
	}
	if strings.Contains(card.TypeLine, "Planeswalker") {
		s = append(s, fmt.Sprintf("[%s]", card.Loyalty))
	}

	// Change linebreaks to \\
	modifiedOracleText := strings.Replace(card.OracleText, "\n", " \\ ", -1)
	// Change the open/closing parens of reminder text to also start and end italics
	modifiedOracleText = strings.Replace(modifiedOracleText, "(", "\x1D(", -1)
	modifiedOracleText = strings.Replace(modifiedOracleText, ")", ")\x0F", -1)
	
	s = append(s, modifiedOracleText)
	s = append(s, fmt.Sprintf("· %s ·", card.formatExpansions()))
	s = append(s, card.formatLegalities())

	return strings.Join(s, " ")
}

func standardiseColorIndicator(ColorIndicators []string) string {
	expandedColors := map[string]string{"W": "White",
		"U": "Blue",
		"B": "Black",
		"R": "Red",
		"G": "Green"}
	mappedColors := map[string]int{"White": 0,
		"Blue":  1,
		"Black": 2,
		"Red":   3,
		"Green": 4}

	var colorWords []string
	for _, color := range ColorIndicators {
		colorWords = append(colorWords, expandedColors[color])
	}

	sort.Slice(colorWords, func(i, j int) bool {
		return mappedColors[colorWords[i]] < mappedColors[colorWords[j]]
	})

	return "[" + strings.Join(colorWords, "/") + "]"
}

func normaliseCardName(input string) string {
	ret := nonAlphaRegex.ReplaceAllString(strings.ToLower(input), "")
	// log.Debug("Normalising", "Input", input, "Output", ret)
	return ret
}

func lookupUniqueNamePrefix(input string) string {
	log.Debug("in lookupUniqueNamePrefix", "Input", input, "NCN", normaliseCardName(input), "Length of CN", len(cardNames))
	var err error
	if len(cardNames) == 0 {
		log.Debug("In lookupUniqueNamePrefix -- Importing")
		cardNames, err = importCardNames(false)
		if err != nil {
			log.Warn("Error importing card names", "Error", err)
			return ""
		}
	}
	c := cardNames[:0]
	for _, x := range cardNames {
		if strings.HasPrefix(normaliseCardName(x), normaliseCardName(input)) {
			log.Debug("In lookupUniqueNamePrefix", "Gottem", x)
			c = append(c, x)
		}
	}
	log.Debug("In lookupUniqueNamePrefix", "C", c)
	if len(c) == 1 {
		return c[0]
	}
	// Look for something legendary-ish
	var i int
	var j string
	for _, x := range c {
		if strings.Contains(x, ",") || strings.Contains(x, "the") {
			i++
			j = x
		}
	}
	if i == 1 {
		return j
	}
	return ""
}

func fetchScryfallCardByFuzzyName(input string) (Card, error) {
	url := fmt.Sprintf(scryfallFuzzyAPIURL, url.QueryEscape(input))
	log.Debug("fetchScryfallCard: Attempting to fetch", "URL", url)
	resp, err := http.Get(url)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Warn("fetchScryfallCard: The HTTP request failed", "Error", err)
		return Card{}, fmt.Errorf("Something went wrong fetching the card")
	}
	defer resp.Body.Close()
	var card Card
	if resp.StatusCode == 200 {
		if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
			raven.CaptureError(err, nil)
			return card, fmt.Errorf("Something went wrong parsing the card")
		}
		return card, nil
	}
	log.Info("fetchScryfallCard: Scryfall returned a non-200", "Status Code", resp.StatusCode)
	return card, fmt.Errorf("Card not found by Scryfall")
}

func checkCacheForCard(ncn string) (Card, error) {
	log.Debug("Checking cache for card", "Name", ncn)
	var emptyCard Card
	if cacheCard, found := nameToCardCache.Get(ncn); found {
		log.Debug("Card was cached")
		if cacheCard == nil {
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
	log.Debug("Not in cache")
	return emptyCard, fmt.Errorf("Card not found in cache")
}

func getScryfallCard(input string) (Card, error) {
	var card Card
	// Normalise input to match how we store in the cache:
	// lowercase, no punctuation.
	ncn := normaliseCardName(input)
	log.Debug("Checking Scryfall for card", "Name", ncn)
	card, err := checkCacheForCard(ncn)
	if err == nil || (err != nil && err.Error() == "Card not found") {
		return card, err
	}

	// Try fuzzily matching the name
	card, err = fetchScryfallCardByFuzzyName(input)
	if err == nil {
		cc, err := checkCacheForCard(normaliseCardName(card.Name))
		if err == nil {
			return cc, err
		}
		nameToCardCache.Add(ncn, card)
		nameToCardCache.Add(normaliseCardName(card.Name), card)
		card.getExtraMetadata("")
		return card, nil
	}
	// No luck - try unique prefix
	cardName := lookupUniqueNamePrefix(input)
	if cardName != "" {
		card, err = fetchScryfallCardByFuzzyName(cardName)
		if err == nil {
			cc, err := checkCacheForCard(normaliseCardName(card.Name))
			if err == nil {
				return cc, err
			}
			card.getExtraMetadata("")
			nameToCardCache.Add(ncn, card)
			nameToCardCache.Add(normaliseCardName(card.Name), card)
			return card, nil
		}
	}
	// Store what they typed
	nameToCardCache.Add(ncn, nil)
	return card, fmt.Errorf("No card found")
}

func getRandomScryfallCard() (Card, error) {
	var card Card
	log.Debug("GetRandomScryfallCard: Attempting to fetch", "URL", scryfallRandomAPIURL)
	resp, err := http.Get(scryfallRandomAPIURL)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Warn("FetchCardNames: The HTTP request failed", "Error", err)
		return card, fmt.Errorf("Something went wrong fetching the cardname catalog")
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
	log.Info("fetchScryfallCard: Scryfall returned a non-200", "Status Code", resp.StatusCode)
	return card, fmt.Errorf("Card not found by Scryfall")
}

// CardCatalog stores the result of the catalog/card-names API call
type CardCatalog struct {
	Object      string   `json:"object"`
	URI         string   `json:"uri"`
	TotalValues int      `json:"total_values"`
	Data        []string `json:"data"`
}

func fetchCardNames() error {
	// Fetch it
	out, err := os.Create(namesFile)
	if err != nil {
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
	defer f.Close()
	if err != nil {
		log.Warn("Error opening cardNames file", "Error", err)
		return []string{}, err
	}
	var catalog CardCatalog
	if err := json.NewDecoder(f).Decode(&catalog); err != nil {
		// raven.CaptureError(err, nil)
		log.Warn("Error parsing cardnames file", "Error", err)
		return []string{}, fmt.Errorf("Something went wrong parsing the cardname catalog")
	}
	return catalog.Data, nil
}

func (card *Card) getRulings(rulingNumber int) string {
	// Do we already have the Rulings?
	if card.Rulings == nil {
		// If we don't, fetch them
		err := (card).fetchRulings()
		if err != nil {
			return "Problem fetching the rulings"
		}
		// Update the Cache ???? Necessary ?
		nameToCardCache.Add(normaliseCardName(card.Name), card)
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
		return "Too many rulings, please request a specific one"
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
