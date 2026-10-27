package printtpl

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/ivm97/GoIrbis/irbis"
)

// Store loads RTF/text templates from a directory.
type Store struct {
	dir string
}

func NewStore(dir string) *Store {
	if dir == "" {
		dir = "templates"
	}
	return &Store{dir: dir}
}

func (s *Store) Dir() string { return s.dir }

// List returns template basenames without extension (shabl, dopshab, …).
func (s *Store) List() ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".txt" && ext != ".rtf" {
			continue
		}
		out = append(out, strings.TrimSuffix(name, ext))
	}
	sort.Strings(out)
	return out, nil
}

// HasRepeating reports whether the template uses #D2#…#2D# row blocks.
func (s *Store) HasRepeating(name string) bool {
	raw, _, err := s.load(name)
	if err != nil {
		return false
	}
	return bytes.Contains(raw, []byte("#D2#")) && bytes.Contains(raw, []byte("#2D#"))
}

func (s *Store) load(name string) ([]byte, string, error) {
	base := filepath.Base(name)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	if base == "" || strings.Contains(base, "..") {
		return nil, "", fmt.Errorf("invalid template name")
	}
	candidates := []string{
		filepath.Join(s.dir, base+".txt"),
		filepath.Join(s.dir, base+".rtf"),
		filepath.Join(s.dir, strings.ToUpper(base)+".TXT"),
		filepath.Join(s.dir, strings.ToUpper(base)+".RTF"),
	}
	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if err == nil {
			return data, filepath.Ext(p), nil
		}
	}
	return nil, "", fmt.Errorf("template %q not found", base)
}

var leftoverKey = regexp.MustCompile(`#[A-Za-z][A-Za-z0-9_]*#`)

// Render fills a template for one or many records.
// Inventory-style templates (dopshab) repeat the block between #D2# and #2D#.
// Card-style templates (shabl) are rendered once per record and joined.
func (s *Store) Render(name string, recordsKeys []map[string]string, extras map[string]string) ([]byte, string, error) {
	raw, ext, err := s.load(name)
	if err != nil {
		return nil, "", err
	}
	base := strings.ToLower(strings.TrimSuffix(filepath.Base(name), filepath.Ext(name)))
	if base == "" {
		base = strings.ToLower(name)
	}

	var out []byte
	if bytes.Contains(raw, []byte("#D2#")) && bytes.Contains(raw, []byte("#2D#")) {
		out, err = renderRepeating(raw, recordsKeys, extras)
	} else {
		out, err = renderCards(raw, recordsKeys, extras)
	}
	if err != nil {
		return nil, "", err
	}
	out = ensureRTFDocument(out)
	if ext == "" || strings.EqualFold(ext, ".txt") {
		ext = ".rtf"
	}
	return out, ext, nil
}

// ensureRTFDocument wraps legacy section fragments (shabl/dopshab) into a valid RTF file.
func ensureRTFDocument(body []byte) []byte {
	trimmed := bytes.TrimSpace(body)
	if bytes.HasPrefix(trimmed, []byte(`{\rtf`)) {
		return body
	}
	// Old templates often start with "\sect }" closing a previous group.
	body = bytes.TrimPrefix(body, []byte(`\sect }`))
	var buf bytes.Buffer
	buf.WriteString(`{\rtf1\ansi\ansicpg1251\uc1\deff0\deflang1049\deflangfe1049 `)
	buf.Write(body)
	if !bytes.HasSuffix(bytes.TrimSpace(body), []byte("}")) {
		buf.WriteByte('}')
	}
	return buf.Bytes()
}

func renderRepeating(raw []byte, rows []map[string]string, extras map[string]string) ([]byte, error) {
	start := bytes.Index(raw, []byte("#D2#"))
	end := bytes.Index(raw, []byte("#2D#"))
	if start < 0 || end < 0 || end < start {
		return nil, fmt.Errorf("template missing #D2#/#2D# markers")
	}
	head := raw[:start]
	rowTpl := raw[start+len("#D2#") : end]
	tail := raw[end+len("#2D#"):]

	var body bytes.Buffer
	body.Grow(len(rowTpl)*len(rows) + len(head) + len(tail))
	body.Write(applyKeys(head, extras))
	for _, row := range rows {
		merged := mergeKeys(extras, row)
		body.Write(applyKeys(rowTpl, merged))
	}
	body.Write(applyKeys(tail, extras))
	return body.Bytes(), nil
}

func renderCards(raw []byte, cards []map[string]string, extras map[string]string) ([]byte, error) {
	if len(cards) == 0 {
		return applyKeys(raw, extras), nil
	}
	var body bytes.Buffer
	for i, card := range cards {
		if i > 0 {
			body.WriteString(`\pard \page `)
		}
		merged := mergeKeys(extras, card)
		body.Write(applyKeys(raw, merged))
	}
	return body.Bytes(), nil
}

func mergeKeys(a, b map[string]string) map[string]string {
	out := make(map[string]string, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

func applyKeys(tpl []byte, keys map[string]string) []byte {
	out := tpl
	// Longer keys first so prefixes don't collide.
	names := make([]string, 0, len(keys))
	for k := range keys {
		names = append(names, k)
	}
	sort.Slice(names, func(i, j int) bool { return len(names[i]) > len(names[j]) })
	for _, k := range names {
		ph := []byte("#" + k + "#")
		out = bytes.ReplaceAll(out, ph, encodeRTF(keys[k]))
	}
	// Drop unfilled placeholders (keep layout).
	out = leftoverKey.ReplaceAll(out, nil)
	return out
}

// encodeRTF converts UTF-8 text to CP1251 bytes suitable for ANSI RTF templates.
func encodeRTF(s string) []byte {
	if s == "" {
		return nil
	}
	ansi := irbis.ToAnsi(s)
	var buf bytes.Buffer
	buf.Grow(len(ansi) + 8)
	for _, b := range ansi {
		switch b {
		case '\\', '{', '}':
			buf.WriteByte('\\')
			buf.WriteByte(b)
		default:
			buf.WriteByte(b)
		}
	}
	// Ensure we didn't lose validity if ToAnsi returned empty for bad input.
	if buf.Len() == 0 && utf8.ValidString(s) && strings.TrimSpace(s) != "" {
		return []byte("?")
	}
	return buf.Bytes()
}
