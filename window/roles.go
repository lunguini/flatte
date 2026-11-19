package window

import (
	"github.com/lunguini/flatte/flatui/layout"
	ds "github.com/lunguini/flatte/theme"
)

// resolveStyle turns the design roles a renderer has no font or color of
// its own for into what it draws: captions, meta and page titles a size
// and weight from the type scale, strong text bold, and color roles (and
// the text roles' colors) an explicit Fg from the palette. Body, title and
// subtle text keep their role, so the renderers' own fonts apply.
func resolveStyle(s layout.TextStyle, p *ds.Palette, ts ds.TypeScale) layout.TextStyle {
	size := func(spec ds.TextSpec) {
		if s.Size == 0 {
			s.Size = spec.Size
		}
	}
	switch s.Role {
	case layout.TextCaption:
		size(ts.Caption)
		s.Bold = s.Bold || ts.Caption.Weight >= 600
		s.Role = layout.TextBody
		if s.Fg == nil && s.Color == layout.ColorDefault {
			s.Fg = p.TextMuted
		}
	case layout.TextMeta:
		size(ts.Meta)
		s.Role = layout.TextBody
		if s.Fg == nil && s.Color == layout.ColorDefault {
			s.Fg = p.TextSecondary
		}
	case layout.TextStrong:
		s.Bold = true
		s.Role = layout.TextBody
	case layout.TextPageTitle:
		size(ts.PageTitle)
		s.Role = layout.TextTitle
	}
	if s.Fg == nil && s.Color != layout.ColorDefault {
		s.Fg = layout.TextColorOf(p, s)
	}
	return s
}
