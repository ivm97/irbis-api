package irbisgw

import (
	"testing"

	"github.com/ivm97/GoIrbis/irbis"
	"github.com/ivm97/irbis-api/internal/domain"
)

func TestToDomainAndFromWrite_RoundTrip(t *testing.T) {
	in := domain.RecordWrite{
		MFN:     12,
		Status:  0,
		Version: 3,
		Fields: []domain.Field{
			{Tag: 200, Subfields: []domain.SubField{{Code: "a", Value: "Title"}, {Code: "e", Value: "sub"}}},
			{Tag: 700, Subfields: []domain.SubField{{Code: "a", Value: "Author"}}},
			{Tag: 920, Value: "PAZK"},
		},
	}
	rec := fromWrite("IBIS", in)
	if rec.Mfn != 12 || len(rec.Fields) != 3 {
		t.Fatalf("%+v", rec)
	}
	out := toDomain("IBIS", rec)
	if out.Database != "IBIS" || out.MFN != 12 || len(out.Fields) != 3 {
		t.Fatalf("%+v", out)
	}
	if out.Fields[0].Tag != 200 || len(out.Fields[0].Subfields) != 2 || out.Fields[0].Subfields[0].Value != "Title" {
		t.Fatalf("field0=%+v", out.Fields[0])
	}
	if out.Fields[2].Tag != 920 || out.Fields[2].Value != "PAZK" {
		t.Fatalf("field2=%+v", out.Fields[2])
	}
}

func TestSubfieldCode_Safe(t *testing.T) {
	if subfieldCode("") != 'a' {
		t.Fatal("empty")
	}
	if subfieldCode("b") != 'b' {
		t.Fatal("b")
	}
	// multi-byte first rune should not panic
	_ = subfieldCode("я")
}

func TestToDomain_EmptySubfields(t *testing.T) {
	r := irbis.NewMarcRecord()
	r.Mfn = 1
	r.Add(200, "plain")
	out := toDomain("IBIS", r)
	if len(out.Fields) != 1 || out.Fields[0].Value != "plain" {
		t.Fatalf("%+v", out)
	}
}
