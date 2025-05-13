package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"unicode/utf8"

	"golang.org/x/term"

	"github.com/holoplot/go-evdev"
	"github.com/mappu/miqt/qt6"
)

func escalate() {
	var cmd *exec.Cmd
	if term.IsTerminal(0) {
		cmd = exec.Command("sudo", os.Args...)
	} else {
		cmd = exec.Command("pkexec", os.Args...)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		panic(err)
	}
	os.Exit(0)
}

var (
	history   []Key
	historyMu sync.Mutex
)

var (
	sakuraIris = "#696ac2"
	sakuraTree = "#33b473"
	sakuraRose = "#d875a7"
	sakuraGold = "#b4b433"
	sakuraLove = "#d87576"
)

func main() {
	if syscall.Geteuid() != 0 {
		escalate()
		return
	}

	devs := grabKeyboards()

	doGUI := !term.IsTerminal(0)
	color := regexp.MustCompile("^--(\\w+)=\"(#[a-fA-F0-9]{6})\"$")
	for _, arg := range os.Args {
		if arg == "--gui" {
			doGUI = true
			continue
		} else if arg == "--no-gui" {
			doGUI = false
			continue
		}

		for _, group := range color.FindAllStringSubmatch(arg, -1) {
			if len(group) != 2 {
				panic("Invalid color arg: " + arg)
			}

			name, hex := group[0], group[1]

			switch name {
			case "iris", "blue":
				sakuraIris = hex
			case "tree", "green":
				sakuraTree = hex
			case "rose", "pink":
				sakuraRose = hex
			case "gold", "yellow":
				sakuraGold = hex
			case "love", "red":
				sakuraRose = hex
			}
		}
	}

	done := make(chan bool)
	for _, dev := range devs {
		go listen(done, dev)
	}

	if doGUI {
		makeGUI()
	}

	<-done
}

func makeGUI() {
	fmt.Println("gui")
	qt6.NewQApplication(os.Args)
	defer qt6.QApplication_Exec()

	win := qt6.NewQMainWindow(nil)
	win.SetWindowTitle("KbViz")
	win.SetMinimumSize2(400, 40)
	win.Show()
}

func grabKeyboards() []*evdev.InputDevice {
	ret := []*evdev.InputDevice{}

	basePath := "/dev/input"

	files, err := os.ReadDir(basePath)
	if err != nil {
		panic(err)
	}

	for _, fileName := range files {
		if fileName.IsDir() {
			continue
		}

		full := fmt.Sprintf("%s/%s", basePath, fileName.Name())
		dev, err := evdev.OpenWithFlags(full, os.O_RDONLY)
		if err != nil {
			continue
		}

		good := false
		for _, t := range dev.CapableTypes() {
			switch t {
			case evdev.EV_KEY:
				good = true
			}
		}
		if good {
			ret = append(ret, dev)
		} else {
			dev.Close()
		}
	}

	return ret
}

func listen(done chan bool, dev *evdev.InputDevice) {
	defer dev.Close()

	path := dev.Path()
	skip := ModSet[bool]{}
	name, err := dev.Name()
	if err != nil {
		panic(err)
	}

	for {
		evt, err := dev.ReadOne()
		if err != nil {
			fmt.Fprintf(os.Stderr, "read: \x1b[91;1m%s\x1b[0m [%s]: %s", name, path, err.Error())
			done <- true
			return
		}

		go goHandle(dev, evt, &skip)
	}
}

func goHandle(dev *evdev.InputDevice, evt *evdev.InputEvent, skip *ModSet[bool]) {
	switch evt.Type {
	case evdev.EV_KEY:
		key := makeKey(skip, dev, evt)
		if key == nil {
			return
		}
		historyMu.Lock()
		var last *Key
		if len(history) > 0 {
			last = &history[len(history)-1]
		} else {
			last = &Key{}
		}
		if last.Equals(*key) {
			last.Count = last.Count + 1
			key = last
		} else {
			history = append(history, *key)
		}
		historyMu.Unlock()
		PrintHistory()
	}
}

type Key struct {
	Char  string
	Code  evdev.EvCode
	Name  string
	Found bool
	Held  ModSet[bool]
	Count int
}

func (this Key) Equals(other Key) bool {
	return this.Code == other.Code &&
		this.Held.Shift == other.Held.Shift &&
		this.Held.Ctrl == other.Held.Ctrl &&
		this.Held.Alt == other.Held.Alt &&
		this.Held.Meta == other.Held.Meta
}

