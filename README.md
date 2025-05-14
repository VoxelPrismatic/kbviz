# kbviz

Keyboard visualizer

## Go Version (binary)

> [!NOTE]
> Feel free to build yourself, but beware that `mappu/miqt` literally takes hours to build.
> I am not going to hack you, the binary is safe.

1. Supports Qt6
   - When running in the terminal, pass the `-gui` flag to launch the GUI
2. Will automatically escalate to root
   - Make sure `pkexec` is available. This is used to escalate to root when no terminal is available (eg running from a `.desktop` file)
3. Designed for Wayland & evdev
4. Customize the output
   - Ignore key events
   - Listen to key events
     - Trackpad events are ignored by default, so this may be useful to you
   - Customize output string
   - Customize colors
   - Customize font
   - `-h` for help
5. Dead-simple sizing
   - Always one row, and it fits as many squares as possible
6. No wierd terminal nonsense

Preview:

## Python version

1. Must be run as root
2. Be sure to pip install `evdev`
3. Only tested on Linux
4. Designed for Wayland
5. Use a [Nerd Font](https://nerdfonts.com). Lots of symbols (ctrl, esc, alt, meta, etc) come from there

Preview:

https://github.com/user-attachments/assets/91ec164b-a3f4-453e-bf89-e5e72cd3d2c9

> \*_apologies for the recording bugs, spectacle isn't perfect_

> [!NOTE]
> If you're using KDE Konsole, you need at least two lines, otherwise it won't display anything.
> This does not appear to be an issue with other emulators, like Ghostty.

Neovim plugin: [Rabbit.nvim](https://github.com/voxelprismatic/rabbit.nvim)
