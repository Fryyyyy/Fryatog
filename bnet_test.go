package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/FuzzyStatic/blizzard/wowgd"
	"github.com/FuzzyStatic/blizzard/wowp"
)

func TestDistinguishRealmFromPlayer(t *testing.T) {
	wowRealms = &wowgd.RealmIndex{
		Realms: []struct {
			Key struct {
				Href string `json:"href"`
			} `json:"key"`
			Name string `json:"name"`
			ID   int    `json:"id"`
			Slug string `json:"slug"`
		}{
			{Name: "TestRealm", Slug: "testrealm"},
		},
	}
	type args struct {
		input1 string
		input2 string
	}
	tests := []struct {
		name       string
		args       args
		wantPlayer string
		wantRealm  string
		wantErr    bool
	}{
		{"realm, player", args{"testrealm", "testplayer"}, "testplayer", "testrealm", false},
		{"player, realm", args{"testplayer", "testrealm"}, "testplayer", "testrealm", false},
		{"notrealm, player", args{"fakerealm", "testplayer"}, "", "", true},
		{"player, notrealm", args{"testplayer", "fakerealm"}, "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRealm, gotPlayer, err := distinguishRealmFromPlayer(tt.args.input1, tt.args.input2)
			if (err != nil) != tt.wantErr {
				t.Errorf("distinguishRealmFromPlayer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotPlayer != tt.wantPlayer {
				t.Errorf("distinguishRealmFromPlayer() player got = %v, want %v", gotPlayer, tt.wantPlayer)
			}
			if gotRealm != tt.wantRealm {
				t.Errorf("distinguishRealmFromPlayer() realm got = %v, want %v", gotRealm, tt.wantRealm)
			}
		})
	}
}

func TestPlayerSingleChieveStatus(t *testing.T) {
	var cas wowp.CharacterAchievementsSummary
	fi, err := os.Open("test_data/wowcharacterchieve.json")
	if err != nil {
		t.Errorf("Unable to open JSON: %v", err)
	}
	if err := json.NewDecoder(fi).Decode(&cas); err != nil {
		t.Errorf("Something went wrong parsing the card list")
	}
	type args struct {
		cas        *wowp.CharacterAchievementsSummary
		chieveName string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"hasSimpleChieve", args{&cas, "Level 10"}, "Achievement Unlocked"},
		{"doesntHaveSimpleChieve", args{&cas, "Split Personality"}, "not got"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := playerSingleChieveStatus(tt.args.cas, tt.args.chieveName); !strings.Contains(got, tt.want) {
				t.Errorf("playerSingleChieveStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatChieveForSlack(t *testing.T) {
	var a wowgd.Achievement
	fi, err := os.Open("test_data/wowsinglechieve.json")
	if err != nil {
		t.Errorf("Unable to open JSON: %v", err)
	}
	if err := json.NewDecoder(fi).Decode(&a); err != nil {
		t.Errorf("Something went wrong parsing the card list")
	}
	type args struct {
		a *wowgd.Achievement
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"pathfinder", args{&a}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatChieveForSlack(tt.args.a); got != tt.want {
				t.Errorf("formatChieveForSlack() = %v, want %v", got, tt.want)
			}
		})
	}
}
