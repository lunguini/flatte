// Package kit is Flatte's design-system components: pane headers, tabs,
// list rows, segmented choices, badges, sections, setting rows, empty
// states and popovers, built from layout nodes in the theme's roles (fills,
// text roles and colors) and token sizes (layout.RowSize, BarSize…). They
// hold no colors or lengths of their own, so a theme — its palette,
// density and roundness — restyles them all, in a window and in a
// terminal alike.
//
// Each component is a plain struct; Layout builds its tree, as flatui widgets do. Components are
// stateless: selection, focus and values come from app state, and clicks
// arrive as MouseEvents targeting the IDs each one documents. Every slot
// that takes nodes (Actions, Trailing, Children) takes any layout.Node, so
// an app can put its own pieces where the recipe leaves room.

package kit
