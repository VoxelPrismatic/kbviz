from typing import List
import evdev
from evdev import ecodes as C
import os
import re
import threading

history = []

scancodes = {
    # Scancode: ASCII Code
    C.KEY_RESERVED: None,
    C.KEY_ESC: "󱥨",
    C.KEY_1: "1",
    C.KEY_2: "2",
    C.KEY_3: "3",
    C.KEY_4: "4",
    C.KEY_5: "5",
    C.KEY_6: "6",
    C.KEY_7: "7",
    C.KEY_8: "8",
    C.KEY_9: "9",
    C.KEY_0: "0",
    C.KEY_MINUS: "-",
    C.KEY_EQUAL: "=",
    C.KEY_BACKSPACE: "󰁮",
    C.KEY_DELETE: "󰹾",
    C.KEY_TAB: "↹",
    C.KEY_Q: "Q",
    C.KEY_W: "W",
    C.KEY_E: "E",
    C.KEY_R: "R",
    C.KEY_T: "T",
    C.KEY_Y: "Y",
    C.KEY_U: "U",
    C.KEY_I: "I",
    C.KEY_O: "O",
    C.KEY_P: "P",
    C.KEY_LEFTBRACE: "[",
    C.KEY_RIGHTBRACE: "]",
    C.KEY_ENTER: "↲",
    C.KEY_A: "A",
    C.KEY_S: "S",
    C.KEY_D: "D",
    C.KEY_F: "F",
    C.KEY_G: "G",
    C.KEY_H: "H",
    C.KEY_J: "J",
    C.KEY_K: "K",
    C.KEY_L: "L",
    C.KEY_SEMICOLON: ";",
    C.KEY_APOSTROPHE: "'",
    C.KEY_GRAVE: "`",
    C.KEY_BACKSLASH: "\\",
    C.KEY_Z: "Z",
    C.KEY_X: "X",
    C.KEY_C: "C",
    C.KEY_V: "V",
    C.KEY_B: "B",
    C.KEY_N: "N",
    C.KEY_M: "M",
    C.KEY_COMMA: ",",
    C.KEY_DOT: ".",
    C.KEY_SLASH: "/",
    C.KEY_RIGHT: "←",
    C.KEY_LEFT: "→",
    C.KEY_UP: "↑",
    C.KEY_DOWN: "↓",
    C.KEY_SPACE: "⋯",
    C.KEY_HOME: "⇐",
    C.KEY_END: "⇒",
    C.KEY_PAGEUP: "↥",
    C.KEY_PAGEDOWN: "↧",
    C.KEY_INSERT: "INS",
    C.KEY_F1: "󰊕1",
    C.KEY_F2: "󰊕2",
    C.KEY_F3: "󰊕3",
    C.KEY_F4: "󰊕4",
    C.KEY_F5: "󰊕5",
    C.KEY_F6: "󰊕6",
    C.KEY_F7: "󰊕7",
    C.KEY_F8: "󰊕8",
    C.KEY_F9: "󰊕9",
    C.KEY_F10: "󰊕10",
    C.KEY_F11: "󰊕11",
    C.KEY_F12: "󰊕12",
    C.KEY_LEFTMETA: "",
    C.KEY_RIGHTMETA: "",
    C.KEY_LEFTCTRL: "▲",
    C.KEY_RIGHTCTRL: "▲",
    C.KEY_LEFTSHIFT: "⮭",
    C.KEY_RIGHTSHIFT: "⮭",
    C.KEY_LEFTALT: "",
    C.KEY_RIGHTALT: "",
}

shifts = {
    "a": "A", "b": "B", "c": "C", "d": "D", "e": "E", "f": "F", "g": "G", "h": "H", "i": "I", "j": "J",
    "k": "K", "l": "L", "m": "M", "n": "N", "o": "O", "p": "P", "q": "Q", "r": "R", "s": "S", "t": "T",
    "u": "U", "v": "V", "w": "W", "x": "X", "y": "Y", "z": "Z", "`": "~", "1": "!", "2": "@", "3": "#",
    "4": "$", "5": "%", "6": "^", "7": "&", "8": "*", "9": "(", "0": ")", "-": "_", "=": "+", "[": "{",
    "]": "}", "\\": "|", ";": ":", "'": '"', ",": "<", ".": ">", "/": "?"
}


