package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

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
		{"702.54", "<b>702.54a.</b> Bloodthirst is a static ability. \"Bloodthirst N\" means \"If an opponent was dealt damage this turn, this permanent enters the battlefield with N +1/+1 counters on it.\""},
		{"702.23", "<b>702.23a.</b> Rampage is a triggered ability. \"Rampage N\" means \"Whenever this creature becomes blocked, it gets +N/+N until end of turn for each creature blocking it beyond the first.\" (See rule 509, \"Declare Blockers Step.\")"},
		{"def Rampage", "<b>Rampage</b>: A keyword ability that can make a creature better in combat. See rule 702.23, \"Rampage.\"\n<b>702.23a.</b> Rampage is a triggered ability. \"Rampage N\" means \"Whenever this creature becomes blocked, it gets +N/+N until end of turn for each creature blocking it beyond the first.\" (See rule 509, \"Declare Blockers Step.\")"},
		{"702.31", "<b>702.31b.</b> A creature with horsemanship can't be blocked by creatures without horsemanship. A creature with horsemanship can block a creature with or without horsemanship. (See rule 509, \"Declare Blockers Step.\")"},
		{"702.3.", "<b>702.3b.</b> A creature with defender can't attack."},
		{"702.45.", "<b>702.45a.</b> Bushido is a triggered ability. \"Bushido N\" means \"Whenever this creature blocks or becomes blocked, it gets +N/+N until end of turn.\" (See rule 509, \"Declare Blockers Step.\")"},
		{"def Bushido", "<b>Bushido</b>: A keyword ability that can make a creature better in combat. See rule 702.45, \"Bushido.\"\n<b>702.45a.</b> Bushido is a triggered ability. \"Bushido N\" means \"Whenever this creature blocks or becomes blocked, it gets +N/+N until end of turn.\" (See rule 509, \"Declare Blockers Step.\")"},
		{"702.14", "<b>702.14c.</b> A creature with landwalk can't be blocked as long as the defending player controls at least one land with the specified subtype (as in \"islandwalk\"), with the specified supertype (as in \"legendary landwalk\"), without the specified supertype (as in \"nonbasic landwalk\"), or with both the specified supertype and the specified subtype (as in \"snow swampwalk\"). (See rule 509, \"Declare Blockers Step.\")"},
		{"def Landwalk", "<b>Landwalk</b>: A generic term for a group of keyword abilities that restrict whether a creature may be blocked. See rule 702.14, \"Landwalk.\"\n<b>702.14c.</b> A creature with landwalk can't be blocked as long as the defending player controls at least one land with the specified subtype (as in \"islandwalk\"), with the specified supertype (as in \"legendary landwalk\"), without the specified supertype (as in \"nonbasic landwalk\"), or with both the specified supertype and the specified subtype (as in \"snow swampwalk\"). (See rule 509, \"Declare Blockers Step.\")"},
		{"702.57", "<b>702.57b.</b> A forecast ability may be activated only during the upkeep step of the card's owner and only once each turn. The controller of the forecast ability reveals the card with that ability from their hand as the ability is activated. That player plays with that card revealed in their hand until it leaves the player's hand or until a step or phase that isn't an upkeep step begins, whichever comes first."},
		{"def Forecast", "<b>Forecast</b>: A keyword ability that allows an activated ability to be activated from a player's hand. See rule 702.57, \"Forecast.\"\n<b>702.57b.</b> A forecast ability may be activated only during the upkeep step of the card's owner and only once each turn. The controller of the forecast ability reveals the card with that ability from their hand as the ability is activated. That player plays with that card revealed in their hand until it leaves the player's hand or until a step or phase that isn't an upkeep step begins, whichever comes first."},
		{"def Vigilance", "<b>Vigilance</b>: A keyword ability that lets a creature attack without tapping. See rule 702.20, \"Vigilance.\"\n<b>702.20b.</b> Attacking doesn't cause creatures with vigilance to tap. (See rule 508, \"Declare Attackers Step.\")"},
		{"702.20", "<b>702.20b.</b> Attacking doesn't cause creatures with vigilance to tap. (See rule 508, \"Declare Attackers Step.\")"},
		{"def Banding", "<b>Banding, \"Bands with Other\"</b>: Banding is a keyword ability that modifies the rules for declaring attackers and assigning combat damage. \"Bands with other\" is a specialized version of the ability. See rule 702.22, \"Banding.\"\n<b>702.22c.</b> As a player declares attackers, they may declare that one or more attacking creatures with banding and up to one attacking creature without banding (even if it has \"bands with other\") are all in a \"band.\" They may also declare that one or more attacking [quality] creatures with \"bands with other [quality]\" and any number of other attacking [quality] creatures are all in a band. A player may declare as many attacking bands as they want, but each creature may be a member of only one of them. (Defending players can't declare bands but may use banding in a different way; see rule 702.22j.)"},
		{"702.22.", "<b>702.22c.</b> As a player declares attackers, they may declare that one or more attacking creatures with banding and up to one attacking creature without banding (even if it has \"bands with other\") are all in a \"band.\" They may also declare that one or more attacking [quality] creatures with \"bands with other [quality]\" and any number of other attacking [quality] creatures are all in a band. A player may declare as many attacking bands as they want, but each creature may be a member of only one of them. (Defending players can't declare bands but may use banding in a different way; see rule 702.22j.)"},
		{"205.3m.", "<b>205.3m.</b> <i>[This subtype list is too long for chat. Please see https://www.vensersjournal.com/205.3m ]</i>"},
		{"define source", "<b>Source of Damage</b>: The object that dealt that damage. See rule 609.7.\n<b>609.7a.</b> If an effect requires a player to choose a source of damage, they may choose a permanent; a spell on the stack (including a permanent spell); any object referred to by an object on the stack, by a replacement or prevention effect that's waiting to apply, or by a delayed triggered ability that's waiting to trigger (even if that object is no longer in the zone it used to be in); or a face-up object in the command zone. A source doesn't need to be capable of dealing damage to be a legal choice. The source is chosen when the effect is created. If the player chooses a permanent, the effect will apply to the next damage dealt by that permanent, regardless of whether it's combat damage or damage dealt as the result of a spell or ability. If the player chooses a permanent spell, the effect will apply to any damage dealt by that spell and any damage dealt by the permanent that spell becomes when it resolves."},
		{"define mana abilities", "<b>Mana Ability</b>: An activated or triggered ability that could create mana and doesn't use the stack. See rule 605, \"Mana Abilities.\"\n<b>605.1a.</b> An activated ability is a mana ability if it meets all of the following criteria: it doesn't require a target (see rule 115.6), it could add mana to a player's mana pool when it resolves, and it's not a loyalty ability. (See rule 606, \"Loyalty Abilities.\")"},
		{"define legend rule", "<b>Legend Rule</b>: A state-based action that causes a player who controls two or more legendary permanents with the same name to put all but one into their owners' graveyards. See rule 704.5j.\n<b>704.5j.</b> If a player controls two or more legendary permanents with the same name, that player chooses one of them, and the rest are put into their owners' graveyards. This is called the \"legend rule.\""},
		{"define monarch", "<b>Monarch</b>: A designation a player can have. Some effects instruct a player to become the monarch. The monarch draws a card at the beginning of their end step. Dealing combat damage to the monarch steals the title from that player. See rule 718, \"The Monarch.\"\n<b>718.2.</b> There are two inherent triggered abilities associated with being the monarch. These triggered abilities have no source and are controlled by the player who was the monarch at the time the abilities triggered. This is an exception to rule 113.8. The full texts of these abilities are \"At the beginning of the monarch's end step, that player draws a card\" and \"Whenever a creature deals combat damage to the monarch, its controller becomes the monarch.\""},
		{"define destroy", "<b>Destroy</b>: To move a permanent from the battlefield to its owner's graveyard. See rule 701.7, \"Destroy.\"\n<b>701.7b.</b> The only ways a permanent can be destroyed are as a result of an effect that uses the word \"destroy\" or as a result of the state-based actions that check for lethal damage (see rule 704.5g) or damage from a source with deathtouch (see rule 704.5h). If a permanent is put into its owner's graveyard for any other reason, it hasn't been \"destroyed.\""},
	}
	for _, table := range tables {
		got := handleRulesQuery(table.input)
		if got != table.output {
			t.Errorf("Incorrect output --\ngot  %s\nwant %s", got, table.output)
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
		{"100.1a", "<b>100.1a.</b> A two-player game is a game that begins with only two players."},
		{"r 100.1a", "<b>100.1a.</b> A two-player game is a game that begins with only two players."},
		{"cr 100.1a", "<b>100.1a.</b> A two-player game is a game that begins with only two players."},
		{"rule 100.1a", "<b>100.1a.</b> A two-player game is a game that begins with only two players."},
		{"rule 701.29", "<b>701.29a.</b> Certain spells and abilities can detain a permanent. Until the next turn of the controller of that spell or ability, that permanent can't attack or block and its activated abilities can't be activated."},
		{"702.22a", "<b>702.22a.</b> Banding is a static ability that modifies the rules for combat."},
		{"702.22", "<b>702.22c.</b> As a player declares attackers, they may declare that one or more attacking creatures with banding and up to one attacking creature without banding (even if it has \"bands with other\") are all in a \"band.\" They may also declare that one or more attacking [quality] creatures with \"bands with other [quality]\" and any number of other attacking [quality] creatures are all in a band. A player may declare as many attacking bands as they want, but each creature may be a member of only one of them. (Defending players can't declare bands but may use banding in a different way; see rule 702.22j.)"},
		{"509.1b", "<b>509.1b.</b> The defending player checks each creature they control to see whether it's affected by any restrictions (effects that say a creature can't block, or that it can't block unless some condition is met). If any restrictions are being disobeyed, the declaration of blockers is illegal. A restriction may be created by an evasion ability (a static ability an attacking creature has that restricts what can block it). If an attacking creature gains or loses an evasion ability after a legal block has been declared, it doesn't affect that block. Different evasion abilities are cumulative."},
		{"def Absorb", "<b>Absorb</b>: A keyword ability that prevents damage. See rule 702.64, \"Absorb.\"\n<b>702.64a.</b> Absorb is a static ability. \"Absorb N\" means \"If a source would deal damage to this creature, prevent N of that damage.\""},
		{"define Absorb", "<b>Absorb</b>: A keyword ability that prevents damage. See rule 702.64, \"Absorb.\"\n<b>702.64a.</b> Absorb is a static ability. \"Absorb N\" means \"If a source would deal damage to this creature, prevent N of that damage.\""},
		{"rule Absorb", "<b>Absorb</b>: A keyword ability that prevents damage. See rule 702.64, \"Absorb.\"\n<b>702.64a.</b> Absorb is a static ability. \"Absorb N\" means \"If a source would deal damage to this creature, prevent N of that damage.\""},
		{"ex 101.2", "<b>[101.2] Example:</b> If one effect reads \"You may play an additional land this turn\" and another reads \"You can't play lands this turn,\" the effect that precludes you from playing lands wins."},
		{"ex101.2", "<b>[101.2] Example:</b> If one effect reads \"You may play an additional land this turn\" and another reads \"You can't play lands this turn,\" the effect that precludes you from playing lands wins."},
		{"example 101.2", "<b>[101.2] Example:</b> If one effect reads \"You may play an additional land this turn\" and another reads \"You can't play lands this turn,\" the effect that precludes you from playing lands wins."},
		{"ex 999.99", "Example not found"},
		{"example 999.99", "Example not found"},
		{"999.99", "Rule not found"},
		{"r 999.99", "Rule not found"},
		{"cr 999.99", "Rule not found"},
		{"rule 999.99", "Rule not found"},
		{"def CLOWNS", ""},
		{"define CLOWNS", ""},
		{"define bury", "<b>Bury</b>: A term that meant \"put [a permanent] into its owner's graveyard.\" In general, cards that were printed with the term \"bury\" have received errata in the Oracle card reference to read, \"Destroy [a permanent]. It can't be regenerated,\" or \"Sacrifice [a permanent].\""},
		{"define adsorb", "<b>Absorb</b>: A keyword ability that prevents damage. See rule 702.64, \"Absorb.\"\n<b>702.64a.</b> Absorb is a static ability. \"Absorb N\" means \"If a source would deal damage to this creature, prevent N of that damage.\""},
		{"define deaftouch", "<b>Deathtouch</b>: A keyword ability that causes damage dealt by an object to be especially effective. See rule 702.2, \"Deathtouch.\"\n<b>702.2b.</b> A creature with toughness greater than 0 that's been dealt damage by a source with deathtouch since the last time state-based actions were checked is destroyed as a state-based action. See rule 704."},
		{"define die", "<b>Dies</b>: A creature or planeswalker \"dies\" if it is put into a graveyard from the battlefield. See rule 700.4."},
		{"define detain", "<b>Detain</b>: A keyword action that temporarily stops a permanent from attacking, blocking, or having its activated abilities activated. See rule 701.29, \"Detain.\"\n<b>701.29a.</b> Certain spells and abilities can detain a permanent. Until the next turn of the controller of that spell or ability, that permanent can't attack or block and its activated abilities can't be activated."},
		{"ex 603.7a", "<b>[603.7a] Example:</b> Part of an effect reads \"When this creature leaves the battlefield,\" but the creature in question leaves the battlefield before the spell or ability creating the effect resolves. In this case, the delayed ability never triggers.\n<b>[603.7a] Example:</b> If an effect reads \"When this creature becomes untapped\" and the named creature becomes untapped before the effect resolves, the ability waits for the next time that creature untaps."},
		{"def strive", "<b>Strive</b>: Strive lets you pay additional mana to allow a spell to have additional targets. [Unofficial]"},
		{"def Strive", "<b>Strive</b>: Strive lets you pay additional mana to allow a spell to have additional targets. [Unofficial]"},
		{"def wiLL of Athe Councel", "<b>Will Of The Council</b>: Will of the Council lets players vote on an outcome and the outcome/s with the highest number of votes happens. [Unofficial]"},
		{"def cycle", "<b>Cycling</b>: A keyword ability that lets a card be discarded and replaced with a new card. See rule 702.29, \"Cycling.\"\n<b>702.29a.</b> Cycling is an activated ability that functions only while the card with cycling is in a player's hand. \"Cycling [cost]\" means \"[Cost], Discard this card: Draw a card.\""},
		{"def Active Player, Nonactive Player", "<b>Active Player, Nonactive Player Order</b>: A system that determines the order by which players make choices if multiple players are instructed to make choices at the same time. See rule 101.4. This rule is modified for games using the shared team turns option; see rule 805.6.\n<b>101.4.</b> If multiple players would make choices and/or take actions at the same time, the active player (the player whose turn it is) makes any choices required, then the next player in turn order (usually the player seated to the active player's left) makes any choices required, followed by the remaining nonactive players in turn order. Then the actions happen simultaneously. This rule is often referred to as the \"Active Player, Nonactive Player (APNAP) order\" rule."},
	}
	for _, table := range tables {
		got := handleRulesQuery(table.input)
		if diff := cmp.Diff(table.output, got); diff != "" {
			t.Errorf("Incorrect output --\ngot  %s\nwant %s", got, table.output)
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
		{"absorb", []string{"<b>Absorb</b>: A keyword ability that prevents damage. See rule 702.64, \"Absorb.\""}},
		{"ex101.2", []string{`Example: If one effect reads "You may play an additional land this turn" and another reads "You can't play lands this turn," the effect that precludes you from playing lands wins.`}},
		{"205.3i", []string{`Lands have their own unique set of subtypes; these subtypes are called land types. The land types are Desert, Forest, Gate, Island, Lair, Locus, Mine, Mountain, Plains, Power-Plant, Swamp, Tower, and Urza's.`, ` Of that list, Forest, Island, Mountain, Plains, and Swamp are the basic land types. See rule 305.6.`}},
		{"205.4c", []string{`Any land with the supertype "basic" is a basic land. Any land that doesn't have this supertype is a nonbasic land, even if it has a basic land type.`, ` Cards printed in sets prior to the Eighth Edition core set didn't use the word "basic" to indicate a basic land. Cards from those sets with the following names are basic lands and have received errata in the Oracle card reference accordingly: Forest, Island, Mountain, Plains, Swamp, Snow-Covered Forest, Snow-Covered Island, Snow-Covered Mountain, Snow-Covered Plains, and Snow-Covered Swamp.`}},
		{"509.1b", []string{`The defending player checks each creature they control to see whether it's affected by any restrictions (effects that say a creature can't block, or that it can't block unless some condition is met). If any restrictions are being disobeyed, the declaration of blockers is illegal.`, ` A restriction may be created by an evasion ability (a static ability an attacking creature has that restricts what can block it). If an attacking creature gains or loses an evasion ability after a legal block has been declared, it doesn't affect that block. Different evasion abilities are cumulative.`}},
	}
	for _, table := range tables {
		got := rules[table.input]
		if diff := cmp.Diff(got, table.output); diff != "" {
			t.Errorf("Incorrect output --\ngot  %s\nwant %s", got, table.output)
		}
	}
}
