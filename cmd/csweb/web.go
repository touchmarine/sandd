// Original: https://github.com/google/codesearch/tree/v1.3.0-rc.1
//
// Original notice:
//  Copyright 2020 The Go Authors. All rights reserved.
//  Use of this source code is governed by a BSD-style
//  license that can be found in the LICENSE file.

package main

import (
	"archive/zip"
	"bytes"
	"embed"
	"flag"
	"fmt"
	"html"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/codesearch/index"
	"github.com/google/codesearch/regexp"
	"github.com/touchmarine/sandd/codesearchpatch"
)

var verboseFlag = flag.Bool("verbose", false, "print extra information")

func main() {
	flag.Parse()

	http.HandleFunc("GET /", home)
	http.Handle("GET /_static/", http.FileServer(http.FS(static)))
	http.HandleFunc("GET /show/", show)
	log.Fatal(http.ListenAndServe("localhost:2473", nil))
}

//go:embed _static
var static embed.FS

func home(w http.ResponseWriter, r *http.Request) {
	qarg := r.FormValue("q")
	farg := r.FormValue("f")
	isCaseSensitive := r.FormValue("case-sensitive") != ""
	isRegex := r.FormValue("regex") != ""

	replacements := []string{
		"QUERY", html.EscapeString(qarg),
		"FILE", html.EscapeString(farg),
	}
	if isCaseSensitive {
		replacements = append(replacements, "CASE-SENSITIVE", "checked")
	}
	if isRegex {
		replacements = append(replacements, "REGEX", "checked")
	}
	replaced := strings.NewReplacer(replacements...).Replace(homePage)
	w.Write([]byte(replaced))
	searchPartial(w, qarg, farg, !isRegex, !isCaseSensitive)

	w.Write([]byte(
		`
    </main>
</div>

<script src="https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.10.0/highlight.min.js"></script>
<script>hljs.highlightAll();</script>
<script>
matchesNoTop = document.getElementById('matches-no-top')
matchesNoBottom = document.getElementById('matches-no-bottom')
matchesNoTop.textContent = matchesNoBottom.textContent
document.querySelectorAll('[data-ext-pattern]').forEach((btn) => {
    btn.addEventListener('click', () => {
        const input = document.getElementById('file')
        input.value = btn.dataset.extPattern
    })
})
</script>
</body>
</html>
`))
}

const homePage = `
<!DOCTYPE html>
<html>
<head>
<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.10.0/styles/github.min.css">
<style>
header {
    margin-bottom: 32px;
}
.match {
    margin-bottom: 32px;
}
</style>
</head>

<body>
<header>
    <form style="display: flex; column-gap: 32px; text-wrap: nowrap;">
        <label for="query">Search:</label>
        <input type="search" id="query" name="q" value="QUERY" placeholder="Search (regex)" style="width: 100%;">

        <label for="file">Path:</label>
        <input type="search" id="file" name="f" value="FILE" placeholder="Filter Files (regex)" style="width: 100%;">

        <input type="checkbox" id="case-sensitive" name="case-sensitive" CASE-SENSITIVE>
        <label for="case-sensitive">Case-Sensitive</label>

        <input type="checkbox" id="regex" name="regex" REGEX>
        <label for="regex">Regular Expression</label>

        <button>Search</button>
    </form>
</header>
<div class="container">
    <!--
    <aside>
        <form>
            <fieldset>
                <legend>Paths</legend>

                <input type="checkbox" id="dbt" name="paths" value="/dbt">
                <label for="dbt">/dbt</label>
            </fieldset>
            <fieldset>
                <legend>Languages</legend>

                <input type="checkbox" id="go" name="languages" value="go">
                <label for="go">Go</label>
            </fieldset>
            <button>Update</button>
        </form>
    </aside>
    -->
    <main>
    <p id="matches-no-top" style="margin-bottom: 32px;"> matches in s</p>
`

