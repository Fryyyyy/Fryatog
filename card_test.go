package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	lru "github.com/hashicorp/golang-lru"
)

var (
	TestCardWithNoRulings        = Card{ID: "1", Name: "TestCardWithNoRulings"}
	TestCardWithEmptyRulings     = Card{ID: "1", Name: "TestCardWithNoRulings", Rulings: []CardRuling{}}
	TestCardWithOneNonWOTCRuling = Card{ID: "1", Name: "TestCardWithOneNonWOTCRuling", Rulings: []CardRuling{{Source: "Fry", Comment: "No Print!"}}}
	TestCardWithOneWOTCRuling    = Card{ID: "1", Name: "TestCardWithOneWOTCRuling", Rulings: []CardRuling{{Source: "wotc", Comment: "Print Me", PublishedAt: "1900-01-01"}}}
	TestCardWithTwoWOTCRulings   = Card{ID: "1", Name: "TestCardWithTwoWOTCRulings", Rulings: []CardRuling{{Source: "wotc", Comment: "Print Me", PublishedAt: "1900-02-02"}, {Source: "wotc", Comment: "Print Me Too!", PublishedAt: "1900-03-03"}}}
	FakeCards                    = []Card{TestCardWithNoRulings, TestCardWithEmptyRulings, TestCardWithOneNonWOTCRuling, TestCardWithOneWOTCRuling, TestCardWithTwoWOTCRulings}
	RealCards                    = map[string]string{
		"Ponder":                      "test_data/ponder.json",
		"Shahrazad":                   "test_data/shahrazad.json",
		"Jace, the Mind Sculptor":     "test_data/jacethemindsculptor.json",
		"Expansion":                   "test_data/expansion.json",
		"Bushi Tenderfoot":            "test_data/bushitenderfoot.json",
		"Claim":                       "test_data/claimtofame.json",
		"Poison-Tip Archer":           "test_data/poisontiparcher.json",
		"Faithless Looting":           "test_data/faithlesslooting.json",
		"Fleetwheel Cruiser":          "test_data/fleetwheelcruiser.json",
		"Disenchant":                  "test_data/disenchant.json",
		"Tawnos's Coffin":             "test_data/tawnosscoffin.json",
		"Ixidron":                     "test_data/ixidron.json",
		"Nicol Bolas, the Arisen":     "test_data/nicolbolasthearisen.json",
		"Dryad Arbor":                 "test_data/dryadarbor.json",
		"Arlinn Kord":                 "test_data/arlinnkord.json",
		"Consign":                     "test_data/consign.json",
		"Jace, Vryn's Prodigy":        "test_data/jacevrynsprodigy.json",
		"Mairsil, the Pretender":      "test_data/mairsil.json",
		"Ral, Storm Conduit":          "test_data/ralstormconduit.json",
		"Tarmogoyf":                   "test_data/tarmogoyf.json",
		"Sarkhan the Masterless Plus": "test_data/sarkhanthemasterlessplus.json",
		"Fire//Ice No Multiverse":     "test_data/fireice-nomultiverse.json",
		"Ancestral Recall":            "test_data/ancestralrecall.json",
		"Crashing Footfalls":          "test_data/crashingfootfalls.json",
		"Confiscate":                  "test_data/confiscate.json",
		"Thermo-Alchemist":            "test_data/thermoalchemist.json",
		"Wicked Guardian":             "test_data/wickedguardian.json",
		"Erebos' Titan":               "test_data/erebostitanDE.json",
		"Erebos's Titan":              "test_data/erebosstitan.json",
		"Halvar, God of Battle":       "test_data/halvargodofbattle.json",
		"Cosima, God of the Voyage":   "test_data/cosimagodofthevoyage.json",
		"Kindly Ancestor":             "test_data/kindlyancestor.json",
	}
)

func (card *Card) fakeGetLangs(lang string) (Card, error) {
	var c Card
	var list CardList
	fi, err := os.Open("test_data/" + normaliseCardName(card.Name) + "-langs.json")
	if err != nil {
		return c, fmt.Errorf("Unable to open langs JSON: %v", err)
	}
	if err := json.NewDecoder(fi).Decode(&list); err != nil {
		return c, fmt.Errorf("Something went wrong parsing the card list")
	}
	if len(list.Warnings) > 0 {
		return c, fmt.Errorf("Scryfall said there were errors: %v", list.Warnings)
	}
	// These are in printing order, since the prints_search_uri includes "order=released"
	for _, cs := range list.Data {
		if cs.Lang == lang {
			return cs, nil
		}
	}
	return c, fmt.Errorf("Unknown Language")
}

