// Copyright 2020 Andrew Archibald. Licensed under the MIT License.

// Command tetrisrun is a driver for Peter De Wachter's Whitespace
// Tetris game. It introduces gravity and provides several key mappings.
//
// For better results, disable input processing and echo back using
// stty, then run tetris.ws with tetrisrun piped into it.
//
// For example:
//
//     nebula-compile tetris.ws tetris
//     go build -o tetrisrun/tetrisrun tetrisrun/tetrisrun.go
//     stty raw -echo && tetrisrun/tetrisrun | ./tetris
//
// Controls:
//
//     i / w / up arrow - rotate
//     j / a / left arrow - move left
//     k / s / down arrow - drop
//     l / d / right arrow - move right
//     ESC / q - quit
//
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	escTimeout      = 100 * time.Millisecond
	initialDropRate = 1000 * time.Millisecond
	finalDropRate   = 400 * time.Microsecond
	dropRateDelta   = 1 * time.Millisecond
)

var (
	stdin = bufio.NewReader(os.Stdin)
	done  = make(chan bool)
	pause = make(chan bool)
)

func main() {
	signal.Ignore(syscall.SIGPIPE)
	dropRate := initialDropRate

	// Forward key presses to stdout
	go func() {
		for {
			select {
			default:
				key, err := readKey()
				if err != nil {
					if err != io.EOF {
						fmt.Fprintln(os.Stderr, err)
					}
					writeByte('\x1b') // ESC quits the game
					done <- true
					return
				}
				if !writeByte(key) {
					return
				}
			case <-done:
				return
			}
		}
	}()

	// Move block downwards
Drop:
	for {
		select {
		case <-time.After(dropRate):
			if !writeByte('k') {
				break Drop
			}
			if dropRate > finalDropRate {
				dropRate -= dropRateDelta
			}
		case <-pause:
			<-pause
		case <-done:
			break Drop
		}
	}
}

func writeByte(b byte) bool {
	_, err := os.Stdout.Write([]byte{b})
	if err != nil {
		// Suppress SIGPIPE
		if pe, ok := err.(*os.PathError); !ok || pe.Err != syscall.EPIPE {
			fmt.Fprintln(os.Stderr, err)
		}
		done <- true
		return false
	}
	return true
}

// readKey reads a key press and handles key aliases. Arrow keys and
// wasd are translated to ijjl; q and various control keys are
// translated to quit.
func readKey() (byte, error) {
	for {
		b, err := stdin.ReadByte()
		if err != nil {
			return 0, err
		}
		switch b {
		case 'i', 'w': // up
			return 'i', nil
		case 'j', 'a': // left
			return 'j', nil
		case 'k', 's': // down
			return 'k', nil
		case 'l', 'd': // right
			return 'l', nil
		case 'q', '\x00', '\x03', '\x04', '\x1a': // q, ^@, ^C, ^D, ^Z
			return 0, io.EOF
		case 'p', ' ':
			pause <- true
			for {
				b, err := stdin.ReadByte()
				if err != nil {
					return 0, err
				}
				if b == 'p' || b == ' ' {
					break
				}
			}
			pause <- false
		case '\x1b': // ESC
			// Translate the ANSI escape sequences for arrow keys into ijkl
			// and quit on ESC key press. If a bracket is not read within
			// escTimeout, it is treated as plain ESC.
			readBracket := make(chan bool, 1)
			go func() {
				// Try to read the next character
				b, err := stdin.ReadByte()
				readBracket <- err == nil && b == '['
			}()
			select {
			// Handle ANSI arrow key escape sequences
			case isBracket := <-readBracket:
				if !isBracket {
					return 0, io.EOF
				}
				b, err := stdin.ReadByte()
				if err != nil {
					return 0, err
				}
				switch b {
				case 'A': // up
					return 'i', nil
				case 'B': // down
					return 'k', nil
				case 'C': // right
					return 'l', nil
				case 'D': // left
					return 'j', nil
				}
			// Timeout for lone ESC
			case <-time.After(escTimeout):
				return 0, io.EOF
			}
		}
	}
}