func searchPartial(w io.Writer, qarg, farg string, literal, caseInsensitive bool) {
	var b bytes.Buffer
	prevName := ""
	g := codesearchpatch.Grep{
		N:      true,
		Limit:  10,
		Stdout: w,
		Stderr: w,
		OnMatch: func(buf []byte, name string, lineno, lineStart, lineEnd int) {
			if name == prevName {
				// the same file
				b.Reset() // clear closing tag
			} else {
				// new file
				fmt.Fprint(&b, `<div class="match">`)
				fmt.Fprintf(&b, "<p>%s (<a href=\"/show/%s\">show</a>)</p>\n", html.EscapeString(name), html.EscapeString(strings.ReplaceAll(name, "#", ">")))
			}

			fmt.Fprintf(&b, "<small style=\"float: right;\"><a href=\"/show/%s#L%d\">#%d</a></small>\n", html.EscapeString(strings.ReplaceAll(name, "#", ">")), lineno, lineno)
			fmt.Fprint(&b, "<pre><code>")
			before, match, after := codesearchpatch.LineContext(1, 1, buf, lineStart, lineEnd)
			for _, line := range before {
				fmt.Fprintf(&b, "%s\n", line)
			}
			fmt.Fprintf(&b, "%s\n", match)
			for _, line := range after {
				fmt.Fprintf(&b, "%s\n", line)
			}
			fmt.Fprint(&b, "</code></pre>\n")

			b.WriteTo(w) // flush

			// Buffer the closing tag so we have all match's html here. The buffer is:
			// - reset if the next match is in the same file,
			// - flushed if the next match is not in the same file or if end of matches.
			fmt.Fprint(&b, "</div>\n")

			prevName = name
		},
	}

	afterReader := func() {
		// flush any unread buffer (should be closing div tag)
		b.WriteTo(w)
	}

	pat := qarg
	if literal {
		pat = backslashEscapeAllPunctuation(pat)
	}
	pat = "(?m)" + pat // multiline: ^ and $ match begin/end line in addition to begin/end text
	if caseInsensitive {
		pat = "(?i)" + pat // case-insensitive
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		fmt.Fprintf(w, "Bad query: %v\n", err)
		return
	}
	g.Regexp = re
	var fre *regexp.Regexp
	if farg != "" {
		fre, err = regexp.Compile(farg)
		if err != nil {
			fmt.Fprintf(w, "Bad -f flag: %v\n", err)
			return
		}
	}
	q := index.RegexpQuery(re.Syntax)
	if *verboseFlag {
		log.Printf("query: %s\n", q)
	}

	start := time.Now()
	ix := index.Open(index.File())
	ix.Verbose = *verboseFlag
	post := ix.PostingQuery(q)
	if *verboseFlag {
		fmt.Fprintf(w, "post query identified %d possible files\n", len(post))
	}

	exts := map[string]int{}
	for _, fileid := range post {
		name := ix.Name(fileid).String()
		ext := filepath.Ext(name)
		// trigram match count, not actual matched files count
		exts[ext]++
	}

	if fre != nil {
		fnames := make([]int, 0, len(post))

		for _, fileid := range post {
			name := ix.Name(fileid)
			if fre.MatchString(name.String(), true, true) < 0 {
				continue
			}
			fnames = append(fnames, fileid)
			exts[filepath.Ext(name.String())]++
		}

		if *verboseFlag {
			fmt.Fprintf(w, "filename regexp matched %d files\n", len(fnames))
		}
		post = fnames
	}

	// sort extensions by count desc
	type extInfo struct {
		ext   string
		count int
	}
	exts2 := make([]extInfo, 0, len(exts))
	for ext, count := range exts {
		exts2 = append(exts2, extInfo{ext: ext, count: count})
	}
	sort.Slice(exts2, func(i, j int) bool {
		return exts2[i].count > exts2[j].count
	})

	for _, e := range exts2 {
		// Don't show count as it's misleading since it's not the actual count
		// (this serves as a plain suggestion).
		fmt.Fprintf(w, "<button data-ext-pattern=\".*\\%s$\">%s</button>\n", e.ext, e.ext)
	}

	var (
		zipFile   string
		zipReader *zip.ReadCloser
		zipMap    map[string]*zip.File
	)

	for _, fileid := range post {
		if g.Limited {
			break
		}
		name := ix.Name(fileid).String()
		file, err := os.Open(name)
		if err != nil {
			if i := strings.Index(name, ".zip\x01"); i >= 0 {
				zfile, zname := name[:i+4], name[i+5:]
				if zfile != zipFile {
					if zipReader != nil {
						zipReader.Close()
						zipMap = nil
					}
					zipFile = zfile
					zipReader, err = zip.OpenReader(zfile)
					if err != nil {
						zipReader = nil
					}
					if zipReader != nil {
						zipMap = make(map[string]*zip.File)
						for _, file := range zipReader.File {
							zipMap[file.Name] = file
						}
					}
				}
				file := zipMap[zname]
				if file != nil {
					r, err := file.Open()
					if err != nil {
						continue
					}
					g.Reader(r, name)
					afterReader()
					r.Close()
					continue
				}
			}
			continue
		}
		g.Reader(file, name)
		afterReader()
		file.Close()
	}

	fmt.Fprintf(w, "\n<p id='matches-no-bottom'>%d matches in %.3fs</p>\n", g.Matches, time.Since(start).Seconds())
	if g.Limited {
		fmt.Fprintf(w, "<p>more matches not shown due to match limit</p>\n")
	}
}

