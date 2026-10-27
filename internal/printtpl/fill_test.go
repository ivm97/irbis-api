package printtpl

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ivm97/irbis-api/internal/domain"
)

func TestCardKeysAndInventoryRows(t *testing.T) {
	rec := domain.Record{
		MFN: 1,
		Fields: []domain.Field{
			{Tag: 200, Subfields: []domain.SubField{{Code: "a", Value: "Истинное волшебство"}}},
			{Tag: 700, Subfields: []domain.SubField{{Code: "a", Value: "Способина"}, {Code: "b", Value: "Н."}}},
			{Tag: 210, Subfields: []domain.SubField{{Code: "d", Value: "2024"}, {Code: "a", Value: "Москва"}, {Code: "c", Value: "МИФ"}}},
			{Tag: 621, Value: "84(2Рос=Рус)6"},
			{Tag: 910, Subfields: []domain.SubField{
				{Code: "b", Value: "848506"},
				{Code: "d", Value: "01.02.2024"},
				{Code: "h", Value: "цмб"},
				{Code: "e", Value: "450"},
			}},
			{Tag: 910, Subfields: []domain.SubField{
				{Code: "b", Value: "852106"},
				{Code: "h", Value: "бМая"},
			}},
		},
	}
	card := CardKeys(rec)
	if card["zagl"] != "" && card["biblio"] == "" {
		t.Fatal("expected biblio")
	}
	if card["invent"] != "848506, 852106" {
		t.Fatalf("invent=%q", card["invent"])
	}
	if card["zagol"] != "Способина, Н." {
		t.Fatalf("zagol=%q", card["zagol"])
	}
	rows := InventoryRows(rec)
	if len(rows) != 2 {
		t.Fatalf("rows=%d", len(rows))
	}
	if rows[0]["invn"] != "848506" || rows[1]["invn"] != "852106" {
		t.Fatalf("invn %#v %#v", rows[0]["invn"], rows[1]["invn"])
	}
	if rows[0]["zagl"] != "Истинное волшебство" {
		t.Fatalf("row zagl=%q", rows[0]["zagl"])
	}
}

func TestRenderRepeating(t *testing.T) {
	dir := t.TempDir()
	tpl := []byte(`HEAD #god# #D2#ROW:#invn#/#zagl# #2D#TAIL`)
	if err := os.WriteFile(filepath.Join(dir, "dopshab.txt"), tpl, 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewStore(dir)
	out, ext, err := s.Render("dopshab", []map[string]string{
		{"invn": "1", "zagl": "A"},
		{"invn": "2", "zagl": "B"},
	}, DocExtras(map[string]string{"god": "2026"}))
	if err != nil {
		t.Fatal(err)
	}
	if ext != ".rtf" {
		t.Fatalf("ext=%s", ext)
	}
	if !bytes.Contains(out, []byte("ROW:1/A")) || !bytes.Contains(out, []byte("ROW:2/B")) {
		t.Fatalf("body=%s", out)
	}
	if !bytes.Contains(out, []byte("2026")) {
		t.Fatalf("missing god: %s", out)
	}
	if !bytes.HasPrefix(bytes.TrimSpace(out), []byte(`{\rtf`)) {
		t.Fatalf("expected rtf wrapper: %s", out[:min(40, len(out))])
	}
}

func TestRenderCards(t *testing.T) {
	dir := t.TempDir()
	tpl := []byte(`\sect }CARD #zagol# / #biblio#`)
	if err := os.WriteFile(filepath.Join(dir, "shabl.txt"), tpl, 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewStore(dir)
	out, _, err := s.Render("shabl", []map[string]string{
		{"zagol": "A", "biblio": "desc"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte("#zagol#")) {
		t.Fatal("placeholder left")
	}
	if !bytes.Contains(out, []byte("CARD A / desc")) {
		t.Fatalf("%s", out)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