func (key Key) String(withCount bool) string {
	sub := key.Char
	r, sz := utf8.DecodeRuneInString(sub + ".")
	if !key.Found {
		sub = fmt.Sprintf("\x1b[92;1m<%d: %s>\x1b[0m", key.Code, key.Name)
	} else if utf8.RuneCountInString(key.Char) > 1 && r < 255 {
		sub = fmt.Sprintf("\x1b[94;1m<%s>\x1b[0m", sub)
	} else if r == leftCharRune {
		sub = fmt.Sprintf("\x1b[93;1m%s\x1b[94;1m%s\x1b[0m", leftChar, sub[sz:])
	} else if r > 255 {
		r, sz = utf8.DecodeLastRuneInString(sub)
		if r == rightCharRune {
			sub = fmt.Sprintf("\x1b[94;1m%s\x1b[93;1m%s\x1b[0m", sub[:len(sub)-sz], rightChar)
		} else {
			sub = fmt.Sprintf("\x1b[94;1m%s\x1b[0m", sub)
		}
	} else {
		sub = strings.ToLower(sub)
	}

	if key.Held.Shift {
		shift, exist := shifts[sub]
		if exist {
			sub = shift
		} else {
			sub = modLove.Shift + sub
		}
	}
	if key.Held.Alt {
		sub = modLove.Alt + sub
	}
	if key.Held.Ctrl {
		sub = modLove.Ctrl + sub
	}
	if key.Held.Meta {
		sub = modLove.Meta + sub
	}
	if withCount && key.Count > 1 {
		sub = fmt.Sprintf("%s\x1b[95;3m×%d\x1b[0m", sub, key.Count)
	}

	return sub
}

func (key Key) HTMLString() string {
	sub := key.Char
	r, sz := utf8.DecodeRuneInString(sub + ".")
	if !key.Found {
		sub = fmt.Sprintf("<font color='%s'>&lt;%d: <b>%s</b>&gt;</font>", sakuraTree, key.Code, key.Name)
	} else if utf8.RuneCountInString(key.Char) > 1 && r < 255 {
		sub = fmt.Sprintf("<font color='%s'>&lt;<b>%s</b>&gt</font>", sakuraIris, sub)
	} else if r == leftCharRune {
		sub = fmt.Sprintf("<font color='%s'>%s</font>", sakuraGold, leftChar) +
			fmt.Sprintf("<font color='%s'><b>%s</b></font>", sakuraIris, sub[sz:])
	} else if r > 255 {
		r, sz = utf8.DecodeLastRuneInString(sub)
		if r == rightCharRune {
			sub = fmt.Sprintf("<font color='%s'><b>%s</b></font>", sakuraIris, sub[:len(sub)-sz]) +
				fmt.Sprintf("<font color='%s'>%s</font>", sakuraGold, rightChar)
		} else {
			sub = fmt.Sprintf("<font color='%s'><b>%s</b></font>", sakuraIris, sub)
		}
	} else {
		sub = strings.ToLower(sub)
	}

	modHtml := ModSet[string]{
		Shift: fmt.Sprintf("<font color='%s'><b>%s</b></font>", sakuraLove, modChar.Shift),
		Ctrl:  fmt.Sprintf("<font color='%s'><b>%s</b></font>", sakuraLove, modChar.Ctrl),
		Alt:   fmt.Sprintf("<font color='%s'><b>%s</b></font>", sakuraLove, modChar.Alt),
		Meta:  fmt.Sprintf("<font color='%s'><b>%s</b></font>", sakuraLove, modChar.Meta),
	}
	if key.Held.Shift {
		shift, exist := shifts[sub]
		if exist {
			sub = shift
		} else {
			sub = modHtml.Shift + sub
		}
	}
	if key.Held.Alt {
		sub = modHtml.Alt + sub
	}
	if key.Held.Ctrl {
		sub = modHtml.Ctrl + sub
	}
	if key.Held.Meta {
		sub = modHtml.Meta + sub
	}
	sub = fmt.Sprintf("%s<i><font color='%s'>×%d</font></i>", sub, sakuraRose, key.Count)

	return sub
}

