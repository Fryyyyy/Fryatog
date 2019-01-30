package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strings"

	raven "github.com/getsentry/raven-go"
	closestmatch "github.com/schollz/closestmatch"
	log "gopkg.in/inconshreveable/log15.v2"
)

const namesFile = "names.json"
const namesURL = "https://api.scryfall.com/catalog/card-names"

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

func (ruling CardRuling) formatRuling() string {
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
	Object         string `json:"object"`
	Name           string `json:"name"`
	ManaCost       string `json:"mana_cost"`
	TypeLine       string `json:"type_line"`
	OracleText     string `json:"oracle_text"`
	Power          string `json:"power"`
	Toughness      string `json:"toughness"`
	Watermark      string `json:"watermark"`
	Artist         string `json:"artist"`
	IllustrationID string `json:"illustration_id,omitempty"`
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
	ManaCost      string     `json:"mana_cost"`
	Cmc           float32    `json:"cmc"`
	TypeLine      string     `json:"type_line"`
	OracleText    string     `json:"oracle_text"`
	Power         string     `json:"power"`
	Toughness     string     `json:"toughness"`
	Loyalty       string     `json:"loyalty"`
	Colors        []string   `json:"colors"`
	ColorIdentity []string   `json:"color_identity"`
	CardFaces     []CardFace `json:"card_faces"`
	Legalities    struct {
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
func (card Card) getExtraMetadata(inputURL string) error {
	log.Debug("Getting Metadata")
	// This is called even for empty Card objects, do don't do anything in that case
	if card.ID == "" {
		return nil
	}
	fetchURL := card.PrintsSearchURI
	var cm CardMetadata
	// Aready have metadata?
	if !reflect.DeepEqual(card.Metadata, cm) {
		return nil
	}
	// Use parameter over stored, for recursive lists
	if inputURL != "" {
		fetchURL = inputURL
	}
	// Have a url?
	if fetchURL == "" {
		return fmt.Errorf("No prints URI")
	}
	log.Debug("GetExtraMetadata: Attempting to fetch", "URL", fetchURL)
	resp, err := http.Get(fetchURL)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Warn("GetExtraMetadata: The HTTP request failed", "Error", err)
		return fmt.Errorf("Something went wrong fetching the card list")
	}
	defer resp.Body.Close()
	var list CardList
	if resp.StatusCode == 200 {
		if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
			raven.CaptureError(err, nil)
			return fmt.Errorf("Something went wrong parsing the card list")
		}
		if len(list.Warnings) > 0 {
			return fmt.Errorf("Scryfall said there were errors: %v", list.Warnings)
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
			// Only need previous reminder text if current one doesn't have
			if c.getReminderTexts() == "Reminder text not found" && c.getReminderTexts() != "Reminder text not found" {
				cm.PreviousReminderTexts = append(cm.PreviousReminderTexts, c.getReminderTexts())
			}
		}
		card.Metadata = cm
		// Update the Cache ???? Necessary ?
		nameToCardCache.Add(normaliseCardName(card.Name), card)
		return nil
	}
	log.Info("GetExtraMetadata: Scryfall returned a non-200", "Status Code", resp.StatusCode)
	return fmt.Errorf("Card list not found by Scryfall !?")
}

func formatManaCost(input string) string {
	// return fmt.Sprintf("{%s}", strings.Replace(strings.Replace(input, "{", "", -1), "}", "", -1))
	return input
}

func (card Card) formatExpansions() string {
	ret := ""
	if card.Name != "Plains" && card.Name != "Island" && card.Name != "Swamp" && card.Name != "Mountain" && card.Name != "Forest" {
		if len(card.Metadata.PreviousPrintings) > 0 {
			ret = fmt.Sprintf("%s,", strings.Join(card.Metadata.PreviousPrintings, ","))
		}
	}
	return ret + fmt.Sprintf("%s-%s", strings.ToUpper(card.Set), strings.ToUpper(card.Rarity[0:1]))
}