mod_SHIFT = "\x1b[94;1m⮭\x1b[0m"
mod_CTRL = "\x1b[94;1m▲\x1b[0m"
mod_ALT = "\x1b[94;1m\x1b[0m"
mod_META = "\x1b[94;1m\x1b[0m"

mods = [
    mod_SHIFT,
    mod_CTRL,
    mod_ALT,
    mod_META
]


def grab_keyboards() -> List[evdev.InputDevice]:
    devices = [evdev.InputDevice(fn) for fn in evdev.list_devices()]
    keyboards: List[evdev.InputDevice] = []
    for device in devices:
        caps = device.capabilities(verbose=True)
        if ("EV_KEY", 1) not in caps:
            continue

        for key in caps[("EV_KEY", 1)]:
            if key[0] == "KEY_ESC":
                break
        else:
            continue

        keyboards.append(device)
        print("\n\n")

    return keyboards


skip_shift = False
skip_ctrl = False
skip_alt = False
skip_meta = False


def decode_event(kb, evt: evdev.events.InputEvent):
    global skip_shift, skip_ctrl, skip_alt, skip_meta
    cat: evdev.events.KeyEvent = evdev.categorize(evt)

    st = ""
    if cat.scancode in scancodes:
        st = scancodes[cat.scancode]
        if st[0] == "\x1b":
            pass
        if len(st) > 1 and ord(st[0]) < 255:
            st = "\x1b[94;1m<" + st + ">\x1b[0m"
        elif ord(st[0]) > 255:
            st = "\x1b[94;1m" + st + "\x1b[0m"
        else:
            st = st.lower()
    else:
        st = f"\x1b[92;1m<{cat.scancode}:" + str(cat.keycode) + ">\x1b[0m"

    if st not in mods and evt.value == 0:
        return
    elif st in mods and evt.value != 0:
        return

    if skip_shift and st == mod_SHIFT:
        skip_shift = False
        return
    if skip_ctrl and st == mod_CTRL:
        skip_ctrl = False
        return
    if skip_alt and st == mod_ALT:
        skip_alt = False
        return
    if skip_meta and st == mod_META:
        skip_meta = False
        return

    for key in kb.active_keys(verbose=True):
        match key[0]:
            case "KEY_LEFTSHIFT" | "KEY_RIGHTSHIFT":
                skip_shift = True
                if st in shifts:
                    st = shifts[st]
                else:
                    st = st.replace("94", "91")
            case "KEY_LEFTCTRL" | "KEY_RIGHTCTRL":
                st = mod_CTRL.replace("94", "91") + st
                skip_ctrl = True
            case "KEY_LEFTMETA" | "KEY_RIGHTMETA":
                st = mod_META.replace("94", "91") + st
                skip_meta = True
            case "KEY_LEFTALT" | "KEY_RIGHTALT":
                st = mod_ALT.replace("94", "91") + st
                skip_alt = True

    history.insert(0, st)


def print_history():
    global history
    st = ""
    last_st = ""
    last_k = ""
    k_count = 1
    w = os.get_terminal_size().columns
    ln = 0
    ls = []

    for k in history:
        new_st = st
        if k == last_k:
            k_count += 1
            new_st = k + f"\x1b[95;3m×{k_count}\x1b[0m " + last_st
        else:
            last_st = st
            k_count = 1
            new_st = k + " " + st
            last_k = k
        ln = len(re.sub("\x1b\\[(\\d+|\\d+;\\d+)m", "", new_st))
        if ln > w:
            ln = len(re.sub("\x1b\\[(\\d+|\\d+;\\d+)m", "", st))
            history = ls
            return print("\x1b[H\x1b[2J" + (" " * (w - ln)) + st.strip())
        st = new_st
        ls.append(k)
    print("\x1b[H\x1b[2J" + (" " * (w - ln)) + st.strip())


def listen(keyboard: evdev.InputDevice):
    for evt in keyboard.read_loop():
        if evt.type == evdev.ecodes.EV_KEY:
            decode_event(keyboard, evt)
            print_history()


def main():
    keyboards = grab_keyboards()

    if len(keyboards) == 0:
        print("No keyboards found. Try running as \x1b[91;1mroot\x1b[0m.")
        return

    for kb in keyboards:
        thread = threading.Thread(target=listen, args=(kb,))
        thread.start()

    print_history()


if __name__ == "__main__":
    main()
