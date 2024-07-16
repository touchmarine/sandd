// Original: https://github.com/google/codesearch/blob/v1.3.0-rc.1/regexp/match.go
//
// Changelog:
//  - add Grep.OnMatch
//  - export lineContext
//
// Original notice:
//  Copyright 2020 The Go Authors. All rights reserved.
//  Use of this source code is governed by a BSD-style
//  license that can be found in the LICENSE file.

package codesearchpatch

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"strings"

	"github.com/google/codesearch/regexp"
)

type Grep struct {
	Regexp *regexp.Regexp // regexp to search for
	Stdout io.Writer      // output target
	Stderr io.Writer      // error target

	L bool // L flag - print file names only
	C bool // C flag - print count of matches
	N bool // N flag - print line numbers
	H bool // H flag - do not print file names
	V bool // V flag - print non-matching lines (only for cgrep, not csearch)

	HTML    bool // emit HTML output for csweb
	Match   bool // were any matches found?
	Matches int  // how many matches were found?
	Limit   int  // stop after this many matches
	Limited bool // stopped because of limit

	PreContext  int // number of lines to print after
	PostContext int // number of lines to print before
	// custom callback on match
	OnMatch func(buf []byte, name string, lineno, lineStart, lineEnd int)

	buf []byte
}

func (g *Grep) esc(s string) string {
	if g.HTML {
		return html.EscapeString(s)
	}
	return s
}

var nl = []byte{'\n'}

func countNL(b []byte) int {
	n := 0
	for {
		i := bytes.IndexByte(b, '\n')
		if i < 0 {
			break
		}
		n++
		b = b[i+1:]
	}
	return n
}

func (g *Grep) Reader(r io.Reader, name string) {
	if g.buf == nil {
		g.buf = make([]byte, 1<<20)
	}
	var (
		buf        = g.buf[:0]
		needLineno = g.N || g.HTML
		lineno     = 1
		count      = 0
		prefix     = ""
		beginText  = true
		endText    = false
	)
	if !g.H {
		prefix = name + ":"
	}
	chunkStart := 0
	for {
		n, err := io.ReadFull(r, buf[len(buf):cap(buf)])
		buf = buf[:len(buf)+n]
		end := len(buf)
		if err == nil {
			// Stop scan before trailing fragment of a line;
			// also stop before g.PostContext whole lines,
			// so we know we'll have the context we need to print.
			d := lineSuffixLen(buf, g.PostContext+1)
			if d < len(buf) {
				end = len(buf) - d
			}
		} else {
			endText = true
		}
		for chunkStart < end {
			m1 := g.Regexp.Match(buf[chunkStart:end], beginText, endText) + chunkStart
			beginText = false
			if m1 < chunkStart {
				break
			}
			g.Match = true
			if g.Limit > 0 && g.Matches >= g.Limit {
				g.Limited = true
				return
			}
			g.Matches++
			if g.L {
				if g.HTML {
					fmt.Fprintf(g.Stdout, "<a href=\"show/%s\">%s</a>\n", g.esc(name), g.esc(name))
				} else {
					fmt.Fprintf(g.Stdout, "%s\n", name)
				}
				return
			}
			lineStart := bytes.LastIndex(buf[chunkStart:m1], nl) + 1 + chunkStart
			lineEnd := m1 + 1
			if lineEnd > end {
				lineEnd = end
			}
			if needLineno {
				lineno += countNL(buf[chunkStart:lineStart])
			}
			line := buf[lineStart:lineEnd]
			nl := ""
			if len(line) == 0 || line[len(line)-1] != '\n' {
				nl = "\n"
			}
			switch {
			case g.C:
				count++
			case g.OnMatch != nil:
				g.OnMatch(buf, name, lineno, lineStart, lineEnd)
			case g.PreContext+g.PostContext > 0:
				fmt.Fprintf(g.Stdout, "%s%d:\n", prefix, lineno)
				before, match, after := LineContext(g.PreContext, g.PostContext, buf, lineStart, lineEnd)
				for _, line := range before {
					fmt.Fprintf(g.Stdout, "\t\t%s\n", line)
				}
				fmt.Fprintf(g.Stdout, "\t>>\t%s\n", match)
				for _, line := range after {
					fmt.Fprintf(g.Stdout, "\t\t%s\n", line)
				}
			case g.HTML:
				fmt.Fprintf(g.Stdout, "<a href=\"/show/%s?q=%s#L%d\">%s:%d</a>:%s%s", g.esc(strings.ReplaceAll(name, "#", ">")), g.esc(g.Regexp.String()), lineno, g.esc(name), lineno, g.esc(string(line)), nl)
			case g.N:
				fmt.Fprintf(g.Stdout, "%s%d:%s%s", prefix, lineno, line, nl)
			default:
				fmt.Fprintf(g.Stdout, "%s%s%s", prefix, line, nl)
			}
			if needLineno {
				lineno++
			}
			chunkStart = lineEnd
		}
		if needLineno && err == nil {
			lineno += countNL(buf[chunkStart:end])
		}
		// Slide pre-context and unprocessed bytes down to start of buffer.
		d := lineSuffixLen(buf[:end], g.PreContext)
		if d == end {
			// Not enough room; give up on context.
			d = 0
		}
		n = copy(buf, buf[end-d:])
		buf = buf[:n]
		chunkStart = d
		if endText && err != nil {
			if err != io.EOF && err != io.ErrUnexpectedEOF {
				fmt.Fprintf(g.Stderr, "%s: %v\n", g.esc(name), err)
			}
			break
		}
	}
	if g.C && count > 0 {
		if g.HTML {
			fmt.Fprintf(g.Stdout, "<a href=\"show/%s?q=%s\">%s</a>: %d\n", g.esc(name), g.esc(g.Regexp.String()), g.esc(name), count)
		} else {
			fmt.Fprintf(g.Stdout, "%s: %d\n", name, count)
		}
	}
}

