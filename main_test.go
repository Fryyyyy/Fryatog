package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	hbot "github.com/whyrusleeping/hellabot"
)

func fakeGetCard(cardname string) (Card, error) {
	r := rand.Intn(1000)
	fmt.Printf("Trying to get card %v -- Sleeping %v ms\n", cardname, r)
	time.Sleep(time.Duration(r) * time.Millisecond)
	for _, c := range FakeCards {
		if cardname == c.Name {
			return c, nil
		}
	}
	for k, v := range RealCards {
		if cardname == k {
			var c Card
			fi, err := os.Open(v)
			if err != nil {
				return c, fmt.Errorf("Unable to open card JSON: %s", err)
			}
			if err := json.NewDecoder(fi).Decode(&c); err != nil {
				return c, fmt.Errorf("Something went wrong parsing the card: %s", err)
			}
			return c, nil
		}
	}
	return Card{Name: "CARD", Set: "TestSet", Rarity: "TestRare", ID: cardname}, nil
}

func fakeGetRandomCard() (Card, error) {
	return Card{Name: "RANDOMCARD", Set: "RandomTestSet", Rarity: "RandomTestRare", ID: "randomCard"}, nil
}

func fakeFindCards(tokens []string) ([]Card, error) {
	card1, _ := fakeGetRandomCard()
	card2, _ := fakeGetRandomCard()
	return []Card{card1, card2}, nil
}

func TestNormaliseCardName(t *testing.T) {
	tables := []struct {
		input  string
		output string
	}{
		{"Jace, the Mind Sculptor", "jacethemindsculptor"},
		{"ponder", "ponder"},
	}
	for _, table := range tables {
		got := normaliseCardName(table.input)
		if got != table.output {
			t.Errorf("Incorrect output -- got %s -- want %s", got, table.output)
		}
	}
}

func TestTokens(t *testing.T) {
	// Clear and import rules
	rules = make(map[string][]string)
	err := importRules(false)

	if err != nil {
		t.Errorf("Didn't expect an error -- got %v", err)
	}
	var emptyStringSlice []string
	var testCardExpected = "\x02CARD\x0F ·  · · TESTSET-T · "
	var testRandomCardExpected = "\x02RANDOMCARD\x0F ·  · · RANDOMTESTSET-R · "
	tables := []struct {
		input  string
		output []string
	}{
		{"Hello! ", emptyStringSlice},
		{"!  ", emptyStringSlice},
		{"Test!", emptyStringSlice},
		{"'Test!'", emptyStringSlice},
		{"What?!? Why does that work", emptyStringSlice},
		{"<Bird12> Just making sure, thank you!!!!", emptyStringSlice},
		{"<Cyclops7> Thank you!! I have one more question kind of in the same realm-- if I want to bring some tokens with me to the same event, am I allowed to keep them in the deckbox with my deck and sideboard, or do I have to keep them someplace else?", emptyStringSlice},
		{"<+mtgrelay> [Fear12] Hi!! Quick question: Does Sundial of the Infinite bypass/combo with Psychic Vortex?", emptyStringSlice},
		{"<+mtgrelay> [Fear12] Hi!! Quick question: Does !Sundial of the Infinite bypass/combo with !Psychic Vortex?", []string{testCardExpected, testCardExpected}},
		{"<MW> !!fract ident &treas nabb", []string{testCardExpected, testCardExpected}},
		{"!cr 100.1a", []string{"\x02100.1a.\x0F A two-player game is a game that begins with only two players."}},
		{"!100.1a !!hello", []string{"\x02100.1a.\x0F A two-player game is a game that begins with only two players.", testCardExpected}},
		{`Animate dead ETBing is a trigger. The *entire* trigger resolves like this: "Bring back Karmic guide. Fail to attach to Karmic Guide." State-based actions check and go "that's an aura not attached to anything!" and sends Animate Dead to the graveyard`, emptyStringSlice},
		{"!one&two&three", []string{testCardExpected, testCardExpected, testCardExpected}},
		{"!\"testquote\"", []string{testCardExpected}},
		{"\"!testquote\"", []string{testCardExpected}},
		{"[[One]]", []string{testCardExpected}},
		{"[[One]] [[Two]]", []string{testCardExpected, testCardExpected}},
		{"[[One]]:[[Two]]", []string{testCardExpected, testCardExpected}},
		{"Hello there! I have a question about [[Multani, Yavimaya's Avatar]]: Can you activate her ability with her being on the battlefield?", []string{testCardExpected}},
		{"Hello there!lightning bolt", []string{testCardExpected}},
		{`Hello there!"lightning bolt"`, []string{testCardExpected}},
		{"So what is the right talking to my opponent ( first thank you very much !) To avoid judge calling", emptyStringSlice},
		{"To", []string{""}},
		{"Too", []string{testCardExpected}},
		{"random", []string{testRandomCardExpected}},
		{"!rule of law", []string{testCardExpected}},
		// {"Hello! I was wondering if Selvala, Explorer Returned flip triggers work. If I use Selvala and two nonlands are revealed, is that two triggers of life & mana gain", emptyStringSlice}, -- WONTFIX https://github.com/Fryyyyy/Fryatog/issues/42
		{"!search o:test", []string{testRandomCardExpected + "\n" + testRandomCardExpected}},
	}
	for _, table := range tables {
		got := tokeniseAndDispatchInput(&fryatogParams{m: &hbot.Message{Content: table.input}}, fakeGetCard, fakeGetRandomCard, fakeFindCards)
		if !reflect.DeepEqual(got, table.output) {
			t.Errorf("Incorrect output for [%v] -- got %s -- want %s", table.input, got, table.output)
		}
	}
}

