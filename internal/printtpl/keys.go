package printtpl

import (
	"fmt"
	"strings"
	"time"

	"github.com/ivm97/irbis-api/internal/domain"
)

// CardKeys maps one bibliographic record to #key# placeholders (shabl-style card).
func CardKeys(rec domain.Record) map[string]string {
	title := firstSub(rec, 200, "a")
	subtitle := firstSub(rec, 200, "e")
	author := formatAuthor(rec)
	year := firstSub(rec, 210, "d")
	place := firstSub(rec, 210, "a")
	publisher := firstSub(rec, 210, "c")
	bbk := firstOf(firstField(rec, 621), firstField(rec, 686), firstSub(rec, 675, "a"))
	cipher := firstOf(firstField(rec, 903), firstSub(rec, 903, "a"))
	invent := joinSubs(rec, 910, "b", ", ")
	vogr := firstOf(firstField(rec, 333), firstSub(rec, 333, "a"))
	predm := joinSubs(rec, 606, "a", ". ")
	ann := firstOf(firstField(rec, 331), firstSub(rec, 331, "a"))
	pages := firstSub(rec, 215, "a")
	isbn := firstOf(firstField(rec, 10), firstSub(rec, 10, "a"))

	biblio := buildBiblio(author, title, subtitle, place, publisher, year, pages, isbn)

	keys := map[string]string{
		"zagol":  author,
		"av":     authorMark(author, cipher),
		"zagl":   "",
		"biblio": biblio,
		"invent": invent,
		"bbk":    bbk,
		"klass":  klassLabel(bbk),
		"vogr":   vogr,
		"predm":  predm,
		"ann":    ann,
		"sod":    firstOf(firstField(rec, 327), firstSub(rec, 327, "a")),
		"specif": firstOf(firstField(rec, 300), firstSub(rec, 300, "a")),
		"prilog": firstSub(rec, 215, "e"),
		"recen":  "",
		"recenz": "",
		"DATE":   time.Now().Format("02.01.2006"),
		"zero1":  "",
		"zero2":  "",
		"zero3":  "",
		"zero4":  "",
		"zero5":  "",
		"zero6":  "",
		"god":    year,
		"gi":     cipher,
	}
	return keys
}

// InventoryRows expands a record into one row per 910 (экземпляр) for dopshab-style tables.
// If there is no 910, returns a single row from bibliographic fields.
func InventoryRows(rec domain.Record) []map[string]string {
	base := CardKeys(rec)
	copies := fieldsByTag(rec, 910)
	if len(copies) == 0 {
		row := copyMap(base)
		row["dz"] = ""
		row["invn"] = ""
		row["invk"] = ""
		row["sigl"] = ""
		row["cena"] = ""
		row["itog"] = ""
		row["vseg"] = ""
		row["prim"] = ""
		row["dv"] = ""
		row["zagl"] = firstSub(rec, 200, "a")
		return []map[string]string{row}
	}
	out := make([]map[string]string, 0, len(copies))
	for _, f := range copies {
		row := copyMap(base)
		row["dz"] = sub(f, "d")
		row["invn"] = sub(f, "b")
		row["invk"] = sub(f, "b")
		row["sigl"] = firstOf(sub(f, "h"), sub(f, "c"))
		row["cena"] = firstOf(sub(f, "e"), sub(f, "u"))
		row["itog"] = row["cena"]
		row["vseg"] = ""
		row["prim"] = firstOf(sub(f, "x"), sub(f, "a"))
		row["dv"] = sub(f, "a")
		row["zagl"] = firstSub(rec, 200, "a")
		row["invent"] = row["invn"]
		out = append(out, row)
	}
	return out
}

// DocExtras are document-level keys (header/footer) for inventory books.
func DocExtras(extra map[string]string) map[string]string {
	out := map[string]string{
		"god":   time.Now().Format("2006"),
		"na":    "",
		"np":    "",
		"oz":    "",
		"slstr": "",
		"str":   "",
		"itog":  "",
		"vseg":  "",
		"DATE":  time.Now().Format("02.01.2006"),
		"D1":    "",
		"1D":    "",
		"D3":    "",
		"3D":    "",
	}
	for k, v := range extra {
		if strings.TrimSpace(k) == "" {
			continue
		}
		out[k] = v
	}
	return out
}