func makeKey(skip *ModSet[bool], dev *evdev.InputDevice, evt *evdev.InputEvent) *Key {
	key := Key{
		Code:  evt.Code,
		Name:  evt.CodeName(),
		Held:  modState(dev),
		Count: 1,
	}

	key.Char, key.Found = chars[evt.Code]

	jump := map[string]*bool{
		chars[evdev.KEY_LEFTSHIFT]:  &skip.Shift,
		chars[evdev.KEY_RIGHTSHIFT]: &skip.Shift,
		chars[evdev.KEY_LEFTCTRL]:   &skip.Ctrl,
		chars[evdev.KEY_RIGHTCTRL]:  &skip.Ctrl,
		chars[evdev.KEY_LEFTALT]:    &skip.Alt,
		chars[evdev.KEY_RIGHTALT]:   &skip.Alt,
		chars[evdev.KEY_LEFTMETA]:   &skip.Meta,
		chars[evdev.KEY_RIGHTMETA]:  &skip.Meta,
	}

	ptr, isMod := jump[key.Char]
	if isMod {
		if evt.Value != 0 {
			return nil
		} else if *ptr {
			*ptr = false
			return nil
		}
	} else if evt.Value == 0 {
		return nil
	}

	skip.Shift = skip.Shift || key.Held.Shift
	skip.Alt = skip.Alt || key.Held.Alt
	skip.Ctrl = skip.Ctrl || key.Held.Ctrl
	skip.Meta = skip.Meta || key.Held.Meta

	return &key
}

func modState(dev *evdev.InputDevice) ModSet[bool] {
	state, err := dev.State(evdev.EV_KEY)
	if err != nil {
		panic(err)
	}

	return ModSet[bool]{
		Shift: state[evdev.KEY_LEFTSHIFT] || state[evdev.KEY_RIGHTSHIFT],
		Alt:   state[evdev.KEY_LEFTALT] || state[evdev.KEY_RIGHTALT],
		Ctrl:  state[evdev.KEY_LEFTCTRL] || state[evdev.KEY_RIGHTCTRL],
		Meta:  state[evdev.KEY_LEFTMETA] || state[evdev.KEY_RIGHTMETA],
	}
}

func PrintHistory() {
	if !term.IsTerminal(0) {
		// not a tty
		return
	}

	w, _, err := term.GetSize(0)
	if err != nil {
		return
	}

	var i int
	st := ""
	l := 0
	for i = len(history) - 1; i >= 0; i-- {
		key := history[i]
		if key.Char == "" {
			continue
		}
		new_st := key.String(true) + " " + st
		new_l := utf8.RuneCountInString(ansi.ReplaceAllString(new_st, ""))
		if new_l >= w {
			break
		}
		st = new_st
		l = new_l
	}

	if len(history) > 10*w {
		history = history[10*w:]
	}

	st = strings.Repeat(" ", max(0, w-l)) + st

	fmt.Printf("\x1b[H\x1b[2J%s\r", st)
}

var ansi = regexp.MustCompile("\x1b\\[\\d+(;\\d+)?m")

var shifts = map[string]string{
	"a": "A", "b": "B", "c": "C", "d": "D", "e": "E", "f": "F",
	"g": "G", "h": "H", "i": "I", "j": "J", "k": "K", "l": "L",
	"m": "M", "n": "N", "o": "O", "p": "P", "q": "Q", "r": "R",
	"s": "S", "t": "T", "u": "U", "v": "V", "w": "W", "x": "X",
	"y": "Y", "z": "Z", "`": "~", "1": "!", "2": "@", "3": "#",
	"4": "$", "5": "%", "6": "^", "7": "&", "8": "*", "9": "(",
	"0": ")", "-": "+", "=": "+", "[": "{", "]": "}", ";": ":",
	"'": "\"", ",": "<", ".": ">", "/": "?", "\\": "|",
}

type ModSet[T any] struct {
	Shift T
	Ctrl  T
	Alt   T
	Meta  T
}

var modChar = ModSet[string]{
	Shift: "⮭",
	Ctrl:  "▲",
	Alt:   "",
	Meta:  "",
}

var modLove = ModSet[string]{
	Shift: "\x1b[91;1m" + modChar.Shift + "\x1b[0m",
	Ctrl:  "\x1b[91;1m" + modChar.Ctrl + "\x1b[0m",
	Alt:   "\x1b[91;1m" + modChar.Alt + "\x1b[0m",
	Meta:  "\x1b[91;1m" + modChar.Meta + "\x1b[0m",
}

var (
	rightChar        = ""
	leftChar         = ""
	leftCharRune, _  = utf8.DecodeRuneInString(leftChar)
	rightCharRune, _ = utf8.DecodeRuneInString(rightChar)
)