func TestRules(t *testing.T) {
	// Clear and import rules
	rules = make(map[string][]string)
	err := importRules(false)

	if err != nil {
		t.Errorf("Didn't expect an error -- got %v", err)
	}
	tables := []struct {
		input  string
		output []string
	}{
		{"100.1a", []string{"A two-player game is a game that begins with only two players."}},
		{"absorb", []string{"\x02Absorb\x0F: A keyword ability that prevents damage. See rule 702.63, \"Absorb.\""}},
		{"ex101.2", []string{`Example: If one effect reads "You may play an additional land this turn" and another reads "You can't play lands this turn," the effect that precludes you from playing lands wins.`}},
		{"205.3i", []string{`Lands have their own unique set of subtypes; these subtypes are called land types. The land types are Desert, Forest, Gate, Island, Lair, Locus, Mine, Mountain, Plains, Power-Plant, Swamp, Tower, and Urza's.`, ` Of that list, Forest, Island, Mountain, Plains, and Swamp are the basic land types. See rule 305.6.`}},
		{"205.4c", []string{`Any land with the supertype "basic" is a basic land. Any land that doesn't have this supertype is a nonbasic land, even if it has a basic land type.`, ` Cards printed in sets prior to the Eighth Edition core set didn't use the word "basic" to indicate a basic land. Cards from those sets with the following names are basic lands and have received errata in the Oracle card reference accordingly: Forest, Island, Mountain, Plains, Swamp, Snow-Covered Forest, Snow-Covered Island, Snow-Covered Mountain, Snow-Covered Plains, and Snow-Covered Swamp.`}},
		{"509.1b", []string{`The defending player checks each creature they control to see whether it's affected by any restrictions (effects that say a creature can't block, or that it can't block unless some condition is met). If any restrictions are being disobeyed, the declaration of blockers is illegal.`, ` A restriction may be created by an evasion ability (a static ability an attacking creature has that restricts what can block it). If an attacking creature gains or loses an evasion ability after a legal block has been declared, it doesn't affect that block. Different evasion abilities are cumulative.`}},
	}
	for _, table := range tables {
		got := rules[table.input]
		if diff := cmp.Diff(got, table.output); diff != "" {
			t.Errorf("Incorrect output (-want +got):\n%s", diff)
		}
	}
}

