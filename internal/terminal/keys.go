package terminal

import (
	tea "github.com/charmbracelet/bubbletea"
	uv "github.com/charmbracelet/ultraviolet"
)

// specialKeys maps Bubble Tea key types to the key codes the emulator understands.
var specialKeys = map[tea.KeyType]rune{
	tea.KeyEnter:     uv.KeyEnter,
	tea.KeyTab:       uv.KeyTab,
	tea.KeyBackspace: uv.KeyBackspace,
	tea.KeyEsc:       uv.KeyEscape,
	tea.KeyUp:        uv.KeyUp,
	tea.KeyDown:      uv.KeyDown,
	tea.KeyLeft:      uv.KeyLeft,
	tea.KeyRight:     uv.KeyRight,
	tea.KeyHome:      uv.KeyHome,
	tea.KeyEnd:       uv.KeyEnd,
	tea.KeyPgUp:      uv.KeyPgUp,
	tea.KeyPgDown:    uv.KeyPgDown,
	tea.KeyDelete:    uv.KeyDelete,
	tea.KeyInsert:    uv.KeyInsert,
	tea.KeyF1:        uv.KeyF1,
	tea.KeyF2:        uv.KeyF2,
	tea.KeyF3:        uv.KeyF3,
	tea.KeyF4:        uv.KeyF4,
	tea.KeyF5:        uv.KeyF5,
	tea.KeyF6:        uv.KeyF6,
	tea.KeyF7:        uv.KeyF7,
	tea.KeyF8:        uv.KeyF8,
	tea.KeyF9:        uv.KeyF9,
	tea.KeyF10:       uv.KeyF10,
	tea.KeyF11:       uv.KeyF11,
	tea.KeyF12:       uv.KeyF12,
}

// SendKey forwards a Bubble Tea key to the program. The emulator encodes it for the PTY.
func (t *Terminal) SendKey(msg tea.KeyMsg) {
	emu := t.emu
	var mod uv.KeyMod
	if msg.Alt {
		mod |= uv.ModAlt
	}
	switch {
	case msg.Type == tea.KeyRunes:
		text := string(msg.Runes)
		if msg.Alt {
			text = "\x1b" + text
		}
		// Pasted text arrives as runes too, so it goes in as is.
		emu.SendText(text)
	case msg.Type == tea.KeySpace:
		emu.SendKey(uv.KeyPressEvent{Code: uv.KeySpace, Text: " ", Mod: mod})
	case msg.Type == tea.KeyShiftTab:
		emu.SendKey(uv.KeyPressEvent{Code: uv.KeyTab, Mod: mod | uv.ModShift})
	case specialKeys[msg.Type] != 0:
		emu.SendKey(uv.KeyPressEvent{Code: specialKeys[msg.Type], Mod: mod})
	case msg.Type >= tea.KeyCtrlA && msg.Type <= tea.KeyCtrlZ:
		emu.SendKey(uv.KeyPressEvent{Code: 'a' + rune(msg.Type-tea.KeyCtrlA), Mod: mod | uv.ModCtrl})
	}
}
