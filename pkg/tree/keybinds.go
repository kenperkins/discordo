package tree

import (
	"charm.land/bubbles/v2/key"
	myKey "github.com/ayn2op/discordo/pkg/key"
)

type Keybinds struct {
	Up       key.Binding
	Down     key.Binding
	Top      key.Binding
	Bottom   key.Binding
	PageUp   key.Binding
	PageDown key.Binding

	Select key.Binding
}

func DefaultKeybinds() Keybinds {
	return Keybinds{
		Up:       myKey.NewBinding("up", "up"),
		Down:     myKey.NewBinding("down", "down"),
		Top:      myKey.NewBinding("home", "top"),
		Bottom:   myKey.NewBinding("end", "bottom"),
		PageUp:   myKey.NewBinding("pgup", "page up"),
		PageDown: myKey.NewBinding("pgdn", "page down"),
		Select:   myKey.NewBinding("enter", "select"),
	}
}
