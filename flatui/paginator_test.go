package flatui

import "testing"

func TestPaginatorPagesAndClamps(t *testing.T) {
	var p Paginator
	p.SetPageSize(10)
	p.SetTotal(95)
	if p.Pages() != 10 {
		t.Fatalf("Pages() = %d, want 10", p.Pages())
	}
	p.SelectPage(99)
	if p.Page() != 9 {
		t.Fatalf("Page() after SelectPage(99) = %d, want 9", p.Page())
	}
	first, last := p.Range()
	if first != 90 || last != 95 {
		t.Fatalf("Range() = %d,%d want 90,95", first, last)
	}
	p.SetTotal(15)
	if p.Page() != 1 {
		t.Fatalf("Page() after shrink = %d, want 1", p.Page())
	}
}

func TestPaginatorZeroPageSize(t *testing.T) {
	var p Paginator
	p.SetTotal(10)
	if p.Pages() != 0 {
		t.Fatalf("Pages() = %d, want 0", p.Pages())
	}
	first, last := p.Range()
	if first != 0 || last != 0 {
		t.Fatalf("Range() = %d,%d want 0,0", first, last)
	}
}
