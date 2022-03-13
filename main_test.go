package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	hbot "github.com/whyrusleeping/hellabot"
)

func fakeGetCard(cardname string, isLang bool) (Card, error) {
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
			fmt.Printf("In FakeGetCard: %v %v\n", c.Name, c.Lang)
			if c.Lang != "en" && !isLang {
				return fakeGetCard(c.Name, false)
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

func fakeFindRealCard(tokens []string) ([]string, error) {
	var csr CardSearchResult
	var ret []string
	fi, err := os.Open("test_data/" + normaliseCardName(strings.Join(tokens, " ")) + "-searchresult.json")
	if err != nil {
		return []string{}, fmt.Errorf("Unable to open card JSON: %s", err)
	}
	if err := json.NewDecoder(fi).Decode(&csr); err != nil {
		return nil, fmt.Errorf("Something went wrong parsing the card search results")
	}
	minLen := min(2, len(csr.Data))
	for _, c := range csr.Data[0:minLen] {
		ret = append(ret, c.formatCardForIRC())
	}
	return ret, nil
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

func TestPolicy(t *testing.T) {
	tables := []struct {
		input  string
		output string
	}{
		{"mtr 4.8 is probably what you'll want to look at", "https://blogs.magicjudges.org/rules/mtr4-8"},
		{"ipg", "https://blogs.magicjudges.org/rules/ipg"},
		{"ipg 3.2", "https://blogs.magicjudges.org/rules/ipg3-2"},
		{"url jar", "http://jar.mtgipg.com"},
		{"url cr", "http://cr.mtgipg.com"},
	}

	for _, table := range tables {
		strArray := strings.Fields(table.input)
		got := handlePolicyQuery(strArray)
		if got != table.output {
			t.Errorf("Policy test fail\nWANTED\n%s\nGOT\n%s", table.output, got)
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
		{"Player != Planeswalker", emptyStringSlice},
		{"Trying to bring = up a !Planeswalker =card", []string{testCardExpected}},
		{"https://scryfall.com/search?q=cmc%3D9+f%3Avintage&unique=cards&as=grid&order=name It can go grab any of this fun stuff!", emptyStringSlice},
		{"B&R today!", emptyStringSlice},
		{"!wc WrongPlaceUser", []string{"WrongPlaceUser: Rules questions belong in the rules channel, not in here. Click #magicjudges-rules or type '/join #magicjudges-rules' (without the quotes) to get there"}},
		{"!wc", []string{"Rules questions belong in the rules channel, not in here. Click #magicjudges-rules or type '/join #magicjudges-rules' (without the quotes) to get there"}},
	}
	for _, table := range tables {
		got := tokeniseAndDispatchInput(&fryatogParams{m: &hbot.Message{Content: table.input}}, fakeGetCard, fakeGetCard, fakeGetRandomCard, fakeFindCards)
		sort.Strings(got)
		sort.Strings(table.output)
		if !reflect.DeepEqual(got, table.output) {
			t.Errorf("Incorrect output for [%v] -- got %q -- want %q", table.input, got, table.output)
		}
	}
}

func TestLanguageRecursion(t *testing.T) {
	tables := []struct {
		input  string
		output []string
	}{
		{"Erebos' Titan", []string{"\x02Erebos's Titan\x0f {1}{B}{B}{B} · Creature — Giant · 5/5 · As long as your opponents control no creatures, Erebos's Titan has indestructible. \x1d(Damage and effects that say \"destroy\" don't destroy it.)\x0f \\ Whenever a creature card leaves an opponent's graveyard, you may discard a card. If you do, return Erebos's Titan from your graveyard to your hand. · ORI-M · Vin,Cmr,Leg,Mod,Pio"}},
	}
	for _, table := range tables {
		got := tokeniseAndDispatchInput(&fryatogParams{m: &hbot.Message{Content: table.input}}, fakeGetCard, fakeGetCard, fakeGetRandomCard, fakeFindCards)
		if !reflect.DeepEqual(got, table.output) {
			t.Errorf("Incorrect output for [%v] -- got %q -- want %q", table.input, got, table.output)
		}
	}
}

func TestRegex(t *testing.T) {
	tables := []struct {
		input       string
		wantMatch   bool
		matchGroups []string
	}{
		{"!search pow=0 tou=17", true, []string{"!search pow=0 tou=17"}},
		{"Player != Planeswalker", false, []string{}},
		{"<MW> !!fract ident &treas nabb", true, []string{"!fract ident ", "&treas nabb"}},
		{"!Fork. it creates", true, []string{"!Fork. "}},
	}
	for _, table := range tables {
		got := botCommandRegex.FindAllString(table.input, -1)
		if table.wantMatch && !reflect.DeepEqual(got, table.matchGroups) {
			t.Errorf("%v didn't match as expected -- got %q -- want %q", table.input, got, table.matchGroups)
		}
		if !table.wantMatch && len(table.matchGroups) > 0 {
			t.Errorf("%v should not have matched, but did: %q", table.input, got)
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

func TestCardSearchRealResults(t *testing.T) {
	tables := []struct {
		input  []string
		output []string
	}{
		{[]string{"Search for Azcanta"}, []string{"\x02Search for Azcanta\x0F {1}{U} · Legendary Enchantment · At the beginning of your upkeep, look at the top card of your library. You may put it into your graveyard. Then if you have seven or more cards in your graveyard, you may transform Search for Azcanta. · XLN-R · Vin,Cmr,Leg,Mod,Std\n\x02Azcanta, the Sunken Ruin\x0F · Legendary Land · (Transforms from Search for Azcanta.) \\ {T}: Add {U}. \\ {2}{U}, {T}: Look at the top four cards of your library. You may reveal a noncreature, nonland card from among them and put it into your hand. Put the rest on the bottom of your library in any order."}},
	}
	for _, table := range tables {
		got, err := fakeFindRealCard(table.input)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		} else if !reflect.DeepEqual(got, table.output) {
			t.Errorf("Incorrect output -- got %s - want %s", got, table.output)
		}
	}
}