func (card *Card) fakeGetMetadata() error {
	var cm CardMetadata
	var list CardList
	fi, err := os.Open("test_data/" + normaliseCardName(card.Name) + "-printings.json")
	if err != nil {
		return fmt.Errorf("Unable to open printings JSON: %v", err)
	}
	if err := json.NewDecoder(fi).Decode(&list); err != nil {
		return fmt.Errorf("Something went wrong parsing the card list")
	}
	if len(list.Warnings) > 0 {
		return fmt.Errorf("Scryfall said there were errors: %v", list.Warnings)
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
	return nil
}

func (card *Card) fakeGetRulings(rulingNumber int) string {
	var crr CardRulingResult
	fi, err := os.Open("test_data/" + normaliseCardName(card.Name) + "-rulings.json")
	if err != nil {
		return fmt.Sprintf("Unable to open printings JSON: %v", err)
	}
	if err = json.NewDecoder(fi).Decode(&crr); err != nil {
		return fmt.Sprintf("Something went wrong parsing the rulings: %v", err)
	}

	card.Rulings = crr.Data

	err = card.sortRulings()
	if err != nil {
		return "Something went wrong sorting the rulings"
	}

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

func TestPrintCardForIRC(t *testing.T) {
	tables := []struct {
		cardname string
		output   string
	}{
		{"Ponder", "\x02Ponder\x0F {U} · Sorcery · Look at the top three cards of your library, then put them back in any order. You may shuffle your library. \\ Draw a card. · C18-C · VinRes,Cmr,Leg,ModBan"},
		{"Shahrazad", "\x02Shahrazad\x0F {W}{W} · Sorcery · Players play a Magic subgame, using their libraries as their decks. Each player who doesn't win the subgame loses half their life, rounded up. · ARN-R · [RL] · VinBan,CmrBan,LegBan"},
		{"Jace, the Mind Sculptor", "\x02Jace, the Mind Sculptor\x0F {2}{U}{U} · Legendary Planeswalker — Jace · [3] +2: Look at the top card of target player's library. You may put that card on the bottom of that player's library. \\ 0: Draw three cards, then put two cards from your hand on top of your library in any order. \\ −1: Return target creature to its owner's hand. \\ −12: Exile all cards from target player's library, then that player shuffles their hand into their library. · A25-M · Vin,Cmr,Leg,Mod"},
		{"jtms", "\x02Jace, the Mind Sculptor\x0F {2}{U}{U} · Legendary Planeswalker — Jace · [3] +2: Look at the top card of target player's library. You may put that card on the bottom of that player's library. \\ 0: Draw three cards, then put two cards from your hand on top of your library in any order. \\ −1: Return target creature to its owner's hand. \\ −12: Exile all cards from target player's library, then that player shuffles their hand into their library. · A25-M · Vin,Cmr,Leg,Mod"},
		{"Expansion", "\x02Expansion\x0F {U/R}{U/R} · Instant · Copy target instant or sorcery spell with converted mana cost 4 or less. You may choose new targets for the copy. · GRN-R · Vin,Cmr,Leg,Mod,Std\n\x02Explosion\x0F {X}{U}{U}{R}{R} · Instant · Explosion deals X damage to any target. Target player draws X cards. · GRN-R · Vin,Cmr,Leg,Mod,Std"},
		{"Bushi Tenderfoot", "\x02Bushi Tenderfoot\x0F {W} · Creature — Human Soldier · 1/1 · When a creature dealt damage by Bushi Tenderfoot this turn dies, flip Bushi Tenderfoot. · CHK-U · Vin,Cmr,Leg,Mod\n\x02Kenzo the Hardhearted\x0F · Legendary Creature — Human Samurai · 3/4 · Double strike; bushido 2 \x1D(Whenever this creature blocks or becomes blocked, it gets +2/+2 until end of turn.)\x0F"},
		{"Fleetwheel Cruiser", "\x02Fleetwheel Cruiser\x0F {4} · Artifact — Vehicle · 5/3 · Trample, haste \\ When Fleetwheel Cruiser enters the battlefield, it becomes an artifact creature until end of turn. \\ Crew 2 \x1D(Tap any number of creatures you control with total power 2 or more: This Vehicle becomes an artifact creature until end of turn.)\x0F · KLD-R · Vin,Cmr,Leg,Mod"},
		{"Nicol Bolas, the Arisen", "\x02Nicol Bolas, the Ravager\x0F {1}{U}{B}{R} · Legendary Creature — Elder Dragon · 4/4 · Flying \\ When Nicol Bolas, the Ravager enters the battlefield, each opponent discards a card. \\ {4}{U}{B}{R}: Exile Nicol Bolas, the Ravager, then return him to the battlefield transformed under his owner's control. Activate this ability only any time you could cast a sorcery. · M19-M · Vin,Cmr,Leg,Mod,Std\n\x02Nicol Bolas, the Arisen\x0F · Legendary Planeswalker — Bolas · [Blue/Black/Red] · [7] +2: Draw two cards. \\ −3: Nicol Bolas, the Arisen deals 10 damage to target creature or planeswalker. \\ −4: Put target creature or planeswalker card from a graveyard onto the battlefield under your control. \\ −12: Exile all but the bottom card of target player's library."},
		{"Dryad Arbor", "\x02Dryad Arbor\x0F · Land Creature — Forest Dryad · 1/1 · [Green] · \x1D(Dryad Arbor isn't a spell, it's affected by summoning sickness, and it has \"{T}: Add {G}.\")\x0F · FUT-U · Vin,Cmr,Leg,Mod"},
		{"Arlinn Kord", "\x02Arlinn Kord\x0F {2}{R}{G} · Legendary Planeswalker — Arlinn · [3] +1: Until end of turn, up to one target creature gets +2/+2 and gains vigilance and haste. \\ 0: Create a 2/2 green Wolf creature token. Transform Arlinn Kord. · SOI-M · Vin,Cmr,Leg,Mod\n\x02Arlinn, Embraced by the Moon\x0F · Legendary Planeswalker — Arlinn · [Red/Green] · +1: Creatures you control get +1/+1 and gain trample until end of turn. \\ −1: Arlinn, Embraced by the Moon deals 3 damage to any target. Transform Arlinn, Embraced by the Moon. \\ −6: You get an emblem with \"Creatures you control have haste and '{T}: This creature deals damage equal to its power to any target.'\""},
		{"Consign", "\x02Consign\x0F {1}{U} · Instant · Return target nonland permanent to its owner's hand. · HOU-U · Vin,Cmr,Leg,Mod\n\x02Oblivion\x0F {4}{B} · Sorcery · Aftermath \x1D(Cast this spell only from your graveyard. Then exile it.)\x0F \\ Target opponent discards two cards. · HOU-U · Vin,Cmr,Leg,Mod"},
		{"Jace, Vryn's Prodigy", "\x02Jace, Vryn's Prodigy\x0F {1}{U} · Legendary Creature — Human Wizard · 0/2 · {T}: Draw a card, then discard a card. If there are five or more cards in your graveyard, exile Jace, Vryn's Prodigy, then return him to the battlefield transformed under his owner's control. · ORI-M · Vin,Cmr,Leg,Mod\n\x02Jace, Telepath Unbound\x0F · Legendary Planeswalker — Jace · [Blue] · [5] +1: Up to one target creature gets -2/-0 until your next turn. \\ −3: You may cast target instant or sorcery card from your graveyard this turn. If that card would be put into your graveyard this turn, exile it instead. \\ −9: You get an emblem with \"Whenever you cast a spell, target opponent puts the top five cards of their library into their graveyard.\""},
		{"Ancestral Recall", "\x02Ancestral Recall\x0F {U} · Instant · Target player draws three cards. · VMA-M · [RL] · VinRes,CmrBan,LegBan"},
		{"Crashing Footfalls", "\x02Crashing Footfalls\x0F · Sorcery · [Green] · Suspend 4—{G} \x1D(Rather than cast this card from your hand, pay {G} and exile it with four time counters on it. At the beginning of your upkeep, remove a time counter. When the last is removed, cast it without paying its mana cost.)\x0F \\ Create two 4/4 green Rhino creature tokens with trample. · MH1-R · "},
		{"Confiscate", "\x02Confiscate\x0F {4}{U}{U} · Enchantment — Aura · Enchant permanent \x1D(Target a permanent as you cast this. This card enters the battlefield attached to that permanent.)\x0F \\ You control enchanted permanent. · 9ED-U · Vin,Cmr,Leg,Mod"},
		{"Thermo-Alchemist", "\x02Thermo-Alchemist\x0F {1}{R} · Creature — Human Shaman · 0/3 · Defender \\ {T}: Thermo-Alchemist deals 1 damage to each opponent. \\ Whenever you cast an instant or sorcery spell, untap Thermo-Alchemist. · UMA-C · Vin,Cmr,Leg,Mod,Pio"},
	}
	err := importShortCardNames()
	if err != nil {
		t.Errorf("Error importing short card names: %v", err)
	}
	for _, table := range tables {
		cardname := table.cardname
		if qualifiedName, ok := shortCardNames[cardname]; ok {
			cardname = qualifiedName
		}
		fi, err := os.Open(RealCards[cardname])
		if err != nil {
			t.Errorf("Unable to open %v", RealCards[cardname])
		}
		var c Card
		if err := json.NewDecoder(fi).Decode(&c); err != nil {
			t.Errorf("Something went wrong parsing the card: %s", err)
		}
		fc := c.formatCardForIRC()
		if diff := cmp.Diff(fc, table.output); diff != "" {
			t.Errorf("Incorrect card %s (-want +got):\n%s", cardname, diff)
		}
	}
}

func TestPrintCardForSlack(t *testing.T) {
	highlanderPoints = make(map[string]int)
	err := importHighlanderPoints(false)
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	tables := []struct {
		cardname string
		output   string
	}{
		{"Ponder", "*<http://gatherer.wizards.com/Pages/Card/Details.aspx?multiverseid=451051|Ponder>* :mana-U: · Sorcery · Look at the top three cards of your library, then put them back in any order. You may shuffle your library. \\ Draw a card."},
		{"Shahrazad", "*<http://gatherer.wizards.com/Pages/Card/Details.aspx?multiverseid=980|Shahrazad>* :mana-W::mana-W: · Sorcery · Players play a Magic subgame, using their libraries as their decks. Each player who doesn't win the subgame loses half their life, rounded up. · [RL] ·"},
		{"Jace, the Mind Sculptor", "*<http://gatherer.wizards.com/Pages/Card/Details.aspx?multiverseid=442051|Jace, the Mind Sculptor>* :mana-2::mana-U::mana-U: · Legendary Planeswalker — Jace · [3] +2: Look at the top card of target player's library. You may put that card on the bottom of that player's library. \\ 0: Draw three cards, then put two cards from your hand on top of your library in any order. \\ −1: Return target creature to its owner's hand. \\ −12: Exile all cards from target player's library, then that player shuffles their hand into their library. [:point_right: 1 :point_left:]"},
		{"Expansion", "*<http://gatherer.wizards.com/Pages/Card/Details.aspx?multiverseid=452974|Expansion>* :mana-UR::mana-UR: · Instant · Copy target instant or sorcery spell with converted mana cost 4 or less. You may choose new targets for the copy.\n*<http://gatherer.wizards.com/Pages/Card/Details.aspx?multiverseid=452974|Explosion>* :mana-X::mana-U::mana-U::mana-R::mana-R: · Instant · Explosion deals X damage to any target. Target player draws X cards."},
		{"Bushi Tenderfoot", "*<http://gatherer.wizards.com/Pages/Card/Details.aspx?multiverseid=78600|Bushi Tenderfoot>* :mana-W: · Creature — Human Soldier · 1/1 · When a creature dealt damage by Bushi Tenderfoot this turn dies, flip Bushi Tenderfoot.\n*<http://gatherer.wizards.com/Pages/Card/Details.aspx?multiverseid=78600|Kenzo the Hardhearted>* · Legendary Creature — Human Samurai · 3/4 · Double strike; bushido 2 _(Whenever this creature blocks or becomes blocked, it gets +2/+2 until end of turn.)_"},
		{"Fleetwheel Cruiser", "*<http://gatherer.wizards.com/Pages/Card/Details.aspx?multiverseid=417787|Fleetwheel Cruiser>* :mana-4: · Artifact — Vehicle · 5/3 · Trample, haste \\ When Fleetwheel Cruiser enters the battlefield, it becomes an artifact creature until end of turn. \\ Crew 2 _(Tap any number of creatures you control with total power 2 or more: This Vehicle becomes an artifact creature until end of turn.)_"},
		{"Nicol Bolas, the Arisen", "*<http://gatherer.wizards.com/Pages/Card/Details.aspx?multiverseid=447354|Nicol Bolas, the Ravager>* :mana-1::mana-U::mana-B::mana-R: · Legendary Creature — Elder Dragon · 4/4 · Flying \\ When Nicol Bolas, the Ravager enters the battlefield, each opponent discards a card. \\ :mana-4::mana-U::mana-B::mana-R:: Exile Nicol Bolas, the Ravager, then return him to the battlefield transformed under his owner's control. Activate this ability only any time you could cast a sorcery.\n*<http://gatherer.wizards.com/Pages/Card/Details.aspx?multiverseid=447354|Nicol Bolas, the Arisen>* · Legendary Planeswalker — Bolas · [Blue/Black/Red] · [7] +2: Draw two cards. \\ −3: Nicol Bolas, the Arisen deals 10 damage to target creature or planeswalker. \\ −4: Put target creature or planeswalker card from a graveyard onto the battlefield under your control. \\ −12: Exile all but the bottom card of target player's library."},
		{"Dryad Arbor", "*<http://gatherer.wizards.com/Pages/Card/Details.aspx?multiverseid=136196|Dryad Arbor>* · Land Creature — Forest Dryad · 1/1 · [Green] · _(Dryad Arbor isn't a spell, it's affected by summoning sickness, and it has \":mana-T:: Add :mana-G:.\")_"},
		{"Arlinn Kord", "*<http://gatherer.wizards.com/Pages/Card/Details.aspx?multiverseid=410007|Arlinn Kord>* :mana-2::mana-R::mana-G: · Legendary Planeswalker — Arlinn · [3] +1: Until end of turn, up to one target creature gets +2/+2 and gains vigilance and haste. \\ 0: Create a 2/2 green Wolf creature token. Transform Arlinn Kord.\n*<http://gatherer.wizards.com/Pages/Card/Details.aspx?multiverseid=410007|Arlinn, Embraced by the Moon>* · Legendary Planeswalker — Arlinn · [Red/Green] · +1: Creatures you control get +1/+1 and gain trample until end of turn. \\ −1: Arlinn, Embraced by the Moon deals 3 damage to any target. Transform Arlinn, Embraced by the Moon. \\ −6: You get an emblem with \"Creatures you control have haste and ':mana-T:: This creature deals damage equal to its power to any target.'\""},
		{"Consign", "*<http://gatherer.wizards.com/Pages/Card/Details.aspx?multiverseid=430838|Consign>* :mana-1::mana-U: · Instant · Return target nonland permanent to its owner's hand.\n*<http://gatherer.wizards.com/Pages/Card/Details.aspx?multiverseid=430838|Oblivion>* :mana-4::mana-B: · Sorcery · Aftermath _(Cast this spell only from your graveyard. Then exile it.)_ \\ Target opponent discards two cards."},
		{"Jace, Vryn's Prodigy", "*<http://gatherer.wizards.com/Pages/Card/Details.aspx?multiverseid=398434|Jace, Vryn's Prodigy>* :mana-1::mana-U: · Legendary Creature — Human Wizard · 0/2 · :mana-T:: Draw a card, then discard a card. If there are five or more cards in your graveyard, exile Jace, Vryn's Prodigy, then return him to the battlefield transformed under his owner's control.\n*<http://gatherer.wizards.com/Pages/Card/Details.aspx?multiverseid=398434|Jace, Telepath Unbound>* · Legendary Planeswalker — Jace · [Blue] · [5] +1: Up to one target creature gets -2/-0 until your next turn. \\ −3: You may cast target instant or sorcery card from your graveyard this turn. If that card would be put into your graveyard this turn, exile it instead. \\ −9: You get an emblem with \"Whenever you cast a spell, target opponent puts the top five cards of their library into their graveyard.\""},
		{"Tarmogoyf", "*<http://gatherer.wizards.com/Pages/Card/Details.aspx?multiverseid=456783|Tarmogoyf>* :mana-1::mana-G: · Creature — Lhurgoyf · \xC2\xAD*/1+\xC2\xAD* · Tarmogoyf's power is equal to the number of card types among cards in all graveyards and its toughness is equal to that number plus 1."},
		{"Sarkhan the Masterless Plus", "*<https://scryfall.com/card/war/143%E2%98%85/ja/sarkhan-the-masterless?utm_source=api|Sarkhan the Masterless>* :mana-3::mana-R::mana-R: · Legendary Planeswalker — Sarkhan · [5] Whenever a creature attacks you or a planeswalker you control, each Dragon you control deals 1 damage to that creature. \\ +1: Until end of turn, each planeswalker you control becomes a 4/4 red Dragon creature and gains flying. \\ −3: Create a 4/4 red Dragon creature token with flying."},
		{"Fire//Ice No Multiverse", "*<https://scryfall.com/card/wc01/ar128/fire-ice?utm_source=api|Fire>* :mana-1::mana-R: · Instant · Fire deals 2 damage divided as you choose among one or two targets.\n*<https://scryfall.com/card/wc01/ar128/fire-ice?utm_source=api|Ice>* :mana-1::mana-U: · Instant · Tap target permanent. \\ Draw a card."},
		{"Ancestral Recall", "*<http://gatherer.wizards.com/Pages/Card/Details.aspx?multiverseid=382841|Ancestral Recall>* :mana-U: · Instant · Target player draws three cards. · [RL] · [:point_right: 5 :point_left:]"},
	}
	for _, table := range tables {
		fi, err := os.Open(RealCards[table.cardname])
		if err != nil {
			t.Errorf("Unable to open %v", RealCards[table.cardname])
		}
		var c Card
		if err := json.NewDecoder(fi).Decode(&c); err != nil {
			t.Errorf("Something went wrong parsing the card: %s", err)
		}
		fc := c.formatCardForSlack()
		if diff := cmp.Diff(fc, table.output); diff != "" {
			t.Errorf("Incorrect card (-want +got):\n%s", diff)
		}
	}
}

func TestGetReminders(t *testing.T) {
	tables := []struct {
		cardname string
		output   string
	}{
		{"Ponder", "Reminder text not found"},
		{"Poison-Tip Archer", "This creature can block creatures with flying.\nAny amount of damage this deals to a creature is enough to destroy it."},
		{"Claim", "Cast this spell only from your graveyard. Then exile it."},
		{"Faithless Looting", "You may cast this card from your graveyard for its flashback cost. Then exile it."},
	}
	for _, table := range tables {
		fi, err := os.Open(RealCards[table.cardname])
		if err != nil {
			t.Errorf("Unable to open %v", RealCards[table.cardname])
		}
		var c Card
		if err := json.NewDecoder(fi).Decode(&c); err != nil {
			t.Errorf("Something went wrong parsing the card: %s", err)
		}
		fc := c.getReminderTexts()
		if fc != table.output {
			t.Errorf("Incorrect output -- got %s -- want %s", fc, table.output)
		}
	}
}

func TestGetFlavour(t *testing.T) {
	tables := []struct {
		cardname string
		output   string
	}{
		{"Ponder", "Tomorrow belongs to those who prepare for it today."},
		{"Poison-Tip Archer", "Flavour text not found"},
		{"Cosima, God of the Voyage", "Flavour text not found"}, // DFC no flavour text at all
		{"Halvar, God of Battle", "It cuts through the Cosmos itself, carving new Omenpaths between the realms."}, // DFC flavour back only
		{"Jace, Vryn's Prodigy", `"People's thoughts just come to me. Sometimes I don't know if it's them or me thinking."`}, // DFC flavour front only
		{"Kindly Ancestor", `"You look cold, dearie." \\ "Thank you, Grandmother. I love you too."`}, // DFC flavour both sides
	}
	for _, table := range tables {
		fi, err := os.Open(RealCards[table.cardname])
		if err != nil {
			t.Errorf("Unable to open %v", RealCards[table.cardname])
		}
		var c Card
		if err := json.NewDecoder(fi).Decode(&c); err != nil {
			t.Errorf("Something went wrong parsing the card: %s", err)
		}
		fc := c.getFlavourText()
		if fc != table.output {
			t.Errorf("Incorrect output -- got %s -- want %s", fc, table.output)
		}
	}
}

func TestGetRulings(t *testing.T) {
	tables := []struct {
		input        Card
		rulingNumber int
		output       string
	}{
		{TestCardWithNoRulings, 0, "Problem fetching the rulings"},
		{TestCardWithNoRulings, 1, "Problem fetching the rulings"},
		{TestCardWithEmptyRulings, 0, "Ruling not found"},
		{TestCardWithEmptyRulings, 1, "Ruling not found"},
		{TestCardWithOneNonWOTCRuling, 0, "Ruling not found"},
		{TestCardWithOneWOTCRuling, 0, "1900-01-01: Print Me"},
		{TestCardWithOneWOTCRuling, 1, "1900-01-01: Print Me"},
		{TestCardWithTwoWOTCRulings, 0, "1900-02-02: Print Me\n1900-03-03: Print Me Too!"},
		{TestCardWithTwoWOTCRulings, 1, "1900-02-02: Print Me"},
		{TestCardWithTwoWOTCRulings, 2, "1900-03-03: Print Me Too!"},
	}

	for _, table := range tables {
		got := (table.input).getRulings(table.rulingNumber)
		if got != table.output {
			t.Errorf("Incorrect output -- got %s -- want %s", got, table.output)
		}
	}
}

func TestSortRulings(t *testing.T) {
	tables := []struct {
		cardname     string
		rulingNumber int
		output       string
	}{
		{"Tawnos's Coffin", 1, "2004-10-04: The creature returns to the battlefield tapped. It does not return to the battlefield and then tap afterwards."},
		{"Tawnos's Coffin", 2, "2007-09-16: If the targeted creature is a token, it will cease to exist after being exiled. Any Auras that were attached to it will remain exiled forever."},
		{"Tawnos's Coffin", 3, "2007-09-16: If the exiled card is returned to the battlefield and, for some reason, it now can’t be enchanted by an Aura that was also exiled by Tawnos’s Coffin, that Aura will remain exiled."},
		{"Tawnos's Coffin", 4, "2007-09-16: If Tawnos’s Coffin leaves the battlefield before its ability has resolved, it will exile the targeted creature forever, since its delayed triggered ability will never trigger."},
		{"Tawnos's Coffin", 5, "2008-04-01: Because the new wording doesn’t use phasing, the exiled card will suffer from summoning sickness upon its return to the battlefield."},
		{"Tawnos's Coffin", 6, "2008-04-01: The effect doesn’t care what types the card has after it is exiled, only that it have been a creature while on the battlefield."},
		{"Ixidron", 2, "2006-09-25: The controller of a face-down creature can look at it at any time, even if it doesn’t have morph. Other players can’t, but the rules for face-down permanents state that “you must ensure at all times that your face-down spells and permanents can be easily differentiated from each other.” As a result, all players must be able to figure out what each of the creatures Ixidron turned face down is."},
		{"Ixidron", 5, "2018-04-27: Creatures turned face down by Ixidron are 2/2 creatures with no text, no name, no subtypes, no expansion symbol, and no mana cost. These values are copiable if an object becomes a copy of one of those creatures, and their normal values are not copiable."},
		{"Mairsil, the Pretender", 1, "2017-08-25: The exiled cards remain exiled with cage counters when Mairsil leaves the battlefield. If Mairsil returns to the battlefield, it will see all of those exiled cards with cage counters on them."},
		{"Mairsil, the Pretender", 2, "2017-08-25: If another player gains control of Mairsil, it will have the abilities of only cards that player owns in exile with cage counters on them."},
		{"Mairsil, the Pretender", 12, "Ruling not found"},
	}

	for _, table := range tables {
		fi, err := os.Open(RealCards[table.cardname])
		if err != nil {
			t.Errorf("Unable to open %v", RealCards[table.cardname])
		}

		var c Card
		if err := json.NewDecoder(fi).Decode(&c); err != nil {
			t.Errorf("Something went wrong parsing the card: %s", err)
		}

		got := c.fakeGetRulings(table.rulingNumber)
		if got != table.output {
			t.Errorf("Incorrect output -- got %s -- want %s", got, table.output)
		}
	}
}

func TestGetPrintings(t *testing.T) {
	tables := []struct {
		cardname string
		output   string
	}{
		{"Faithless Looting", "\x02Faithless Looting\x0F {R} · Sorcery · Draw two cards, then discard two cards. \\ Flashback {2}{R} \x1D(You may cast this card from your graveyard for its flashback cost. Then exile it.)\x0F · CM2-C,EMA-C,PZ1-U,C15-C,C14-C,DDK-C,DKA-C,PIDW-R,UMA-C · Vin,Cmr,Leg,Mod"},
		{"Disenchant", "\x02Disenchant\x0F {1}{W} · Instant · Destroy target artifact or enchantment. · IMA-C,PRM-C,CN2-C,TPR-C,[...],A25-C · Vin,Cmr,Leg,Mod"},
		{"Ral, Storm Conduit", "\x02Ral, Storm Conduit\x0F {2}{U}{R} · Legendary Planeswalker — Ral · [4] Whenever you cast or copy an instant or sorcery spell, Ral, Storm Conduit deals 1 damage to target opponent or planeswalker. \\ +2: Scry 1. \\ −2: When you cast your next instant or sorcery spell this turn, copy that spell. You may choose new targets for the copy. · WAR-R · Vin,Cmr,Leg,Mod,Std"},
	}
	for _, table := range tables {
		fi, err := os.Open(RealCards[table.cardname])
		if err != nil {
			t.Errorf("Unable to open %v", RealCards[table.cardname])
		}
		var c Card
		if err := json.NewDecoder(fi).Decode(&c); err != nil {
			t.Errorf("Something went wrong parsing the card: %s", err)
		}
		if err = c.fakeGetMetadata(); err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		fc := c.formatCardForIRC()
		if fc != table.output {
			t.Errorf("Incorrect output -- got %s -- want %s", fc, table.output)
		}
	}
}

func TestCardCache(t *testing.T) {
	var emptyCard Card
	var err error
	nameToCardCache, err = lru.NewARC(2048)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Empty cache
	c, err := checkCacheForCard("testCardNotFound")
	if err == nil {
		t.Errorf("Unexpected non-error: %v", err)
	}
	if err.Error() != "Card not found in cache" {
		t.Errorf("Unexpected error: %v", err)
	}
	if !reflect.DeepEqual(c, emptyCard) {
		t.Errorf("Expected empty card, but got %v", c)
	}

	var faithlessLooting Card
	fi, err := os.Open(RealCards["Faithless Looting"])
	if err != nil {
		t.Errorf("Unable to open %v", RealCards["Faithless Looting"])
	}
	if err := json.NewDecoder(fi).Decode(&faithlessLooting); err != nil {
		t.Errorf("Something went wrong parsing the card: %s", err)
	}
	nameToCardCache.Add(normaliseCardName(faithlessLooting.Name), faithlessLooting)

	// Change it slightly and add it as a non-canonical
	fakeFaithless := faithlessLooting
	fakeFaithless.ManaCost = "{F}"
	nameToCardCache.Add("faithless", fakeFaithless)
	// Try the real one
	cc, err := checkCacheForCard("faithlesslooting")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if diff := cmp.Diff(cc, faithlessLooting); diff != "" {
		t.Errorf("Incorrect card (-want +got):\n%s", diff)
	}

	// Try the fake one
	cc, err = checkCacheForCard("faithless")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if diff := cmp.Diff(cc, faithlessLooting); diff != "" {
		t.Errorf("Incorrect card (-want +got):\n%s", diff)
	}
}

func TestReplaceManaCostForSlack(t *testing.T) {
	tables := []struct {
		inputmana  string
		outputmana string
	}{
		{"{R}", ":mana-R:"},
		{"{1}{B/P}{B/P}", ":mana-1::mana-BP::mana-BP:"},
	}
	for _, table := range tables {
		got := replaceManaCostForSlack(table.inputmana)
		if got != table.outputmana {
			t.Errorf("Incorrect output -- got %s -- want %s", got, table.outputmana)
		}
	}
}

func TestImportPoints(t *testing.T) {
	highlanderPoints = make(map[string]int)
	err := importHighlanderPoints(false)
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	if len(highlanderPoints) == 0 {
		t.Errorf("Empty Highlander points after import")
	}
}

func TestLangs(t *testing.T) {
	tables := []struct {
		cardname string
		lang     string
		output   string
		wanterr  bool
	}{
		{"Wicked Guardian", "pt", "Guardiã Malvada", false},
		{"Wicked Guardian", "de", "Böse Stiefmutter", false},
		{"Wicked Guardian", "abc", "", true},
	}
	for _, table := range tables {
		fi, err := os.Open(RealCards[table.cardname])
		if err != nil {
			t.Errorf("Unable to open %v", RealCards[table.cardname])
		}
		var c Card
		if err := json.NewDecoder(fi).Decode(&c); err != nil {
			t.Errorf("Something went wrong parsing the card: %s", err)
		}
		tc, err := c.fakeGetLangs(table.lang)
		if (err != nil) != table.wanterr {
			t.Errorf("Unexpected error: %v", err)
		}
		if tc.PrintedName != table.output {
			t.Errorf("Incorrect output -- got %s -- want %s", tc.Name, table.output)
		}
	}
}

func TestSearchResultHandling(t *testing.T) {
	tables := []struct {
		jsonFile string
		output   string
		wanterr  bool
	}{
		{"test_data/searchtest-warnings.json", "Invalid expression “is:slick” was ignored. Checking if cards are “slick” is not supported", true},
		{"test_data/searchtest-oneresult.json", "Gleemax", false},
		{"test_data/searchtest-toomanyresults.json", "Too many cards returned (7 > 5)", true},
	}
	for _, table := range tables {
		fi, err := os.Open(table.jsonFile)
		if err != nil {
			t.Errorf("Unable to open %v", table.jsonFile)
		}
		var csr CardSearchResult
		if err := json.NewDecoder(fi).Decode(&csr); err != nil {
			t.Errorf("Something went wrong parsing the search results: %s", err)
		}

		got, err := ParseAndFormatSearchResults(csr)
		if (err != nil) != table.wanterr {
			t.Errorf("Unexpected error: %v", err)
		} 
		if table.wanterr && err.Error() != table.output {
			t.Errorf("Incorrect output -- got %s -- want %s", err, table.output)
		}
		if len(got) > 0 && got[0].Name != table.output {
			t.Errorf("Incorrect output -- got %s -- want %s", got[0].Name, table.output)
		}
	}
}

func TestCoerceRealNamesFromForeignHiccups(t *testing.T) {
	tables := []struct {
		input  string
		output string
	}{
		{"Uro,", "Uro, Titan of Nature's Wrath" },
		{"Uro",  "Uro, Titan of Nature's Wrath" },
		{"uro",  "Uro, Titan of Nature's Wrath" },
	}
	for _, table := range tables {
		got, err := HandleForeignCardOverlapCases(table.input)
		if err != nil {
			t.Errorf("Something broke: %s", err)
		}
		if got != table.output {
			t.Errorf("Incorrect output -- got %s -- want %s", got, table.output)
		}
	}
}

func TestIsDumbCard(t *testing.T) {
	tables := []struct {
		jsonFile string
		output   bool
	}{
		{"test_data/handydandyclonemachine.json", true},
		{"test_data/thehero.json", true},
		{"test_data/ignitethecloneforge.json", true},
		{"test_data/bushitenderfoot.json", false},
		{"test_data/faithlesslooting.json", false},
	}
	for _, table := range tables {
		fi, err := os.Open(table.jsonFile)
		if err != nil {
			t.Errorf("Unable to open %v", table.jsonFile)
		}
		var card Card
		if err := json.NewDecoder(fi).Decode(&card); err != nil {
			t.Errorf("Something went wrong parsing the search results: %s", err)
		}

		got := IsDumbCard(card)
		if got != table.output {
			t.Errorf("Incorrect output for %s -- got %v -- want %v",card.Name, got, table.output)
		}
	}
}
