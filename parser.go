package main

import (
	"errors"
	"io"
	"regexp"
	"strconv"
	"unicode"
)

func (ed *Editor) parse() error {
	var (
		addr int
		err  error
	)
	ed.addrc = 0
	ed.start = ed.dot
	ed.end = ed.dot
	for {
		addr, err = ed.nextAddress()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		if addr < 1 {
			break
		}
		ed.addrc++
		ed.start = ed.end
		ed.end = addr
		if ed.tok != ',' && ed.tok != ';' {
			break
		} else if ed.token() == ';' {
			ed.dot = addr
		}
	}
	if ed.addrc = min(ed.addrc, 2); ed.addrc == 1 || ed.end != addr {
		ed.start = ed.end
	}

	return nil
}

func (ed *Editor) nextAddress() (int, error) {
	var (
		addr  = ed.dot
		err   error
		first = true
	)
	ed.skipWhitespace()
	startpos := ed.tokpos
	for starttok := ed.tok; ; first = false {
		switch {
		case unicode.IsDigit(ed.tok) || ed.tok == '+' || ed.tok == '-' || ed.tok == '^':
			mod := ed.tok
			if !unicode.IsDigit(mod) {
				ed.token()
			}
			var n int
			ed.skipWhitespace()
			if unicode.IsDigit(ed.tok) {
				var s string
				for unicode.IsDigit(ed.tok) {
					s += string(ed.tok)
					ed.token()
				}
				n, err = strconv.Atoi(s)
				if err != nil {
					return -1, err
				}
			} else if !unicode.IsSpace(mod) {
				n = 1
			}
			switch mod {
			case '-', '^':
				addr -= n
			case '+':
				addr += n
			default:
				addr = n
			}
		case ed.tok == '.' || ed.tok == '$':
			if ed.tokpos != startpos {
				return -1, ErrInvalidAddress
			}
			addr = len(ed.Lines)
			if ed.tok == '.' {
				addr = ed.dot
			}
			ed.token()
		case ed.tok == '?', ed.tok == '/':
			if !first {
				return -1, ErrInvalidAddress
			}
			var mod = ed.tok
			ed.token()
			var search = ed.scanStringUntil(mod)
			if ed.tok == mod {
				ed.token()
			}
			if search == "" {
				search = ed.search
				if ed.search == "" {
					return -1, ErrNoPrevPattern
				}
			}
			ed.search = search
			var s, e = 0, len(ed.Lines)
			if mod == '?' {
				s = ed.start - 2
				e = 0
			}
			for i := s; i != e; { //i > s && i < e; {
				match, err := regexp.MatchString(search, ed.Lines[i])
				if err != nil {
					return 0, ErrNoMatch
				}
				if match {
					addr = i + 1
					return addr, nil
				}
				if mod == '/' {
					i++
				} else {
					i--
				}
			}
			return -1, ErrNoMatch
		case ed.tok == '\'':
			if !first {
				return -1, ErrInvalidAddress
			}
			var r = ed.token()
			ed.token()
			if r == EOF || !unicode.IsLower(r) {
				return -1, ErrInvalidMark
			}
			var mark = int(r) - 'a'
			if mark < 0 || mark > len(ed.mark) {
				return -1, ErrInvalidMark
			}
			var maddr = ed.mark[mark]
			if maddr < 1 || maddr > len(ed.Lines) {
				return -1, ErrInvalidAddress
			}
			addr = maddr
		case ed.tok == '%' || ed.tok == ',' || ed.tok == ';':
			if first {
				ed.addrc++
				ed.end = 1
				if ed.tok == ';' {
					ed.end = ed.dot
				}
				ed.token()
				if addr, err = ed.nextAddress(); err != nil {
					addr = len(ed.Lines)
				}
			}
			fallthrough
		default:
			if ed.tok == starttok {
				return -1, io.EOF
			}
			if addr < 0 || addr > len(ed.Lines) {
				ed.addrc++
				return -1, ErrInvalidAddress
			}
			return addr, nil
		}
	}
}

// check validates if n, m are valid depending on how many addresses were
// previously parsed. check returns error "invalid address" if the
// positions are out of bounds.
func (ed *Editor) check(n, m int) error {
	if ed.addrc == 0 {
		ed.start = n
		ed.end = m
	}
	if ed.start > ed.end || ed.start < 1 || ed.end > len(ed.Lines) {
		return ErrInvalidAddress
	}
	return nil
}

// scanString scans the user input until EOF or new line.
func (ed *Editor) scanString() string {
	var str string
	for ed.tok != EOF && ed.tok != '\n' {
		str += string(ed.tok)
		ed.token()
	}
	return str
}

// scanStringUntil works like `scanString` but will continue until it
// sees (and consumes) `delim`. If `delim` is not found it continues
// until EOF.
func (ed *Editor) scanStringUntil(delim rune) string {
	var str string
	for ed.tok != EOF && ed.tok != '\n' && ed.tok != delim {
		str += string(ed.tok)
		ed.token()
	}
	if ed.tok == delim {
		ed.token()
	}
	return str
}

func (e *Editor) skipWhitespace() {
	for e.tok == ' ' || e.tok == '\t' {
		e.token()
	}
}