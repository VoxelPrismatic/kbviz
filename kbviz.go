package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"golang.org/x/term"

	"github.com/holoplot/go-evdev"
	"github.com/mappu/miqt/qt6"
	"github.com/mappu/miqt/qt6/mainthread"
)

func escalate() {
	var cmd *exec.Cmd
	if term.IsTerminal(0) {
		args := []string{"-E"}
		args = append(args, os.Args...)
		cmd = exec.Command("sudo", args...)
	} else {

		file, err := filepath.Abs(os.Args[0])
		if err != nil {
			panic(err)
		}
		os.Args[0] = file

		args := []string{"env"}
		for _, key := range os.Environ() {
			if !strings.Contains(key, " ") {
				args = append(args, key)
			}
		}

		args = append(args, os.Args...)
		cmd = exec.Command("pkexec", args...)
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
	history         []Key
	historyMu       sync.Mutex
	label           *qt6.QLabel
	labelMu         sync.Mutex
	font            *qt6.QFont
	win             *qt6.QWidget
	app             *qt6.QApplication
	keyTime         time.Time
	_flagFontFamily *string
)

var (
	sakuraIris = "#696ac2"
	sakuraTree = "#33b473"
	sakuraRose = "#d875a7"
	sakuraGold = "#b4b433"
	sakuraLove = "#d87576"
)

var ignoreEvt = map[evdev.EvType]map[evdev.EvCode]bool{
	evdev.EV_KEY: {
		evdev.BTN_TOOL_FINGER:    true,
		evdev.BTN_TOUCH:          true,
		evdev.BTN_TOOL_DOUBLETAP: true,
		evdev.BTN_TOOL_TRIPLETAP: true,
	},
}

var classes = map[evdev.EvType]bool{
	evdev.EV_KEY: true,
}

var evStrMap = map[evdev.EvType]map[string]evdev.EvCode{
	evdev.EV_SYN: evdev.SYNFromString,
	evdev.EV_KEY: evdev.KEYFromString,
	evdev.EV_REL: evdev.RELFromString,
	evdev.EV_ABS: evdev.ABSFromString,
	evdev.EV_MSC: evdev.MSCFromString,
	evdev.EV_SW:  evdev.SWFromString,
	evdev.EV_LED: evdev.LEDFromString,
	evdev.EV_SND: evdev.SNDFromString,
	evdev.EV_REP: evdev.REPFromString,
	evdev.EV_FF:  evdev.FFFromString,
}
var evCodeMap = map[evdev.EvType]map[evdev.EvCode]string{
	evdev.EV_SYN: evdev.SYNToString,
	evdev.EV_KEY: evdev.KEYToString,
	evdev.EV_REL: evdev.RELToString,
	evdev.EV_ABS: evdev.ABSToString,
	evdev.EV_MSC: evdev.MSCToString,
	evdev.EV_SW:  evdev.SWToString,
	evdev.EV_LED: evdev.LEDToString,
	evdev.EV_SND: evdev.SNDToString,
	evdev.EV_REP: evdev.REPToString,
	evdev.EV_FF:  evdev.FFToString,
}

func evcode(val string) (evdev.EvType, evdev.EvCode, error) {
	val = strings.ToUpper(val)
	val = strings.ReplaceAll(val, "-", "_")
	val = strings.ReplaceAll(val, " ", "_")

	try, err := strconv.Atoi(val)
	if err == nil {
		ret := evdev.EvCode(try)
		for t, codeMap := range evCodeMap {
			if _, ok := codeMap[ret]; ok {
				return t, ret, nil
			}
		}
		return 0, 0, fmt.Errorf("event `%d' does not exist", try)
	}

	for t, codeMap := range evStrMap {
		code, ok := codeMap[val]
		if ok {
			return t, code, nil
		}
	}

	return 0, 0, fmt.Errorf("event `%s' doesn't exist", val)
}

func applyColor(ptr *string) func(val string) error {
	color := regexp.MustCompile("^#[a-fA-F0-9]{6}$")
	return func(val string) error {
		if !color.MatchString(val) {
			return fmt.Errorf("invalid color code")
		}
		*ptr = val
		return nil
	}
}

func applyEvent(set bool) func(val string) error {
	return func(val string) error {
		t, code, err := evcode(val)
		if ignoreEvt[t] == nil {
			ignoreEvt[t] = map[evdev.EvCode]bool{}
		}

		if err == nil {
			ignoreEvt[t][code] = set
		}
		return err
	}
}
func applyClass(set bool) func(val string) error {
	return func(val string) error {
		val = strings.ToUpper(val)
		try, err := strconv.Atoi(val)
		if err == nil {
			code := evdev.EvType(try)
			_, ok := evdev.EVToString[code]
			if !ok {
				return fmt.Errorf("class `%d' doesn't exist", code)
			}
			classes[code] = set
			return nil
		}

		if !strings.HasPrefix(val, "EV_") {
			val = "EV_" + val
		}

		t, ok := evdev.EVFromString[val]
		if !ok {
			return fmt.Errorf("class `%s' doesn't exist", val)
		}

		classes[t] = set
		return nil
	}
}

func main() {
	if syscall.Geteuid() != 0 {
		escalate()
		return
	}

	devs := grabKeyboards()

	_flagGui := flag.Bool("gui", false, "Enable GUI")
	_flagFontFamily = flag.String("font", "", "Set the font family")
	flag.Func("iris", "Set the color 'iris'", applyColor(&sakuraIris))
	flag.Func("tree", "Set the color 'tree'", applyColor(&sakuraTree))
	flag.Func("rose", "Set the color 'rose'", applyColor(&sakuraRose))
	flag.Func("gold", "Set the color 'gold'", applyColor(&sakuraGold))
	flag.Func("love", "Set the color 'rose'", applyColor(&sakuraLove))
	flag.Func("evt-", "Ignore this event", applyEvent(true))
	flag.Func("evt+", "Listen to this event", applyEvent(false))
	flag.Func("S", "Set a symbol in the format of <key>=<char> eg KEY_NUM_8=8", func(val string) error {
		parts := strings.SplitN(val, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("not in proper format (eg KEY_NUM_8=8)")
		}

		t, code, err := evcode(parts[0])
		if err == nil {
			if tokens[t] == nil {
				tokens[t] = map[evdev.EvCode]string{}
			}
			tokens[t][code] = parts[1]
		}
		return err
	})
	flag.Func("cls-", "Ignore an event class (eg EV_KEY)", applyClass(false))
	flag.Func("cls+", "Listen to an event class (eg EV_KEY)", applyClass(true))
	_flagTimeout := flag.Uint("timeout", 5, "Time before clearing the output")

	flag.Parse()

	doGUI := !term.IsTerminal(0)
	if _flagGui != nil {
		doGUI = *_flagGui
	}

	done := make(chan bool)
	for _, dev := range devs {
		go listen(done, dev)
	}

	keyTime = time.Now()
	timeout := time.Duration(1000 * 1000 * 1000 * (*_flagTimeout))
	go func() {
		for true {
			if time.Now().Sub(keyTime) >= timeout {
				historyMu.Lock()
				history = []Key{}
				historyMu.Unlock()
				PrintHistory()
			}
			time.Sleep(time.Duration(1000 * 1000 * 500))
		}
	}()

	if doGUI {
		makeGUI()
	} else {
		PrintHistory()
	}

	<-done
}

func scaleLabel(sz *qt6.QSize) {
	labelMu.Lock()
	font.SetPixelSize(int(float64(sz.Height()) * 0.8))
	label.SetFont(font)
	label.SetFixedHeight(sz.Height())
	label.SetFixedWidth(sz.Width() - 16)
	label.SetAlignment(qt6.AlignRight)
	labelMu.Unlock()
	PrintHistory()
}

type Sizes struct {
	Max *uint
	Min *uint
	Fix *uint
}

func makeGUI() {
	fmt.Println("gui")
	app = qt6.NewQApplication(os.Args)
	defer qt6.QApplication_Exec()
	fmt.Println(app.StyleSheet())

	win = qt6.NewQWidget(nil)
	win.SetWindowTitle("KbViz")
	ico := qt6.QIcon_FromTheme("ktouch")
	win.SetWindowIcon(ico)
	win.SetFixedSize2(640, 40)

	if _flagFontFamily == nil || *_flagFontFamily == "" {
		font = qt6.QFontDatabase_SystemFont(qt6.QFontDatabase__FixedFont)
	} else {
		font = qt6.NewQFont2(*_flagFontFamily)
	}

	label = qt6.NewQLabel(nil)
	label.SetMinimumSize2(1, 1)
	label.SetAlignment(qt6.AlignRight)
	label.SetFont(font)

	layout := qt6.NewQHBoxLayout(nil)
	layout.SetContentsMargins(0, 0, 0, 0)
	layout.SetDirection(qt6.QBoxLayout__RightToLeft)
	layout.AddWidget3(label.QWidget, 0, qt6.AlignRight)

	win.SetLayoutDirection(qt6.RightToLeft)
	win.SetLayout(layout.QLayout)
	win.SetContentsMargins(8, 0, 8, 0)

	win.OnResizeEvent(func(_ func(_ *qt6.QResizeEvent), evt *qt6.QResizeEvent) {
		scaleLabel(evt.Size())
	})

	scaleLabel(win.Size())

	win.OnCloseEvent(func(_ func(_ *qt6.QCloseEvent), evt *qt6.QCloseEvent) {
		os.Exit(0)
	})

	win.OnShowEvent(func(_ func(event *qt6.QShowEvent), evt *qt6.QShowEvent) {
		win.SetMaximumSize2(65535, 8192)
		win.SetMinimumSize2(16, 16)

		scaleLabel(win.Size())
	})

	win.OnKeyPressEvent(func(_ func(_ *qt6.QKeyEvent), evt *qt6.QKeyEvent) {
		geo := win.Geometry()
		step := 8
		mods := evt.Modifiers()
		if mods&qt6.ShiftModifier > 0 {
			step = 32
		} else if mods&qt6.ControlModifier > 0 {
			step = 1
		}

		switch qt6.Key(evt.Key()) {
		case qt6.Key_H, qt6.Key_Left:
			win.SetGeometry(geo.X(), geo.Y(), max(16, geo.Width()-step), geo.Height())
		case qt6.Key_J, qt6.Key_Down:
			win.SetGeometry(geo.X(), geo.Y(), geo.Width(), min(8192, geo.Height()+step))
		case qt6.Key_K, qt6.Key_Up:
			win.SetGeometry(geo.X(), geo.Y(), geo.Width(), max(16, geo.Height()-step))
		case qt6.Key_L, qt6.Key_Right:
			win.SetGeometry(geo.X(), geo.Y(), min(8192, geo.Width()+step), geo.Height())
		}
	})

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
			if classes[t] {
				good = true
				break
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
	skip := map[evdev.EvType]*ModSet[bool]{}
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

		if skip[evt.Type] == nil {
			skip[evt.Type] = &ModSet[bool]{}
		}

		go goHandle(dev, evt, skip[evt.Type])
	}
}

func goHandle(dev *evdev.InputDevice, evt *evdev.InputEvent, skip *ModSet[bool]) {
	ignoreMap, ok := ignoreEvt[evt.Type]
	if !classes[evt.Type] || (ok && ignoreMap[evt.Code]) {
		return
	}

	key := makeKey(skip, dev, evt)
	fmt.Println(key)
	if key == nil {
		return
	}

	historyMu.Lock()
	var last *Key
	if len(history) > 0 {
		last = &history[len(history)-1]
		for i := len(history) - 1; (i >= 0) && (last.Type != key.Type); i-- {
			fmt.Printf("-> %d\n", i)
			last = &history[i]
		}
	} else {
		last = &Key{}
	}
	if last.Equals(*key) {
		last.Count = last.Count + 1
		dup := *last
		slices.Reverse(history)
		i := slices.Index(history, dup)
		if i >= 0 {
			history = slices.Concat(history[:i], history[i+1:])
		}
		slices.Reverse(history)
		history = append(history, dup)
	} else {
		history = append(history, *key)
	}
	historyMu.Unlock()

	keyTime = time.Now()
	PrintHistory()
}

type Key struct {
	Type  evdev.EvType
	Char  string
	Code  evdev.EvCode
	Name  string
	Found bool
	Held  ModSet[bool]
	Count int
}

func (this Key) Equals(other Key) bool {
	return this.Name == other.Name &&
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
		sub = fmt.Sprintf("\x1b[1m%s\x1b[0m", sub)
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

func (key *Key) Qt() string {
	sub := key.Char
	r, sz := utf8.DecodeRuneInString(sub + ".")
	if !key.Found {
		sub = fmt.Sprintf("<font color='%s'>%d: <b>%s</b></font>", sakuraTree, key.Code, key.Name)
	} else if utf8.RuneCountInString(key.Char) > 1 && r < 255 {
		sub = fmt.Sprintf("<b>%s</b>", sub)
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
	if key.Count > 1 {
		sub = fmt.Sprintf("%s<i><font color='%s'>×%d</font></i>", sub, sakuraRose, key.Count)
	}

	return sub
}

func makeKey(skip *ModSet[bool], dev *evdev.InputDevice, evt *evdev.InputEvent) *Key {
	key := Key{
		Type:  evt.Type,
		Code:  evt.Code,
		Name:  evt.CodeName(),
		Held:  modState(dev),
		Count: 1,
	}

	if charMap, ok := tokens[evt.Type]; ok {
		key.Char, key.Found = charMap[evt.Code]
	} else {
		key.Char, key.Found = "", false
	}

	jump := map[string]*bool{
		tokens[evdev.EV_KEY][evdev.KEY_LEFTSHIFT]:  &skip.Shift,
		tokens[evdev.EV_KEY][evdev.KEY_RIGHTSHIFT]: &skip.Shift,
		tokens[evdev.EV_KEY][evdev.KEY_LEFTCTRL]:   &skip.Ctrl,
		tokens[evdev.EV_KEY][evdev.KEY_RIGHTCTRL]:  &skip.Ctrl,
		tokens[evdev.EV_KEY][evdev.KEY_LEFTALT]:    &skip.Alt,
		tokens[evdev.EV_KEY][evdev.KEY_RIGHTALT]:   &skip.Alt,
		tokens[evdev.EV_KEY][evdev.KEY_LEFTMETA]:   &skip.Meta,
		tokens[evdev.EV_KEY][evdev.KEY_RIGHTMETA]:  &skip.Meta,
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

func PrintQtHistory() {
	metric := qt6.NewQFontMetrics(font)
	w := label.Size().Width()

	var i int
	st := ""
	ansi_st := ""
	for i = len(history) - 1; i >= 0; i-- {
		key := history[i]
		if key.Char == "\x00" {
			continue
		}

		ansi_st = ansi.ReplaceAllString(key.String(true), "") + " " + ansi_st
		l := metric.BoundingRectWithText(ansi_st).Width()
		if l >= w {
			break
		}
		st = key.Qt() + " " + st
	}

	if i >= 24 {
		history = history[i:]
	}

	labelMu.Lock()
	label.SetText(strings.TrimSpace(st))
	labelMu.Unlock()
}

func PrintHistory() {
	if font != nil {
		mainthread.Start(PrintQtHistory)
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
		if key.Char == "\x00" {
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
	"0": ")", "-": "_", "=": "+", "[": "{", "]": "}", ";": ":",
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

var tokens = map[evdev.EvType]map[evdev.EvCode]string{
	evdev.EV_KEY: {
		evdev.KEY_RESERVED:   "\x00",
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
		evdev.KEY_KP0:        "#0",
		evdev.KEY_KP1:        "#1",
		evdev.KEY_KP2:        "#2",
		evdev.KEY_KP3:        "#3",
		evdev.KEY_KP4:        "#4",
		evdev.KEY_KP5:        "#5",
		evdev.KEY_KP6:        "#6",
		evdev.KEY_KP7:        "#7",
		evdev.KEY_KP8:        "#8",
		evdev.KEY_KP9:        "#9",
		evdev.KEY_LEFTMETA:   leftChar + modChar.Meta,
		evdev.KEY_RIGHTMETA:  modChar.Meta + rightChar,
		evdev.KEY_LEFTCTRL:   leftChar + modChar.Ctrl,
		evdev.KEY_RIGHTCTRL:  modChar.Ctrl + rightChar,
		evdev.KEY_LEFTSHIFT:  leftChar + modChar.Shift,
		evdev.KEY_RIGHTSHIFT: modChar.Shift + rightChar,
		evdev.KEY_LEFTALT:    leftChar + modChar.Alt,
		evdev.KEY_RIGHTALT:   modChar.Alt + rightChar,
	},
}