func show(w http.ResponseWriter, r *http.Request) {
	file := strings.TrimPrefix(r.URL.Path, "/show")
	if strings.HasPrefix(file, "/") && filepath.IsAbs(file[1:]) {
		// Turn /c:/foo into c:/foo on Windows.
		file = file[1:]
	}
	// TODO maybe trim file by ix.roots
	// TODO zips
	info, err := os.Stat(file)
	if err != nil {
		// TODO
		http.Error(w, err.Error(), 500)
		return
	}
	if info.IsDir() {
		dirs, err := os.ReadDir(file)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Write(serveDir(file, dirs))
		return
	}

	data, err := os.ReadFile(file)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Write(serveFile(file, data))
}

func printHeader(buf *bytes.Buffer, file string) {
	e := html.EscapeString
	buf.WriteString("<!DOCTYPE html>\n<head>\n")
	buf.WriteString("<link rel=\"stylesheet\" href=\"/_static/viewer.css\">\n")
	buf.WriteString("<script src=\"/_static/viewer.js\"></script>\n")
	fmt.Fprintf(buf, `<title>%s - code search</title>`, e(file))
	buf.WriteString("\n</head><body onload=\"highlight()\"><pre>\n")
	f := ""
	for _, elem := range strings.Split(file, "/") {
		f += "/" + elem
		fmt.Fprintf(buf, `/<a href="/show%s">%s</a>`, e(f), e(elem))
	}
	fmt.Fprintf(buf, `</b> <small>(<a href="/">about</a>)</small>`)
	fmt.Fprintf(buf, "\n\n")
}

func serveDir(file string, dir []fs.DirEntry) []byte {
	var buf bytes.Buffer
	e := html.EscapeString
	printHeader(&buf, file)
	for _, d := range dir {
		// Note: file is the full path including mod@vers.
		file := path.Join(file, d.Name())
		fmt.Fprintf(&buf, "<a href=\"/show%s\">%s</a>\n", e(file), e(path.Base(file)))
	}
	return buf.Bytes()
}

var nl = []byte("\n")

func serveFile(file string, data []byte) []byte {
	if !isText(data) {
		return data
	}

	var buf bytes.Buffer
	e := html.EscapeString
	printHeader(&buf, file)
	n := 1 + bytes.Count(data, nl)
	wid := len(fmt.Sprintf("%d", n))
	wid = (wid+2+7)&^7 - 2
	n = 1
	for len(data) > 0 {
		var line []byte
		line, data, _ = bytes.Cut(data, nl)
		fmt.Fprintf(&buf, "<span id=\"L%d\">%*d  %s\n</span>", n, wid, n, e(string(line)))
		n++
	}
	return buf.Bytes()
}

// isText reports whether a significant prefix of s looks like correct UTF-8;
// that is, if it is likely that s is human-readable text.
func isText(s []byte) bool {
	const max = 1024 // at least utf8.UTFMax
	if len(s) > max {
		s = s[0:max]
	}
	for i, c := range string(s) {
		if i+utf8.UTFMax > len(s) {
			// last char may be incomplete - ignore
			break
		}
		if c == 0xFFFD || c < ' ' && c != '\n' && c != '\t' && c != '\f' {
			// decoding error or control character - not a text file
			return false
		}
	}
	return true
}

// This map is based on a codesearch test, it's not from a definitive source:
// https://github.com/google/codesearch/blob/b34f2a0c5ce12be3c9dc28038640afece6bee523/regexp/regexp_test.go#L136
func backslashEscapeAllPunctuation(s string) string {
	r := strings.NewReplacer(
		`!`, `\!`,
		`"`, `\"`,
		`#`, `\#`,
		`$`, `\$`,
		`%`, `\%`,
		`&`, `\&`,
		`'`, `\'`,
		`(`, `\(`,
		`)`, `\)`,
		`*`, `\*`,
		`+`, `\+`,
		`,`, `\,`,
		`-`, `\-`,
		`.`, `\.`,
		`/`, `\/`,
		`:`, `\:`,
		`;`, `\;`,
		`<`, `\<`,
		`=`, `\=`,
		`>`, `\>`,
		`?`, `\?`,
		`@`, `\@`,
		`[`, `\[`,
		`\`, `\\`,
		`]`, `\]`,
		`^`, `\^`,
		`_`, `\_`,
		`{`, `\{`,
		`|`, `\|`,
		`}`, `\}`,
		`~`, `\~`,
	)
	return r.Replace(s)
}
