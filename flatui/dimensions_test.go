package flatui

import "testing"

func TestWidgetDimensionsClampToProductionCeiling(t *testing.T) {
	extreme := int(^uint(0) >> 1)

	var list List
	list.SetHeight(extreme)
	if list.height != maxWidgetDimension {
		t.Fatalf("list height = %d, want %d", list.height, maxWidgetDimension)
	}

	var tree Tree
	tree.SetHeight(extreme)
	if tree.height != maxWidgetDimension {
		t.Fatalf("tree height = %d, want %d", tree.height, maxWidgetDimension)
	}

	var viewport Viewport
	viewport.SetSize(extreme, extreme)
	if viewport.width != maxWidgetDimension || viewport.height != maxWidgetDimension {
		t.Fatalf("viewport size = %dx%d, want %dx%d", viewport.width, viewport.height, maxWidgetDimension, maxWidgetDimension)
	}

	var textarea Textarea
	textarea.SetSize(extreme, extreme)
	if textarea.width != maxWidgetDimension || textarea.height != maxWidgetDimension {
		t.Fatalf("textarea size = %dx%d, want %dx%d", textarea.width, textarea.height, maxWidgetDimension, maxWidgetDimension)
	}
}
