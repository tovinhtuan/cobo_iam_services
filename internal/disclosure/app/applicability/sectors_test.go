package applicability

import "testing"

func TestNormalizeBusinessSectors_Single(t *testing.T) {
	out, err := NormalizeBusinessSectors([]string{" service "})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0] != BusinessSectorService {
		t.Fatalf("got %#v", out)
	}
}

func TestNormalizeBusinessSectors_MultipleCanonicalOrder(t *testing.T) {
	out, err := NormalizeBusinessSectors([]string{"manufacturing", "commercial", "service"})
	if err != nil {
		t.Fatal(err)
	}
	want := []BusinessSector{BusinessSectorCommercial, BusinessSectorService, BusinessSectorManufacturing}
	if len(out) != len(want) {
		t.Fatalf("len=%d want %d", len(out), len(want))
	}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("out[%d]=%s want %s", i, out[i], want[i])
		}
	}
}

func TestNormalizeBusinessSectors_Duplicates(t *testing.T) {
	out, err := NormalizeBusinessSectors([]string{"commercial", "commercial", "service"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("got %#v", out)
	}
}

func TestNormalizeBusinessSectors_Invalid(t *testing.T) {
	_, err := NormalizeBusinessSectors([]string{"trade"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNormalizeBusinessSectors_Empty(t *testing.T) {
	out, err := NormalizeBusinessSectors([]string{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("got %#v", out)
	}
}
