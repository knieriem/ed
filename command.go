package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"unicode"

	"os"
	"text/scanner"
)

func (ed *Editor) DoCommand() error {
	log.Printf("Cmd='%c' (EOF=%t)\n", ed.token(), ed.token() == scanner.EOF)

	// FIXME: We might need to check the bounds in some of these commands
	// adding a ed.checkRanges() here will block the user from inserting
	// text if the start and end values are invalid.

	switch ed.token() {
	case 'a':
		for {
			line, err := ed.ReadInsert()
			if err != nil {
				ed.setupSignals()
				return nil
			}
			if line == "." {
				break
			}

			if len(ed.Lines) == ed.End {
				ed.Lines = append(ed.Lines, line)
				ed.End++
				continue
			}
			ed.Lines = append(ed.Lines[:ed.End+1], ed.Lines[ed.End:]...)
			ed.Lines[ed.End] = line
			ed.Dirty = true
		}
		return nil

	case 'c':
		ed.Dirty = true
		ed.Lines = append(ed.Lines[:ed.Start-1], ed.Lines[ed.End:]...)
		ed.End = ed.Start - 1
		for {
			line, err := ed.ReadInsert()
			if err != nil {
				ed.setupSignals()
				return nil
			}
			if line == "." {
				break
			}
			ed.Lines = append(ed.Lines[:ed.End+1], ed.Lines[ed.End:]...)
			ed.Lines[ed.End] = line
			ed.End++
		}
		return nil

	case 'd':
		ed.Dirty = true
		ed.Lines = append(ed.Lines[:ed.Start-1], ed.Lines[ed.End:]...)
		if ed.Start > len(ed.Lines) {
			ed.Start = len(ed.Lines)
		}
		ed.Dot = ed.Start
		ed.End = ed.Dot
		ed.Start = ed.Dot
		return nil

	case 'E':
		fallthrough
	case 'e':
		var uc bool = (ed.token() == 'E')
		ed.nextToken()
		ed.nextToken()
		var cmd bool
		if ed.token() == '!' {
			ed.nextToken()
			cmd = true
		}
		ed.skipWhitespace()
		var fname string = ed.scanString()
		switch cmd {
		case true:
			if fname == "" && ed.Cmd != "" {
				fname = ed.Cmd
			}
			log.Printf("e command '%s'\n", fname)
			lines, err := ed.Shell(fname)
			if err != nil {
				return ErrZero
			}
			var siz int
			for i := range lines {
				siz += len(lines[i]) + 1
			}
			ed.Lines = lines
			ed.Dot = len(ed.Lines)
			ed.Start = ed.Dot
			ed.End = ed.Dot
			ed.addr = -1
			fmt.Fprintf(ed.err, "%d\n", siz)
		case false:
			if fname == "" && ed.Path == "" {
				return ErrNoFileName
			}
			if !uc && ed.Dirty {
				ed.Dirty = false
				return ErrFileModified
			}
			if fname == "" {
				fname = ed.Path
			}
			if err := ed.ReadFile(fname); err != nil {
				return err
			}
			log.Printf("Path: '%s'\n", ed.Path)
		}
		return nil

	case 'f':
		ed.nextToken()
		log.Printf("Token=%c\n", ed.token())
		if ed.token() == scanner.EOF {
			if ed.Path == "" {
				return ErrNoFileName
			}
			fmt.Fprintf(ed.err, "%s\n", ed.Path)
			return nil
		}
		ed.nextToken()
		var fname string = ed.scanString()
		log.Printf("Filename: '%s'\n", fname)
		if fname == "" {
			return ErrNoFileName
		}
		ed.Path = fname
		fmt.Fprintf(ed.err, "%s\n", ed.Path)
		return nil

	case 'g':
		return fmt.Errorf("TODO: g (global) not implemented")

	case 'G':
		return fmt.Errorf("TODO: G (interactive g) not implemented")

	case 'H':
		ed.printErrors = !ed.printErrors
		return nil

	case 'h':
		if ed.Error != nil {
			fmt.Fprintf(ed.err, "%s\n", ed.Error)
		}
		return nil

	case 'i':
		for {
			line, err := ed.ReadInsert()
			if err != nil {
				ed.setupSignals()
				goto end_insert
			}
			if line == "." {
				break
			}
			if ed.End-1 < 0 {
				ed.End++
			}
			if ed.End > len(ed.Lines) {
				ed.Lines = append(ed.Lines, line)
				ed.End++
				continue
			}
			if ed.End < 0 {
				return ErrInvalidAddress
			}
			ed.Lines = append(ed.Lines[:ed.End], ed.Lines[ed.End-1:]...)
			ed.Lines[ed.End-1] = line
			ed.End++
			ed.Dirty = true
		}
		ed.End--
	end_insert:
		ed.Dot = ed.End
		ed.Start = ed.Dot
		ed.addr = ed.Dot
		return nil

	case 'j':
		if ed.End == ed.Start {
			ed.End++
		}
		if ed.End > len(ed.Lines) {
			return ErrInvalidAddress
		}
		var joined string = strings.Join(ed.Lines[ed.Start-1:ed.End], "")
		var result []string = append(append([]string{}, ed.Lines[:ed.Start-1]...), joined)
		ed.Lines = append(result, ed.Lines[ed.End:]...)
		ed.Dot = ed.Start
		ed.End = ed.Dot
		ed.addr = ed.Dot
		ed.Dirty = true
		return nil

	case 'k':
		ed.nextToken()
		var buf string = ed.scanString()
		switch len(buf) {
		case 1:
			break
		case 0:
			fallthrough
		default:
			return ErrInvalidCmdSuffix
		}
		var r rune = rune(buf[0])
		if !unicode.IsLower(r) {
			return ErrInvalidMark
		}
		log.Printf("Mark %c\n", r)
		var mark int = int(byte(buf[0])) - 'a'
		if mark >= len(ed.Mark) {
			return ErrInvalidMark
		}
		log.Printf("Mark %d is set to End (%d)\n", mark, ed.End)
		ed.Mark[int(mark)] = ed.End
		return nil

	case 'm':
		var err error
		var dst int
		ed.nextToken()
		log.Printf("Destination: %c\n", ed.token())
		var arg string = ed.scanString()
		dst, err = strconv.Atoi(arg)
		if err != nil {
			return ErrDestinationExpected
		}
		if dst < 0 || dst > len(ed.Lines) {
			// TODO: OpenBSD ed will evaluate the destination address,
			// so `24,26m-5` is actually a valid command
			return ErrDestinationExpected
		}
		log.Printf("Destination (arg): %d (%s)\n", dst, arg)
		lines := make([]string, ed.End-ed.Start+1)
		copy(lines, ed.Lines[ed.Start-1:ed.End+1])
		ed.Lines = append(ed.Lines[:ed.Start-1], ed.Lines[ed.End:]...)
		ed.Lines = append(ed.Lines[:dst], append(lines, ed.Lines[dst:]...)...)
		ed.Dot = dst + len(lines)
		ed.End = ed.Dot
		ed.Start = ed.Dot
		return err

	case 'l':
		fallthrough
	case 'n':
		fallthrough
	case 'p':
		for i := ed.Start - 1; i < ed.End; i++ {
			if i < 0 {
				continue
			}
			switch ed.token() {
			case 'l':
				var q string = strconv.QuoteToASCII(ed.Lines[i])
				fmt.Fprintf(ed.out, "%s$\n", q[1:len(q)-1])
			case 'n':
				fmt.Fprintf(ed.out, "%d\t%s\n", i+1, ed.Lines[i])
			case 'p':
				fmt.Fprintf(ed.out, "%s\n", ed.Lines[i])
			}
		}
		return nil

	case 'P':
		if ed.Prompt == 0 {
			ed.Prompt = defaultPrompt
		} else {
			ed.Prompt = 0
		}
		return nil

	case 'q':
		fallthrough
	case 'Q':
		if ed.token() == 'q' && ed.Dirty {
			ed.Dirty = false
			return ErrFileModified
		}
		os.Exit(0)
		return nil

	case 'r':
		return fmt.Errorf("TODO: r (read) not implemented")

	case 's':
		return fmt.Errorf("TODO: s (substitute) not implemented")

	case 't':
		ed.nextToken()
		if ed.token() == scanner.EOF {
			return ErrDestinationExpected
		}
		dst, err := ed.scanNumber()
		if err != nil {
			return ErrDestinationExpected
		}
		if err := ed.checkRange(); err != nil {
			return err
		}
		if ed.Start-1 < 0 {
			return ErrInvalidAddress
		}
		log.Printf("Copying %d,%d to %d\n", ed.Start, ed.End, dst)
		var lines []string = make([]string, ed.End-ed.Start+1)
		copy(lines, ed.Lines[ed.Start-1:ed.End])
		ed.Lines = append(ed.Lines[:dst], append(lines, ed.Lines[dst:]...)...)
		ed.Dot = dst + len(lines)
		return nil

	case 'u':
		return fmt.Errorf("TODO: u (undo) not implemented")

	case 'v':
		return fmt.Errorf("TODO: v (inverse g) not implemented")

	case 'V':
		return fmt.Errorf("TODO: V (inverse V) not implemented")

	case 'W':
		fallthrough
	case 'w':
		var quit bool
		var r rune = ed.token()
		var full bool = (ed.s.Pos().Offset == 1)
		ed.nextToken()
		if r == 'w' {
			log.Printf("Write\n")
			if ed.token() == 'q' {
				ed.nextToken()
				quit = true
				log.Printf("Quit=%t\n", quit)
			}
		} else {
			log.Printf("Write (Append)\n")
		}
		if ed.token() == ' ' {
			ed.nextToken()
		}
		var fname string = ed.scanString()
		if fname == "" && ed.Path == "" {
			return ErrNoFileName
		}
		log.Printf("ed.Path: '%s'\n", ed.Path)
		if fname == "" {
			fname = ed.Path
		}
		var s int = ed.Start
		var e int = ed.End
		if full {
			log.Printf("Writing the whole file\n")
			s = 1
			e = len(ed.Lines)
		}
		var err error
		if r == 'w' {
			err = ed.WriteFile(s, e, fname)
		} else {
			err = ed.AppendFile(s, e, fname)
		}
		if quit {
			os.Exit(0)
		}
		return err

	case 'z':
		return fmt.Errorf("TODO: z (scroll) not implemented")

	case '=':
		fmt.Fprintf(ed.out, "%d\n", len(ed.Lines))
		return nil

	case '!':
		ed.nextToken()
		ed.skipWhitespace()
		var buf string
		if ed.token() == scanner.EOF {
			if ed.Cmd != "" {
				buf = ed.Cmd
			} else {
				return ErrNoCmd
			}
		} else {
			buf = ed.scanString()
		}
		log.Printf("Command (unparsed): '%s'\n", buf)
		output, err := ed.Shell(buf)
		if err != nil {
			return err
		}
		for i := range output {
			fmt.Fprintf(ed.err, "%s\n", output[i])
		}
		fmt.Fprintln(ed.err, "!")
		return nil

	case 0:
		fallthrough
	case scanner.EOF:
		if ed.End-1 < 0 || ed.End-1 > len(ed.Lines) {
			return ErrInvalidAddress
		}
		fmt.Fprintf(ed.out, "%s\n", ed.Lines[ed.End-1])
		return nil
	default:
		return ErrUnknownCmd
	}
}
