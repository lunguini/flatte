package layout

import "image"

// ImageFit is how an Image's picture fills its box in a GUI.
type ImageFit int

const (
	// ImageContain scales the picture to fit inside the box, keeping its
	// aspect ratio, centered.
	ImageContain ImageFit = iota
	// ImageCover scales it to cover the box, keeping its aspect ratio,
	// centered and cropped.
	ImageCover
	// ImageStretch scales it to the box exactly.
	ImageStretch
)

// Image is a picture. A GUI draws Source in its content box as Fit says,
// one image pixel to a point on Auto axes; a terminal shows Text in it,
// subtle and wrapped to its width (and sizes to Text on Auto axes), so give
// pictures a short description. A GUI reads Source when it first draws it:
// to change the picture, give a new image rather than changing its pixels.
// A Round Box rounds the picture's corners as it would a box's.
type Image struct {
	NodeBase
	Source image.Image
	Text   string
	Fit    ImageFit
}

func (i Image) Size() (Size, Size) { return i.label().Size() }

func (i Image) Render(r Rect) string {
	l := i.label()
	l.Wrap = true
	return l.Render(r)
}

func (i Image) label() Label {
	return Label{NodeBase: i.NodeBase, Text: i.Text, Style: TextStyle{Role: TextSubtle}}
}

// pixels is Source's size in pixels (0×0 without one).
func (i Image) pixels() (int, int) {
	if i.Source == nil {
		return 0, 0
	}
	b := i.Source.Bounds()
	return b.Dx(), b.Dy()
}