func TestGetRule(t *testing.T) {
	// Clear and import rules
	rules = make(map[string][]string)
	err := importRules(false)
	if err != nil {
		t.Errorf("Didn't expect an error -- got %v", err)
	}
	err = importAbilityWords()
	if err != nil {
		t.Errorf("Didn't expect an error -- got %v", err)
	}
	tables := []struct {
		input  string
		output string
	}{
		{"100.1a", "\x02100.1a.\x0F A two-player game is a game that begins with only two players."},
		{"r 100.1a", "\x02100.1a.\x0F A two-player game is a game that begins with only two players."},
		{"cr 100.1a", "\x02100.1a.\x0F A two-player game is a game that begins with only two players."},
		{"rule 100.1a", "\x02100.1a.\x0F A two-player game is a game that begins with only two players."},
		{"rule 701.28", "\x02701.28a.\x0F Certain spells and abilities can detain a permanent. Until the next turn of the controller of that spell or ability, that permanent can't attack or block and its activated abilities can't be activated."},
		{"702.21a", "\x02702.21a.\x0F Banding is a static ability that modifies the rules for combat."},
		{"702.21", "\x02702.21c.\x0F As a player declares attackers, they may declare that one or more attacking creatures with banding and up to one attacking creature without banding (even if it has \"bands with other\") are all in a \"band.\" They may also declare that one or more attacking [quality] creatures with \"bands with other [quality]\" and any number of other attacking [quality] creatures are all in a band. A player may declare as many attacking bands as they want, but each creature may be a member of only one of them. (Defending players can't declare bands but may use banding in a different way; see rule 702.21j.)"},
		{"509.1b", "\x02509.1b.\x0F The defending player checks each creature they control to see whether it's affected by any restrictions (effects that say a creature can't block, or that it can't block unless some condition is met). If any restrictions are being disobeyed, the declaration of blockers is illegal. A restriction may be created by an evasion ability (a static ability an attacking creature has that restricts what can block it). If an attacking creature gains or loses an evasion ability after a legal block has been declared, it doesn't affect that block. Different evasion abilities are cumulative."},
		{"def Absorb", "\x02Absorb\x0F: A keyword ability that prevents damage. See rule 702.63, \"Absorb.\"\n\x02702.63a.\x0F Absorb is a static ability. \"Absorb N\" means \"If a source would deal damage to this creature, prevent N of that damage.\""},
		{"define Absorb", "\x02Absorb\x0F: A keyword ability that prevents damage. See rule 702.63, \"Absorb.\"\n\x02702.63a.\x0F Absorb is a static ability. \"Absorb N\" means \"If a source would deal damage to this creature, prevent N of that damage.\""},
		{"rule Absorb", "\x02Absorb\x0F: A keyword ability that prevents damage. See rule 702.63, \"Absorb.\"\n\x02702.63a.\x0F Absorb is a static ability. \"Absorb N\" means \"If a source would deal damage to this creature, prevent N of that damage.\""},
		{"ex 101.2", "\x02[101.2] Example:\x0F If one effect reads \"You may play an additional land this turn\" and another reads \"You can't play lands this turn,\" the effect that precludes you from playing lands wins."},
		{"ex101.2", "\x02[101.2] Example:\x0F If one effect reads \"You may play an additional land this turn\" and another reads \"You can't play lands this turn,\" the effect that precludes you from playing lands wins."},
		{"example 101.2", "\x02[101.2] Example:\x0F If one effect reads \"You may play an additional land this turn\" and another reads \"You can't play lands this turn,\" the effect that precludes you from playing lands wins."},
		{"ex 999.99", "Example not found"},
		{"example 999.99", "Example not found"},
		{"999.99", "Rule not found"},
		{"r 999.99", "Rule not found"},
		{"cr 999.99", "Rule not found"},
		{"rule 999.99", "Rule not found"},
		{"def CLOWNS", ""},
		{"define CLOWNS", ""},
		{"define adsorb", "\x02Absorb\x0F: A keyword ability that prevents damage. See rule 702.63, \"Absorb.\"\n\x02702.63a.\x0F Absorb is a static ability. \"Absorb N\" means \"If a source would deal damage to this creature, prevent N of that damage.\""},
		{"define deaftouch", "\x02Deathtouch\x0F: A keyword ability that causes damage dealt by an object to be especially effective. See rule 702.2, \"Deathtouch.\"\n\x02702.2b.\x0F A creature with toughness greater than 0 that's been dealt damage by a source with deathtouch since the last time state-based actions were checked is destroyed as a state-based action. See rule 704."},
		{"define die", "\x02Dies\x0F: A creature or planeswalker \"dies\" if it is put into a graveyard from the battlefield. See rule 700.4."},
		{"define detain", "\x02Detain\x0F: A keyword action that temporarily stops a permanent from attacking, blocking, or having its activated abilities activated. See rule 701.28, \"Detain.\"\n\x02701.28a.\x0F Certain spells and abilities can detain a permanent. Until the next turn of the controller of that spell or ability, that permanent can't attack or block and its activated abilities can't be activated."},
		{"ex 603.7a", "\x02[603.7a] Example:\x0F Part of an effect reads \"When this creature leaves the battlefield,\" but the creature in question leaves the battlefield before the spell or ability creating the effect resolves. In this case, the delayed ability never triggers.\n\x02[603.7a] Example:\x0F If an effect reads \"When this creature becomes untapped\" and the named creature becomes untapped before the effect resolves, the ability waits for the next time that creature untaps."},
		{"def strive", "\x02Strive\x0F: Strive lets you pay additional mana to allow a spell to have additional targets. [Unofficial]"},
		{"def Strive", "\x02Strive\x0F: Strive lets you pay additional mana to allow a spell to have additional targets. [Unofficial]"},
		{"def wiLL of Athe Councel", "\x02Will Of The Council\x0F: Will of the Council lets players vote on an outcome and the outcome/s with the highest number of votes happens. [Unofficial]"},
	}
	for _, table := range tables {
		got := handleRulesQuery(table.input)
		if diff := cmp.Diff(table.output, got); diff != "" {
			t.Errorf("Incorrect output (-want +got):\n%s", diff)
		}
	}
}