// Get all possible most recent reminder texts for a card, \n separated
// TODO/NOTE: This doesn't work, since Scryfall doesn't actually give the printed_text field for each previous printing,
// just the current Oracle text.
func (card Card) getReminderTexts() string {
	reminderRegexp := regexp.MustCompile(`\((.*?)\)`)
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
func (card Card) getFlavourText() string {
	if card.FlavourText != "" {
		return card.FlavourText
	}
	if len(card.Metadata.PreviousFlavourTexts) > 0 {
		return card.Metadata.PreviousFlavourTexts[0]
	}
	return "Flavour text not found"
}

func (card Card) formatLegalities() string {
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

func (card Card) formatCard() string {
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
			r = append(r, fmt.Sprintf("| %s |", cf.TypeLine))
			if strings.Contains(cf.TypeLine, "Creature") {
				r = append(r, fmt.Sprintf("%s/%s", cf.Power, cf.Toughness))
			}
			r = append(r, strings.Replace(cf.OracleText, "\n", " \\ ", -1))
			r = append(r, fmt.Sprintf("· %s ·", card.formatExpansions()))
			r = append(r, card.formatLegalities())

			s = append(s, strings.Join(r, " "))
		}
		return strings.Join(s, "\n")
	}
	// Normal card
	s = append(s, fmt.Sprintf("\x02%s\x0F", card.Name))
	if card.ManaCost != "" {
		s = append(s, formatManaCost(card.ManaCost))
	}
	s = append(s, fmt.Sprintf("| %s |", card.TypeLine))
	// P/T or Loyalty or Nothing
	if strings.Contains(card.TypeLine, "Creature") {
		s = append(s, fmt.Sprintf("%s/%s", card.Power, card.Toughness))
	}
	if strings.Contains(card.TypeLine, "Planeswalker") {
		s = append(s, fmt.Sprintf("[%s]", card.Loyalty))
	}
	s = append(s, strings.Replace(card.OracleText, "\n", " \\ ", -1))
	s = append(s, fmt.Sprintf("· %s ·", card.formatExpansions()))
	s = append(s, card.formatLegalities())

	return strings.Join(s, " ")
}

func normaliseCardName(input string) string {
	nonAlphaRegex := regexp.MustCompile(`\W+`)
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
	url := fmt.Sprintf("https://api.scryfall.com/cards/named?fuzzy=%s", url.QueryEscape(input))
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

func getScryfallCard(input string) (Card, error) {
	var card Card
	defer func() {
		go card.getExtraMetadata("")
	}()
	// Normalise input to match how we store in the cache:
	// lowercase, no punctuation.
	ncn := normaliseCardName(input)
	if cacheCard, found := nameToCardCache.Get(ncn); found {
		log.Debug("Card was cached")
		if cacheCard == nil {
			return card, fmt.Errorf("Card not found")
		}
		return cacheCard.(Card), nil
	}
	// Try fuzzily matching the name
	card, err := fetchScryfallCardByFuzzyName(input)
	if err == nil {
		nameToCardCache.Add(ncn, card)
		nameToCardCache.Add(normaliseCardName(card.Name), card)
		return card, nil
	}
	// No luck - try unique prefix
	cardName := lookupUniqueNamePrefix(input)
	if cardName != "" {
		card, err = fetchScryfallCardByFuzzyName(cardName)
		if err == nil {
			nameToCardCache.Add(ncn, card)
			nameToCardCache.Add(normaliseCardName(card.Name), card)
			return card, nil
		}
	}
	// Store what they typed
	nameToCardCache.Add(ncn, nil)
	return card, fmt.Errorf("No card found")
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
	log.Debug("FetchCardNames: Attempting to fetch", "URL", namesURL)
	resp, err := http.Get(namesURL)
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
	cardCM, err = closestmatch.Load(cardNamesGob)
	if err != nil {
		log.Debug("Cards CM -- Creating from Scratch")
		cardCM = closestmatch.New(cardNames, []int{2, 3, 4, 5, 6, 7})
		err = cardCM.Save(cardNamesGob)
		log.Warn("Cards CM", "Error", err)
	}
	// log.Debug("Cards CM", "Accuracy", cardCM.AccuracyMutatingWords())
	return catalog.Data, nil
}

func (card Card) getRulings(rulingNumber int) string {
	// Do we already have the Rulings?
	if card.Rulings == nil {
		// If we don't, fetch them
		err := (&card).fetchRulings()
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
	url := fmt.Sprintf(card.RulingsURI)
	log.Debug("FetchRulings: Attempting to fetch", "URL", url)
	resp, err := http.Get(url)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Warn("FetchRulings: The HTTP request failed", "Error", err)
		return fmt.Errorf("Something went wrong fetching the card")
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		var crr CardRulingResult
		if err := json.NewDecoder(resp.Body).Decode(&crr); err != nil {
			raven.CaptureError(err, nil)
			return fmt.Errorf("Something went wrong parsing the card")
		}
		// Store what they typed, and also the real card name.
		card.Rulings = crr.Data
		return nil
	}
	log.Info("FetchRulings: Scryfall returned a non-200", "Status Code", resp.StatusCode)
	return fmt.Errorf("Unable to fetch rulings from Scryfall")
}
