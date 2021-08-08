# Gamebert

A Game Boy emulator written by Robert, in Go.

## Running it

1. Download [Gameboy bootrom](https://gbdev.gg8.se/files/roms/bootroms/) and save at `./dmg_boot.bin` (TODO: make this easier)
2. Edit `main.go` to point at [a ROM that you've downloaded](https://www.emulatorgames.net/roms/gameboy/) (TODO: add as a command line flag)
3. `go run .`

## Is Gamebert any good?

Overall I think it's alright!

+:
* It pretty much works!
* I think that the code is relatively clean
* I think that using an `opcodes.json` file abstracts away a lot of tedium

-:
* Probably plenty of bugs
* No command line flags
* Plenty of wonky design decisions that I wouldn't repeat if I did the project again
* Sound not implemented