func TestCardMetadata(t *testing.T) {
	tables := []struct {
		command string
		message string
		output  string
	}{
		{"ruling", "ruling TestCardWithOneWOTCRuling 1", "1900-01-01: Print Me"},
		{"ruling", "ruling TestCardWithOneWOTCRuling", "1900-01-01: Print Me"},
		{"ruling", "ruling TestCardWithOneNonWOTCRuling 1", "Ruling not found"},
		{"ruling", "ruling TestCardWithOneNonWOTCRuling", "Ruling not found"},
		{"reminder", "reminder Ponder", "Reminder text not found"},
		{"reminder", "reminder Faithless Looting", "You may cast this card from your graveyard for its flashback cost. Then exile it."},
		{"reminder", "reminder Poison-Tip Archer", "This creature can block creatures with flying.\nAny amount of damage this deals to a creature is enough to destroy it."},
		{"flavour", "flavour Ponder", "Tomorrow belongs to those who prepare for it today."},
		{"flavor", "flavor Faithless Looting", "\"Avacyn has abandoned us! We have nothing left except what we can take!\""},
		{"flavor", "flavor Bushi Tenderfoot", "Flavour text not found"},
	}
	for _, table := range tables {
		got := handleCardMetadataQuery(&fryatogParams{message: table.message, cardGetFunction: fakeGetCard}, table.command)
		if got != table.output {
			t.Errorf("Incorrect output -- got %s - want %s", got, table.output)
		}
	}
}

func TestHelp(t *testing.T) {
	got := printHelp()
	if !strings.Contains(got, "!cardname") {
		t.Errorf("Incorrect output -- got %s - want %s", got, "!cardname")
	}
}

