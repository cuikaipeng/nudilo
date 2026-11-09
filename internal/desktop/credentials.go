package desktop

import (
	"callbell/internal/engine"
	"callbell/internal/model"
)

type localChannels struct {
	saved   map[model.Channel]model.Credentials
	missing map[model.Channel]bool
}

// Read the local encrypted file, never the operating system credential store.
// The UI keeps this snapshot separate from unsaved form edits.
func readLocalChannels(e *engine.Engine, p model.Preferences) localChannels {
	local := localChannels{map[model.Channel]model.Credentials{}, map[model.Channel]bool{}}
	for ch, enabled := range p.Channels {
		if enabled {
			credentials, err := e.Credentials(ch)
			if err != nil {
				local.missing[ch] = true
				continue
			}
			credentials.Normalize(ch)
			local.saved[ch] = credentials
		}
	}
	return local
}

func (a *app) applyLocalChannels(local localChannels) {
	a.savedChannels, a.localMissing = local.saved, local.missing
}