func buildBiblio(author, title, subtitle, place, publisher, year, pages, isbn string) string {
	var b strings.Builder
	if author != "" {
		b.WriteString(author)
		b.WriteString(" ")
	}
	b.WriteString(title)
	if subtitle != "" {
		b.WriteString(" : ")
		b.WriteString(subtitle)
	}
	if author != "" && title != "" {
		b.WriteString(" / ")
		b.WriteString(author)
	}
	parts := make([]string, 0, 4)
	if place != "" {
		parts = append(parts, place)
	}
	if publisher != "" {
		parts = append(parts, publisher)
	}
	if year != "" {
		parts = append(parts, year)
	}
	if len(parts) > 0 {
		b.WriteString(". – ")
		b.WriteString(strings.Join(parts, " : "))
	}
	if pages != "" {
		b.WriteString(". – ")
		b.WriteString(pages)
	}
	if isbn != "" {
		b.WriteString(". – ISBN ")
		b.WriteString(isbn)
	}
	s := b.String()
	if s != "" && !strings.HasSuffix(s, ".") {
		s += "."
	}
	return s
}

func formatAuthor(rec domain.Record) string {
	a := firstSub(rec, 700, "a")
	b := firstSub(rec, 700, "b")
	g := firstSub(rec, 700, "g")
	if a == "" {
		// Collective / secondary
		a = firstSub(rec, 710, "a")
		b = firstSub(rec, 710, "b")
	}
	switch {
	case a != "" && b != "":
		return a + ", " + b
	case a != "" && g != "":
		return a + ", " + g
	case a != "":
		return a
	default:
		return ""
	}
}

func authorMark(author, cipher string) string {
	if cipher != "" {
		return cipher
	}
	author = strings.TrimSpace(author)
	if author == "" {
		return ""
	}
	// Simple shelf mark: first letter(s) + space — users can refine later.
	runes := []rune(author)
	if len(runes) > 8 {
		runes = runes[:8]
	}
	return string(runes)
}

func klassLabel(bbk string) string {
	bbk = strings.TrimSpace(bbk)
	if bbk == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToUpper(bbk), "ББК") || strings.HasPrefix(strings.ToUpper(bbk), "BBK") {
		return bbk
	}
	return "ББК " + bbk
}

func fieldsByTag(rec domain.Record, tag int) []domain.Field {
	out := make([]domain.Field, 0)
	for _, f := range rec.Fields {
		if f.Tag == tag {
			out = append(out, f)
		}
	}
	return out
}

func firstField(rec domain.Record, tag int) string {
	for _, f := range rec.Fields {
		if f.Tag != tag {
			continue
		}
		if v := strings.TrimSpace(f.Value); v != "" {
			return v
		}
		if len(f.Subfields) > 0 {
			parts := make([]string, 0, len(f.Subfields))
			for _, sf := range f.Subfields {
				if strings.TrimSpace(sf.Value) != "" {
					parts = append(parts, sf.Value)
				}
			}
			return strings.Join(parts, " ")
		}
	}
	return ""
}

func firstSub(rec domain.Record, tag int, code string) string {
	for _, f := range rec.Fields {
		if f.Tag != tag {
			continue
		}
		if v := sub(f, code); v != "" {
			return v
		}
	}
	return ""
}

func joinSubs(rec domain.Record, tag int, code, sep string) string {
	parts := make([]string, 0)
	for _, f := range rec.Fields {
		if f.Tag != tag {
			continue
		}
		if v := sub(f, code); v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, sep)
}

func sub(f domain.Field, code string) string {
	for _, sf := range f.Subfields {
		if sf.Code == code {
			return strings.TrimSpace(sf.Value)
		}
	}
	return ""
}

func firstOf(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func copyMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// Ensure unused import for fmt if we add errors later — keep helper for filename defaults.
func DefaultFilename(template string, n int) string {
	base := strings.TrimSpace(template)
	if base == "" {
		base = "print"
	}
	if n > 0 {
		return fmt.Sprintf("%s_%d", base, n)
	}
	return base
}