func lineSuffixLen(buf []byte, n int) int {
	end := len(buf)
	for i := 0; i < n; i++ {
		j := bytes.LastIndex(buf[:end], nl)
		if j < 0 {
			break
		}
		end = j
	}
	if j := bytes.LastIndex(buf[:end], nl); j >= 0 {
		return len(buf) - (j + 1)
	}
	return len(buf)
}

func linePrefixLen(buf []byte, lines int) int {
	start := 0
	for i := 0; i < lines; i++ {
		j := bytes.IndexByte(buf[start:], '\n')
		if j < 0 {
			return len(buf)
		}
		start += j + 1
	}
	return start
}

// LineContext returns the given line and the surrounding lines.
func LineContext(numBefore, numAfter int, buf []byte, lineStart, lineEnd int) (before [][]byte, line []byte, after [][]byte) {
	beforeChunk := buf[lineStart-lineSuffixLen(buf[:lineStart], numBefore) : lineStart]
	afterChunk := buf[lineEnd : lineEnd+linePrefixLen(buf[lineEnd:], numAfter)]

	line = chomp(buf[lineStart:lineEnd])
	before = bytes.SplitAfter(beforeChunk, nl)
	if len(before[len(before)-1]) == 0 {
		before = before[:len(before)-1]
	}
	for i := range before {
		before[i] = chomp(before[i])
	}
	after = bytes.Split(afterChunk, nl)
	if len(after[len(after)-1]) == 0 {
		after = after[:len(after)-1]
	}
	for i := range after {
		after[i] = chomp(after[i])
	}

	var prefix []byte
	prefix = updatePrefix(prefix, line)
	for _, l := range before {
		prefix = updatePrefix(prefix, l)
	}
	for _, l := range after {
		prefix = updatePrefix(prefix, l)
	}

	line = cutPrefix(line, prefix)
	for i, l := range before {
		before[i] = cutPrefix(l, prefix)
	}
	for i, l := range after {
		after[i] = cutPrefix(l, prefix)
	}
	return
}

func updatePrefix(prefix, line []byte) []byte {
	if prefix == nil {
		i := 0
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		return line[:i]
	}

	i := 0
	for i < len(line) && i < len(prefix) && line[i] == prefix[i] {
		i++
	}
	if i >= len(line) {
		return prefix
	}
	return prefix[:i]
}

func cutPrefix(line, prefix []byte) []byte {
	if len(prefix) > len(line) {
		return nil
	}
	return line[len(prefix):]
}

func chomp(s []byte) []byte {
	i := len(s)
	for i > 0 && (s[i-1] == ' ' || s[i-1] == '\t' || s[i-1] == '\r' || s[i-1] == '\n') {
		i--
	}
	return s[:i]
}