var chars = map[evdev.EvCode]string{
	evdev.KEY_RESERVED:   "",
	evdev.BTN_RIGHT:      "",
	evdev.BTN_LEFT:       "",
	evdev.BTN_MIDDLE:     "",
	evdev.BTN_EXTRA:      "¹",
	evdev.BTN_SIDE:       "²",
	evdev.KEY_ESC:        "󱥨",
	evdev.KEY_1:          "1",
	evdev.KEY_2:          "2",
	evdev.KEY_3:          "3",
	evdev.KEY_4:          "4",
	evdev.KEY_5:          "5",
	evdev.KEY_6:          "6",
	evdev.KEY_7:          "7",
	evdev.KEY_8:          "8",
	evdev.KEY_9:          "9",
	evdev.KEY_0:          "0",
	evdev.KEY_MINUS:      "-",
	evdev.KEY_EQUAL:      "=",
	evdev.KEY_BACKSPACE:  "󰁮",
	evdev.KEY_DELETE:     "󰹾",
	evdev.KEY_TAB:        "↹",
	evdev.KEY_Q:          "Q",
	evdev.KEY_W:          "W",
	evdev.KEY_E:          "E",
	evdev.KEY_R:          "R",
	evdev.KEY_T:          "T",
	evdev.KEY_Y:          "Y",
	evdev.KEY_U:          "U",
	evdev.KEY_I:          "I",
	evdev.KEY_O:          "O",
	evdev.KEY_P:          "P",
	evdev.KEY_LEFTBRACE:  "[",
	evdev.KEY_RIGHTBRACE: "]",
	evdev.KEY_ENTER:      "↲",
	evdev.KEY_A:          "A",
	evdev.KEY_S:          "S",
	evdev.KEY_D:          "D",
	evdev.KEY_F:          "F",
	evdev.KEY_G:          "G",
	evdev.KEY_H:          "H",
	evdev.KEY_J:          "J",
	evdev.KEY_K:          "K",
	evdev.KEY_L:          "L",
	evdev.KEY_SEMICOLON:  ";",
	evdev.KEY_APOSTROPHE: "'",
	evdev.KEY_GRAVE:      "`",
	evdev.KEY_BACKSLASH:  "\\",
	evdev.KEY_Z:          "Z",
	evdev.KEY_X:          "X",
	evdev.KEY_C:          "C",
	evdev.KEY_V:          "V",
	evdev.KEY_B:          "B",
	evdev.KEY_N:          "N",
	evdev.KEY_M:          "M",
	evdev.KEY_COMMA:      ",",
	evdev.KEY_DOT:        ".",
	evdev.KEY_SLASH:      "/",
	evdev.KEY_LEFT:       "←",
	evdev.KEY_RIGHT:      "→",
	evdev.KEY_UP:         "↑",
	evdev.KEY_DOWN:       "↓",
	evdev.KEY_SPACE:      "⋯",
	evdev.KEY_HOME:       "⇐",
	evdev.KEY_END:        "⇒",
	evdev.KEY_PAGEUP:     "↥",
	evdev.KEY_PAGEDOWN:   "↧",
	evdev.KEY_INSERT:     "INS",
	evdev.KEY_F1:         "󰊕1",
	evdev.KEY_F2:         "󰊕2",
	evdev.KEY_F3:         "󰊕3",
	evdev.KEY_F4:         "󰊕4",
	evdev.KEY_F5:         "󰊕5",
	evdev.KEY_F6:         "󰊕6",
	evdev.KEY_F7:         "󰊕7",
	evdev.KEY_F8:         "󰊕8",
	evdev.KEY_F9:         "󰊕9",
	evdev.KEY_F10:        "󰊕10",
	evdev.KEY_F11:        "󰊕11",
	evdev.KEY_F12:        "󰊕12",
	evdev.KEY_F13:        "󰊕13",
	evdev.KEY_F14:        "󰊕14",
	evdev.KEY_F15:        "󰊕15",
	evdev.KEY_F16:        "󰊕16",
	evdev.KEY_F17:        "󰊕17",
	evdev.KEY_F18:        "󰊕18",
	evdev.KEY_F19:        "󰊕19",
	evdev.KEY_F20:        "󰊕20",
	evdev.KEY_F21:        "󰊕21",
	evdev.KEY_F22:        "󰊕22",
	evdev.KEY_F23:        "󰊕23",
	evdev.KEY_F24:        "󰊕24",
	evdev.KEY_LEFTMETA:   leftChar + modChar.Meta,
	evdev.KEY_RIGHTMETA:  modChar.Meta + rightChar,
	evdev.KEY_LEFTCTRL:   leftChar + modChar.Ctrl,
	evdev.KEY_RIGHTCTRL:  modChar.Ctrl + rightChar,
	evdev.KEY_LEFTSHIFT:  leftChar + modChar.Shift,
	evdev.KEY_RIGHTSHIFT: modChar.Shift + rightChar,
	evdev.KEY_LEFTALT:    leftChar + modChar.Alt,
	evdev.KEY_RIGHTALT:   modChar.Alt + rightChar,
}
