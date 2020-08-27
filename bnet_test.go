package main

import (
	"testing"

	"github.com/FuzzyStatic/blizzard/wowgd"
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
