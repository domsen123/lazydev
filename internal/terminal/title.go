package terminal

import "bytes"

// titleFilter removes window title sequences (OSC 0, 1 and 2) from PTY output and reports the
// title instead. x/vt treats the byte 0x9C as String Terminator even inside UTF-8 characters, so a
// title like "✳ Claude Code" (✳ is E2 9C B3) ends early and the rest shows up as text on screen.
// Sequences can be split across reads, so the filter keeps an unfinished one until the next call.
type titleFilter struct {
	pending []byte
	onTitle func(string)
}

// maxOSC bounds a pending sequence. Longer ones are passed through unchanged.
const maxOSC = 4096

// filter returns data without title sequences.
func (f *titleFilter) filter(data []byte) []byte {
	data = append(f.pending, data...)
	f.pending = nil
	out := make([]byte, 0, len(data))
	for len(data) > 0 {
		i := bytes.Index(data, []byte("\x1b]"))
		if i < 0 {
			// A trailing ESC might start a sequence in the next read.
			if data[len(data)-1] == 0x1b {
				out = append(out, data[:len(data)-1]...)
				f.pending = []byte{0x1b}
				return out
			}
			return append(out, data...)
		}
		out = append(out, data[:i]...)
		data = data[i:]
		end, termLen := oscEnd(data)
		if end < 0 {
			if len(data) > maxOSC {
				return append(out, data...)
			}
			f.pending = append([]byte(nil), data...)
			return out
		}
		seq := data[:end+termLen]
		payload := data[2:end]
		if title, ok := titlePayload(payload); ok {
			if f.onTitle != nil {
				f.onTitle(title)
			}
		} else {
			out = append(out, seq...)
		}
		data = data[end+termLen:]
	}
	return out
}

// oscEnd finds the terminator of the OSC sequence at the start of data: BEL or ESC \. It returns
// -1 if the sequence is not complete yet.
func oscEnd(data []byte) (end, termLen int) {
	for i := 2; i < len(data); i++ {
		switch {
		case data[i] == 0x07:
			return i, 1
		case data[i] == 0x1b && i+1 < len(data) && data[i+1] == '\\':
			return i, 2
		}
	}
	return -1, 0
}

// titlePayload reports whether payload sets the window or icon title, and the title.
func titlePayload(payload []byte) (string, bool) {
	for _, prefix := range []string{"0;", "1;", "2;"} {
		if title, ok := bytes.CutPrefix(payload, []byte(prefix)); ok {
			return string(title), true
		}
	}
	return "", false
}
