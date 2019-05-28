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
