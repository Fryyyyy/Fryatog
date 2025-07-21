package main

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var (
	HSCards = map[string]string{
		"Lich King":       "test_data/hs_lichking.json",
		"Topsy Turvy":     "test_data/hs_topsyturvy.json",
		"Arcanite Reaper": "test_data/hs_arcanitereaper.json",
	}
)

func TestGetCard(t *testing.T) {
	tables := []struct {
		cardname string
		output   string
	}{
		{"Lich King", `*<http://www.hearthpwn.com//cards/62922-the-lich-king|The Lich King>* · {8} · Minion · 8/8 · *Taunt* At the end of your turn, add a random *Death Knight* card to your hand. · _"All that I am: anger, cruelty, vengeance, 8 attack - I bestow upon you, my chosen knight."_ · ICECROWN-L`},
		{"Topsy Turvy", `*<http://www.hearthpwn.com//cards/89924-topsy-turvy|Topsy Turvy>* · {0} · Spell · Swap a minion's Attack and Health. · _Help! I've fallen and I can't get down!_ · BOOMSDAY-C`},
		{"Arcanite Reaper", `*<http://www.hearthpwn.com//cards/182-arcanite-reaper|Arcanite Reaper>* · {5} · Weapon · 5/2 · _No… actually you should fear the Reaper._ · CORE-F`},
	}
	for _, table := range tables {
		fi, err := os.ReadFile(HSCards[table.cardname])
		if err != nil {
			t.Errorf("Unable to open %v", RealCards[table.cardname])
		}
		var objmap interface{}
		err = json.Unmarshal(fi, &objmap)
		if err != nil {
			t.Errorf("Something went wrong parsing the card: %s", err)
		}
		fc := formatHSCard(objmap.(map[string]interface{}))
		if diff := cmp.Diff(fc, table.output); diff != "" {
			t.Errorf("Incorrect card (-want +got):\n%s", diff)
		}
	}
}