func TestBetterGetRule(t *testing.T) {
	// Clear and import rules
	rules = make(map[string][]string)
	err := importRules(false)
	if err != nil {
		t.Errorf("Didn't expect an error -- got %v", err)
	}
	tables := []struct {
		input  string
		output string
	}{
		{"702.53", "\x02702.53a.\x0F Bloodthirst is a static ability. \"Bloodthirst N\" means \"If an opponent was dealt damage this turn, this permanent enters the battlefield with N +1/+1 counters on it.\""},
		{"702.22", "\x02702.22a.\x0F Rampage is a triggered ability. \"Rampage N\" means \"Whenever this creature becomes blocked, it gets +N/+N until end of turn for each creature blocking it beyond the first.\" (See rule 509, \"Declare Blockers Step.\")"},
		{"def Rampage", "\x02Rampage\x0F: A keyword ability that can make a creature better in combat. See rule 702.22, \"Rampage.\"\n\x02702.22a.\x0F Rampage is a triggered ability. \"Rampage N\" means \"Whenever this creature becomes blocked, it gets +N/+N until end of turn for each creature blocking it beyond the first.\" (See rule 509, \"Declare Blockers Step.\")"},
		{"702.30", "\x02702.30b.\x0F A creature with horsemanship can't be blocked by creatures without horsemanship. A creature with horsemanship can block a creature with or without horsemanship. (See rule 509, \"Declare Blockers Step.\")"},
		{"702.3.", "\x02702.3b.\x0F A creature with defender can't attack."},
		{"702.44.", "\x02702.44a.\x0F Bushido is a triggered ability. \"Bushido N\" means \"Whenever this creature blocks or becomes blocked, it gets +N/+N until end of turn.\" (See rule 509, \"Declare Blockers Step.\")"},
		{"def Bushido", "\x02Bushido\x0F: A keyword ability that can make a creature better in combat. See rule 702.44, \"Bushido.\"\n\x02702.44a.\x0F Bushido is a triggered ability. \"Bushido N\" means \"Whenever this creature blocks or becomes blocked, it gets +N/+N until end of turn.\" (See rule 509, \"Declare Blockers Step.\")"},
		{"702.14", "\x02702.14c.\x0F A creature with landwalk can't be blocked as long as the defending player controls at least one land with the specified subtype (as in \"islandwalk\"), with the specified supertype (as in \"legendary landwalk\"), without the specified supertype (as in \"nonbasic landwalk\"), or with both the specified supertype and the specified subtype (as in \"snow swampwalk\"). (See rule 509, \"Declare Blockers Step.\")"},
		{"def Landwalk", "\x02Landwalk\x0F: A generic term for a group of keyword abilities that restrict whether a creature may be blocked. See rule 702.14, \"Landwalk.\"\n\x02702.14c.\x0F A creature with landwalk can't be blocked as long as the defending player controls at least one land with the specified subtype (as in \"islandwalk\"), with the specified supertype (as in \"legendary landwalk\"), without the specified supertype (as in \"nonbasic landwalk\"), or with both the specified supertype and the specified subtype (as in \"snow swampwalk\"). (See rule 509, \"Declare Blockers Step.\")"},
		{"702.56", "\x02702.56b.\x0F A forecast ability may be activated only during the upkeep step of the card's owner and only once each turn. The controller of the forecast ability reveals the card with that ability from their hand as the ability is activated. That player plays with that card revealed in their hand until it leaves the player's hand or until a step or phase that isn't an upkeep step begins, whichever comes first."},
		{"def Forecast", "\x02Forecast\x0F: A keyword ability that allows an activated ability to be activated from a player's hand. See rule 702.56, \"Forecast.\"\n\x02702.56b.\x0F A forecast ability may be activated only during the upkeep step of the card's owner and only once each turn. The controller of the forecast ability reveals the card with that ability from their hand as the ability is activated. That player plays with that card revealed in their hand until it leaves the player's hand or until a step or phase that isn't an upkeep step begins, whichever comes first."},
		{"def Vigilance", "\x02Vigilance\x0F: A keyword ability that lets a creature attack without tapping. See rule 702.20, \"Vigilance.\"\n\x02702.20b.\x0F Attacking doesn't cause creatures with vigilance to tap. (See rule 508, \"Declare Attackers Step.\")"},
		{"702.20", "\x02702.20b.\x0F Attacking doesn't cause creatures with vigilance to tap. (See rule 508, \"Declare Attackers Step.\")"},
		{"def Banding", "\x02Banding, \"Bands with Other\"\x0F: Banding is a keyword ability that modifies the rules for declaring attackers and assigning combat damage. \"Bands with other\" is a specialized version of the ability. See rule 702.21, \"Banding.\"\n\x02702.21c.\x0F As a player declares attackers, they may declare that one or more attacking creatures with banding and up to one attacking creature without banding (even if it has \"bands with other\") are all in a \"band.\" They may also declare that one or more attacking [quality] creatures with \"bands with other [quality]\" and any number of other attacking [quality] creatures are all in a band. A player may declare as many attacking bands as they want, but each creature may be a member of only one of them. (Defending players can't declare bands but may use banding in a different way; see rule 702.21j.)"},
		{"702.21.", "\x02702.21c.\x0F As a player declares attackers, they may declare that one or more attacking creatures with banding and up to one attacking creature without banding (even if it has \"bands with other\") are all in a \"band.\" They may also declare that one or more attacking [quality] creatures with \"bands with other [quality]\" and any number of other attacking [quality] creatures are all in a band. A player may declare as many attacking bands as they want, but each creature may be a member of only one of them. (Defending players can't declare bands but may use banding in a different way; see rule 702.21j.)"},
	}
	for _, table := range tables {
		got := handleRulesQuery(table.input)
		if got != table.output {
			t.Errorf("Incorrect output -- got %s - want %s", got, table.output)
		}
	}
}

func TestAbilityWords(t *testing.T) {
	err := importAbilityWords()
	if err != nil {
		t.Errorf("Didn't expect an error -- got %v", err)
	}
	if len(abilityWords) < 30 {
		t.Errorf("Too few ability words -- got %v", len(abilityWords))
	}
	if len(abilityWords["strive"]) < 30 {
		t.Errorf("Length of e.g strive was too short -- got %v", len(abilityWords["strive"]))
	}
}
