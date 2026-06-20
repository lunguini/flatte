package flatui

import "fmt"

// Paginator is app-owned page/range state. It owns no key policy; apps choose
// whether to bind arrows, h/l, PgUp/PgDn, or buttons.
type Paginator struct {
	total    int
	pageSize int
	page     int
}

// SetTotal sets the total number of items, clamping the selected page.
func (p *Paginator) SetTotal(n int) {
	p.total = max(n, 0)
	p.clamp()
}

// SetPageSize sets the number of items per page, clamping the selected page.
func (p *Paginator) SetPageSize(n int) {
	p.pageSize = max(n, 0)
	p.clamp()
}

// Total is the total number of items.
func (p Paginator) Total() int { return p.total }

// PageSize is the number of items per page.
func (p Paginator) PageSize() int { return p.pageSize }

// Page is the selected zero-based page index, or 0 when there are no pages.
func (p Paginator) Page() int { return p.page }

// Pages is the number of pages needed to display Total items at PageSize.
func (p Paginator) Pages() int {
	if p.pageSize <= 0 || p.total <= 0 {
		return 0
	}
	return (p.total + p.pageSize - 1) / p.pageSize
}

// NextPage moves to the next page, clamped at the last page.
func (p *Paginator) NextPage() { p.SelectPage(p.page + 1) }

// PrevPage moves to the previous page, clamped at the first page.
func (p *Paginator) PrevPage() { p.SelectPage(p.page - 1) }

// SelectPage moves to page, clamped to the available page range.
func (p *Paginator) SelectPage(page int) {
	p.page = page
	p.clamp()
}

// Range returns the half-open item index range [first, last) for the selected
// page. It returns 0, 0 when no page can be formed.
func (p Paginator) Range() (first, last int) {
	if p.pageSize <= 0 || p.total <= 0 {
		return 0, 0
	}
	first = p.page * p.pageSize
	last = min(first+p.pageSize, p.total)
	return first, last
}

// View renders a compact page indicator for the selected page.
func (p Paginator) View() string {
	pages := p.Pages()
	if pages <= 1 {
		return "page 1/1"
	}
	return fmt.Sprintf("page %d/%d", p.page+1, pages)
}

func (p *Paginator) clamp() {
	pages := p.Pages()
	if pages == 0 {
		p.page = 0
		return
	}
	p.page = min(max(p.page, 0), pages-1)
}
