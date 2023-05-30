package main

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var (
	SnapCards = map[string]string{
		"Jubilee": "test_data/snap_jubilee.json",
		"Asgard":  "test_data/snap_asgard.json",
	}
)

func TestSnapCard(t *testing.T) {
	tables := []struct {
		cardname string
		output   string
	}{
		{"Jubilee", "*<https://marvelsnap.io/card/jubilee-66|Jubilee>* - :mana-4: · ⚔️ 1 ⚔️ · On Reveal: Play a card from your deck at this location. · (_Collection Level 222-450 (Pool 2)_)"},
		{"Asgard", "*<https://marvelsnap.io/card/asgard-284|Asgard>* - After turn 4, whoever is winning here draws 2 cards."},
	}
	for _, table := range tables {
		fi, err := os.Open(SnapCards[table.cardname])
		if err != nil {
			t.Fatalf("Unable to open %v: %v", table.cardname, err)
		}
		var r SnapResponse
		err = json.NewDecoder(fi).Decode(&r)
		if err != nil {
			t.Errorf("Something went wrong parsing the response: %s", err)
		}
		fc := formatSnapCard(r.Card[0])
		if diff := cmp.Diff(fc, table.output); diff != "" {
			t.Errorf("Incorrect card (-want +got):\n%s", diff)
		}
	}
}
