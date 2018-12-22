package main

import (
	"fmt"
	"strings"
)

// Card represents the JSON returned by the Scryfall API
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
	ManaCost      string   `json:"mana_cost"`
	Cmc           float32  `json:"cmc"`
	TypeLine      string   `json:"type_line"`
	OracleText    string   `json:"oracle_text"`
	Power         string   `json:"power"`
	Toughness     string   `json:"toughness"`
	Loyalty       string   `json:"loyalty"`
	Colors        []string `json:"colors"`
	ColorIdentity []string `json:"color_identity"`
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
}

func formatManaCost(card *Card) string {
	return fmt.Sprintf("{%s}", strings.Replace(strings.Replace(card.ManaCost, "{", "", -1), "}", "", -1))
}

// TODO: Find all printings
func formatExpansions(card *Card) string {
	return fmt.Sprintf("%s-%s", strings.ToUpper(card.Set), strings.ToUpper(card.Rarity[0:1]))
}

// "legalities":{"standard":"not_legal","future":"not_legal","frontier":"not_legal","modern":"legal","legacy":"legal","pauper":"not_legal","vintage":"legal","penny":"not_legal","commander":"legal","1v1":"legal","duel":"legal","brawl":"not_legal"}
func formatLegalities(card *Card) string {
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

// CREATURE
// Plaguecrafter {2B} |Creature -- Human Shaman| 3/2 When Plaguecrafter enters the battlefield, each player sacrifices a creature or  planeswalker. Each player who can't discards a card. · GRN-U · Vin,Leg,Mod,Std
// SPELL
// Momentous Fall {2GG} |Instant| As an additional cost to cast this spell, sacrifice a creature. / You draw cards equal to the sacrificed creature's power, then you gain life equal to its toughness. · ROE-R · Vin,Leg,Mod
// ENCHANTMENT
//  Abduction {2UU} |Enchantment -- Aura| Enchant creature / When Abduction enters the battlefield, untap enchanted creature. / You control enchanted creature. / When enchanted creature dies, return that card to the
//  battlefield under its owner's control. · 6E-U,WL-U · Vin,Leg
// PLANESWALKER
//  Jace, Cunning Castaway {1UU} |Legendary Planeswalker -- Jace| 3 loyalty. +1: Whenever one or more creatures you control deal combat damage to a player this turn, draw a card, then discard a card. / -2: Create a 2/2 blue
// Illusion creature token with "When this creature becomes the target of a spell, sacrifice it." / -5: Create two tokens that are copies of Jace, Cunning Castaway, except they're ...
// 07:07 <@Datatog> ... not legendary. · XLN-M · Vin,Leg,Mod,Std
func formatCard(card *Card) string {
	var s []string
	s = append(s, card.Name)
	if card.ManaCost != "" {
		s = append(s, formatManaCost(card))
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
	s = append(s, fmt.Sprintf("· %s ·", formatExpansions(card)))
	s = append(s, formatLegalities(card))
	return strings.Join(s, " ")
}
