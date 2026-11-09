package model

import (
	"encoding/json"
	"testing"
)

func TestLegacyCredentialsKeepTheirDestinations(t *testing.T) {
	for _, ch := range Channels {
		var c Credentials
		if err := json.Unmarshal([]byte(`{"webhook":"https://existing.invalid","token":"existing-token","chat_id":"12345"}`), &c); err != nil {
			t.Fatal(err)
		}
		c.Normalize(ch)
		if ch == Telegram {
			if c.Mode != Direct || c.RecipientID != "12345" {
				t.Fatal("legacy private chat lost")
			}
		} else if c.Mode != Group || c.Webhook != "https://existing.invalid" {
			t.Fatal("legacy webhook rerouted")
		}
	}
	for _, id := range []string{"-10012345", "@existing_channel"} {
		c := Credentials{ChatID: id}
		c.Normalize(Telegram)
		if c.Mode != Group || c.ChatID != id {
			t.Fatal("legacy group rerouted")
		}
	}
}
