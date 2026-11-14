package flatte

import (
	"container/list"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"reflect"
	"sort"
	"strings"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/lunguini/flatte/flatui/layout"
)

const (
	kittyCellPixelWidth  = 8
	kittyCellPixelHeight = 16
	// kittyDensity is how many times a nominal cell's pixels a picture is
	// prepared at, at most.
	kittyDensity         = 4
	kittyChunkSize       = 4096
	kittyImageLimit      = 16
	kittyImageBytesLimit = 32 << 20
	kittyImageMaxBytes   = 8 << 20
	kittyImageMaxPixels  = 4 << 20
	kittyImageMemoLimit  = 64
)

type kittyImageOutput interface {
	Position() (x, y int)
	MoveTo(x, y int)
	WriteString(string) (int, error)
}

type kittyAssetMemoKey struct {
	source        string
	width, height int
	fit           layout.ImageFit
}

type kittyPlacementKey struct{ index int }

type kittyImageAsset struct {
	hash    [32]byte
	id      uint32
	data    []byte
	element *list.Element
}

type kittyAssetMemo struct {
	hash   [32]byte
	source image.Image
}

type kittyPlacedImage struct {
	imageID, placementID uint32
	rect                 layout.Rect
	asset                [32]byte
	source               string
	fit                  layout.ImageFit
}

type kittyImagePlan struct {
	key     kittyPlacementKey
	place   layout.TerminalImagePlacement
	source  string
	stable  bool
	width   int
	height  int
	asset   [32]byte
	data    []byte
	imageID uint32
}

type kittyImageState struct {
	enabled bool

	nextImageID     uint32
	nextPlacementID uint32
	assets          map[[32]byte]*kittyImageAsset
	assetsLRU       list.List
	assetBytes      int
	memo            map[kittyAssetMemoKey]kittyAssetMemo
	placements      map[kittyPlacementKey]kittyPlacedImage
}

func newKittyImageState(enabled bool) *kittyImageState {
	var seeds [8]byte
	if enabled {
		_, _ = rand.Read(seeds[:])
	}
	imageID := binary.LittleEndian.Uint32(seeds[:4]) & 0x7fffffff
	placementID := binary.LittleEndian.Uint32(seeds[4:]) & 0x7fffffff
	if imageID == 0 {
		imageID = 1
	}
	if placementID == 0 {
		placementID = 1
	}
	return &kittyImageState{
		enabled:         enabled,
		nextImageID:     imageID,
		nextPlacementID: placementID,
		assets:          make(map[[32]byte]*kittyImageAsset),
		memo:            make(map[kittyAssetMemoKey]kittyAssetMemo),
		placements:      make(map[kittyPlacementKey]kittyPlacedImage),
	}
}

func kittyImagesEnabled(env []string) bool {
	values := make(map[string]string, len(env))
	for _, item := range env {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			values[key] = value
		}
	}
	if strings.EqualFold(strings.TrimSpace(values["FLATTE_IMAGES"]), "cells") {
		return false
	}
	term := strings.ToLower(strings.TrimSpace(values["TERM"]))
	if strings.TrimSpace(values["TMUX"]) != "" || strings.HasPrefix(term, "screen") {
		return false
	}
	program := strings.ToLower(strings.TrimSpace(values["TERM_PROGRAM"]))
	return strings.TrimSpace(values["KITTY_WINDOW_ID"]) != "" || term == "xterm-kitty" ||
		program == "ghostty" || program == "wezterm" || program == "warpterminal"
}

func kittyPlacementsForRuntime(main layout.Composition, windows *windowHost, menus *menuHost, dialogs *termDialogs, width, height int) []layout.TerminalImagePlacement {
	bounds := layout.Rect{W: width, H: height}
	if width <= 0 || height <= 0 {
		return nil
	}
	var windowRects []layout.Rect
	if windows != nil {
		windowRects = make([]layout.Rect, len(windows.open))
		for i, window := range windows.open {
			if window == nil {
				continue
			}
			windowRects[i] = (layout.Rect{
				X: window.x, Y: window.y, W: window.w + 2, H: window.h + 2,
			}).Intersect(bounds)
		}
	}
	var menuRects []layout.Rect
	if menus != nil {
		for _, level := range menus.levels {
			menuRects = append(menuRects, (layout.Rect{
				X: level.x, Y: level.y, W: level.w, H: level.h,
			}).Intersect(bounds))
		}
	}
	var dialogRect layout.Rect
	var dialog *termDialog
	if dialogs != nil {
		dialog = dialogs.cur
		if dialog != nil {
			dialogRect = (layout.Rect{X: dialog.x, Y: dialog.y, W: dialog.w, H: dialog.h}).Intersect(bounds)
		}
	}

	var out []layout.TerminalImagePlacement
	for _, placement := range main.TerminalImages {
		if kittyRectVisible(placement.Rect, bounds) && !kittyImageCovered(placement.Rect, windowRects, menuRects, []layout.Rect{dialogRect}) {
			out = append(out, placement)
		}
	}
	if windows != nil {
		for i, window := range windows.open {
			if window == nil {
				continue
			}
			for _, placement := range window.comp.TerminalImages {
				placed, ok := translateKittyPlacement(placement, window.x+1, window.y+1, bounds)
				if !ok || kittyImageCovered(placed.Rect, windowRects[i+1:], menuRects, []layout.Rect{dialogRect}) {
					continue
				}
				out = append(out, placed)
			}
		}
	}
	if dialog != nil {
		for _, placement := range dialog.comp.TerminalImages {
			if placed, ok := translateKittyPlacement(placement, dialog.x, dialog.y, bounds); ok {
				out = append(out, placed)
			}
		}
	}
	return out
}

// clearUnderKittyImages blanks the cell fallback under each picture,
// keeping the cells' backgrounds, so a picture's transparent pixels show
// the background rather than the half-block cells drawn for terminals
// without images.
func clearUnderKittyImages(screen uv.ScreenBuffer, images []layout.TerminalImagePlacement) {
	for _, image := range images {
		r := image.Rect.Intersect(layout.Rect{W: screen.Width(), H: screen.Height()})
		for y := r.Y; y < r.Y+r.H; y++ {
			for x := r.X; x < r.X+r.W; x++ {
				blank := uv.EmptyCell
				if c := screen.CellAt(x, y); c != nil {
					blank.Style.Bg = c.Style.Bg
				}
				screen.SetCell(x, y, &blank)
			}
		}
	}
}

func translateKittyPlacement(placement layout.TerminalImagePlacement, x, y int, bounds layout.Rect) (layout.TerminalImagePlacement, bool) {
	placement.Rect.X += x
	placement.Rect.Y += y
	return placement, kittyRectVisible(placement.Rect, bounds)
}

func kittyRectVisible(rect, bounds layout.Rect) bool {
	return rect.W > 0 && rect.H > 0 && rect.Intersect(bounds) == rect
}

func kittyImageCovered(rect layout.Rect, covers ...[]layout.Rect) bool {
	for _, group := range covers {
		for _, cover := range group {
			if cover.W <= 0 || cover.H <= 0 {
				continue
			}
			intersection := rect.Intersect(cover)
			if intersection.W > 0 && intersection.H > 0 {
				return true
			}
		}
	}
	return false
}

func kittyTransmit(id uint32, pngBytes []byte) string {
	if id == 0 || len(pngBytes) == 0 {
		return ""
	}
	encoded := base64.StdEncoding.EncodeToString(pngBytes)
	var out strings.Builder
	for start := 0; start < len(encoded); start += kittyChunkSize {
		end := min(start+kittyChunkSize, len(encoded))
		more := 0
		if end < len(encoded) {
			more = 1
		}
		fmt.Fprintf(&out, "\x1b_Ga=t,f=100,t=d,i=%d,m=%d,q=1;%s\x1b\\", id, more, encoded[start:end])
	}
	return out.String()
}

func kittyPlace(imageID, placementID uint32, rect layout.Rect) string {
	if imageID == 0 || placementID == 0 || rect.W <= 0 || rect.H <= 0 {
		return ""
	}
	return fmt.Sprintf("\x1b_Ga=p,i=%d,p=%d,c=%d,r=%d,C=1,z=1,q=1\x1b\\", imageID, placementID, rect.W, rect.H)
}

func kittyDeletePlacement(imageID, placementID uint32) string {
	if imageID == 0 || placementID == 0 {
		return ""
	}
	return fmt.Sprintf("\x1b_Ga=d,d=i,i=%d,p=%d,q=1\x1b\\", imageID, placementID)
}

func kittyDeleteImage(imageID uint32) string {
	if imageID == 0 {
		return ""
	}
	return fmt.Sprintf("\x1b_Ga=d,d=I,i=%d,q=1\x1b\\", imageID)
}

func (r *kittyImageState) update(placements []layout.TerminalImagePlacement, out kittyImageOutput) {
	if !r.enabled {
		return
	}
	plans := make([]kittyImagePlan, 0, len(placements))
	preparedByKey := make(map[kittyAssetMemoKey]kittyAssetMemo)
	preparedByHash := make(map[[32]byte][]byte)
	preparedBytes := 0
	for i, placement := range placements {
		if isNilKittyImage(placement.Source) || placement.Rect.W <= 0 || placement.Rect.H <= 0 {
			continue
		}
		width, height, ok := kittyPixelSize(placement.Rect, placement.Source.Bounds())
		if !ok {
			continue
		}
		source, stable := kittySourceKey(placement.Source)
		plan := kittyImagePlan{
			key:    kittyPlacementKey{index: i},
			place:  placement,
			source: source,
			stable: stable,
			width:  width,
			height: height,
		}
		old, hasOld := r.placements[plan.key]
		if stable && hasOld && old.source == source && old.rect == placement.Rect && old.fit == placement.Fit {
			if _, cached := r.assets[old.asset]; cached {
				plan.asset = old.asset
				plans = append(plans, plan)
				continue
			}
		}
		memoKey := kittyAssetMemoKey{source: source, width: width, height: height, fit: placement.Fit}
		if stable {
			if memo, ok := r.memo[memoKey]; ok {
				if asset := r.assets[memo.hash]; asset != nil {
					plan.asset = memo.hash
					plans = append(plans, plan)
					continue
				}
				delete(r.memo, memoKey)
			}
			if memo, ok := preparedByKey[memoKey]; ok {
				plan.asset, plan.data = memo.hash, preparedByHash[memo.hash]
				plans = append(plans, plan)
				continue
			}
		}
		data, err := prepareKittyPNG(placement.Source, width, height, placement.Fit)
		if err != nil || len(data) == 0 || len(data) > kittyImageMaxBytes {
			continue
		}
		plan.asset = sha256.Sum256(data)
		if r.assets[plan.asset] == nil {
			if shared, ok := preparedByHash[plan.asset]; ok {
				plan.data = shared
			} else {
				if preparedBytes+len(data) > kittyImageBytesLimit {
					continue
				}
				plan.data = data
				preparedByHash[plan.asset] = data
				preparedBytes += len(data)
			}
		}
		if stable {
			preparedByKey[memoKey] = kittyAssetMemo{hash: plan.asset, source: placement.Source}
		}
		plans = append(plans, plan)
	}

	oldPlacements := r.placements
	survivors := make(map[kittyPlacementKey]kittyPlacedImage, len(plans))
	planByKey := make(map[kittyPlacementKey]*kittyImagePlan, len(plans))
	for i := range plans {
		planByKey[plans[i].key] = &plans[i]
	}
	for key, old := range oldPlacements {
		plan := planByKey[key]
		if plan == nil || old.rect != plan.place.Rect || old.asset != plan.asset {
			_, _ = out.WriteString(kittyDeletePlacement(old.imageID, old.placementID))
			continue
		}
		survivors[key] = old
	}
	r.placements = survivors

	wantedAssets := make(map[[32]byte]bool, len(plans))
	for i := range plans {
		wantedAssets[plans[i].asset] = true
	}
	for i := range plans {
		plan := &plans[i]
		asset, ok := r.asset(plan.asset)
		if !ok {
			if len(plan.data) == 0 {
				continue
			}
			asset, ok = r.addAsset(plan.asset, plan.data, wantedAssets, out)
			if !ok {
				continue
			}
		}
		if plan.stable {
			r.memo[kittyAssetMemoKey{
				source: plan.source,
				width:  plan.width,
				height: plan.height,
				fit:    plan.place.Fit,
			}] = kittyAssetMemo{hash: plan.asset, source: plan.place.Source}
			if len(r.memo) > kittyImageMemoLimit {
				for key := range r.memo {
					delete(r.memo, key)
					break
				}
			}
		}
		plan.imageID = asset.id
	}

	startX, startY := out.Position()
	movedCursor := false
	nextPlacements := make(map[kittyPlacementKey]kittyPlacedImage, len(plans))
	for i := range plans {
		plan := &plans[i]
		if plan.imageID == 0 {
			continue
		}
		old, hadOld := oldPlacements[plan.key]
		placementID := uint32(0)
		if hadOld {
			placementID = old.placementID
		} else {
			placementID = r.allocatePlacementID()
		}
		placed := kittyPlacedImage{
			imageID: plan.imageID, placementID: placementID,
			rect: plan.place.Rect, asset: plan.asset, source: plan.source, fit: plan.place.Fit,
		}
		if survivor, unchanged := survivors[plan.key]; unchanged && survivor.imageID == placed.imageID && survivor.rect == placed.rect {
			nextPlacements[plan.key] = survivor
			continue
		}
		out.MoveTo(plan.place.Rect.X, plan.place.Rect.Y)
		movedCursor = true
		_, _ = out.WriteString(kittyPlace(placed.imageID, placed.placementID, placed.rect))
		nextPlacements[plan.key] = placed
	}
	if movedCursor {
		out.MoveTo(startX, startY)
	}
	r.placements = nextPlacements
}

func (r *kittyImageState) clear(out kittyImageOutput) {
	if !r.enabled {
		return
	}
	ids := make([]int, 0, len(r.assets))
	for _, asset := range r.assets {
		ids = append(ids, int(asset.id))
	}
	sort.Ints(ids)
	for _, id := range ids {
		_, _ = out.WriteString(kittyDeleteImage(uint32(id)))
	}
	r.assets = make(map[[32]byte]*kittyImageAsset)
	r.assetsLRU.Init()
	r.assetBytes = 0
	r.memo = make(map[kittyAssetMemoKey]kittyAssetMemo)
	r.placements = make(map[kittyPlacementKey]kittyPlacedImage)
}

func (r *kittyImageState) asset(hash [32]byte) (*kittyImageAsset, bool) {
	asset := r.assets[hash]
	if asset == nil {
		return nil, false
	}
	r.assetsLRU.MoveToFront(asset.element)
	return asset, true
}

func (r *kittyImageState) addAsset(hash [32]byte, data []byte, wanted map[[32]byte]bool, out kittyImageOutput) (*kittyImageAsset, bool) {
	if asset, ok := r.asset(hash); ok {
		return asset, true
	}
	if len(data) > kittyImageBytesLimit {
		return nil, false
	}
	for len(r.assets) >= kittyImageLimit || r.assetBytes+len(data) > kittyImageBytesLimit {
		var victim *kittyImageAsset
		for item := r.assetsLRU.Back(); item != nil; item = item.Prev() {
			candidate := item.Value.(*kittyImageAsset)
			if wanted[candidate.hash] {
				continue
			}
			if r.assetIsPlaced(candidate.id) {
				continue
			}
			victim = candidate
			break
		}
		if victim == nil {
			return nil, false
		}
		r.removeAsset(victim, out)
	}
	asset := &kittyImageAsset{hash: hash, id: r.allocateImageID(), data: append([]byte(nil), data...)}
	asset.element = r.assetsLRU.PushFront(asset)
	r.assets[hash] = asset
	r.assetBytes += len(asset.data)
	_, _ = out.WriteString(kittyTransmit(asset.id, asset.data))
	return asset, true
}

func (r *kittyImageState) assetIsPlaced(id uint32) bool {
	for _, placement := range r.placements {
		if placement.imageID == id {
			return true
		}
	}
	return false
}

func (r *kittyImageState) removeAsset(asset *kittyImageAsset, out kittyImageOutput) {
	hash := asset.hash
	delete(r.assets, hash)
	r.assetsLRU.Remove(asset.element)
	r.assetBytes -= len(asset.data)
	for key, memo := range r.memo {
		if memo.hash == hash {
			delete(r.memo, key)
		}
	}
	_, _ = out.WriteString(kittyDeleteImage(asset.id))
}

func (r *kittyImageState) allocateImageID() uint32 {
	return allocateKittyID(&r.nextImageID)
}

func (r *kittyImageState) allocatePlacementID() uint32 {
	return allocateKittyID(&r.nextPlacementID)
}

func allocateKittyID(next *uint32) uint32 {
	id := *next
	if id == 0 {
		id = 1
	}
	*next = id + 1
	if *next == 0 {
		*next = 1
	}
	return id
}

// kittyPixelSize is the pixel size to prepare rect's picture at. A nominal
// cell is kittyCellPixelWidth×kittyCellPixelHeight, which a high-density
// screen draws two to three times larger, so pictures are prepared at up
// to kittyDensity times that, but never above the source's own resolution
// (the terminal scales the placement to its cells either way).
func kittyPixelSize(rect layout.Rect, source image.Rectangle) (width, height int, ok bool) {
	maxInt := int(^uint(0) >> 1)
	if rect.W <= 0 || rect.H <= 0 || rect.W > maxInt/(kittyCellPixelWidth*kittyDensity) || rect.H > maxInt/(kittyCellPixelHeight*kittyDensity) {
		return 0, 0, false
	}
	width, height = rect.W*kittyCellPixelWidth, rect.H*kittyCellPixelHeight
	for scale := kittyDensity; scale > 1; scale-- {
		if width*scale <= max(source.Dx(), width) || height*scale <= max(source.Dy(), height) {
			width, height = width*scale, height*scale
			break
		}
	}
	if width > kittyImageMaxPixels/height {
		return 0, 0, false
	}
	return width, height, true
}

func kittySourceKey(src image.Image) (string, bool) {
	v := reflect.ValueOf(src)
	if !v.IsValid() {
		return "", false
	}
	if v.Kind() == reflect.Pointer {
		return fmt.Sprintf("%T:%x", src, v.Pointer()), true
	}
	if v.Comparable() {
		return fmt.Sprintf("%T:%#v", src, src), true
	}
	return "", false
}

func isNilKittyImage(src image.Image) bool {
	if src == nil {
		return true
	}
	v := reflect.ValueOf(src)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

func prepareKittyPNG(src image.Image, width, height int, fit layout.ImageFit) ([]byte, error) {
	if isNilKittyImage(src) || width <= 0 || height <= 0 || width > kittyImageMaxPixels/height {
		return nil, fmt.Errorf("flatte: invalid kitty image dimensions %dx%d", width, height)
	}
	bounds := src.Bounds()
	sourceWidth, sourceHeight := bounds.Dx(), bounds.Dy()
	if sourceWidth <= 0 || sourceHeight <= 0 {
		return nil, fmt.Errorf("flatte: empty kitty image source")
	}
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	switch fit {
	case layout.ImageStretch:
		resizeKitty(dst, src, float64(bounds.Min.X), float64(bounds.Min.Y), float64(sourceWidth), float64(sourceHeight), image.Rect(0, 0, width, height))
	case layout.ImageCover:
		scale := math.Max(float64(width)/float64(sourceWidth), float64(height)/float64(sourceHeight))
		cropWidth, cropHeight := float64(width)/scale, float64(height)/scale
		startX := float64(bounds.Min.X) + (float64(sourceWidth)-cropWidth)/2
		startY := float64(bounds.Min.Y) + (float64(sourceHeight)-cropHeight)/2
		resizeKitty(dst, src, startX, startY, cropWidth, cropHeight, image.Rect(0, 0, width, height))
	default:
		scale := math.Min(float64(width)/float64(sourceWidth), float64(height)/float64(sourceHeight))
		drawWidth := max(1, min(width, int(math.Round(float64(sourceWidth)*scale))))
		drawHeight := max(1, min(height, int(math.Round(float64(sourceHeight)*scale))))
		offsetX, offsetY := (width-drawWidth)/2, (height-drawHeight)/2
		sourceDrawRect := image.Rect(offsetX, offsetY, offsetX+drawWidth, offsetY+drawHeight)
		resizeKitty(dst, src, float64(bounds.Min.X), float64(bounds.Min.Y), float64(sourceWidth), float64(sourceHeight), sourceDrawRect)
	}
	var out strings.Builder
	if err := png.Encode(&out, dst); err != nil {
		return nil, err
	}
	if out.Len() > kittyImageMaxBytes {
		return nil, fmt.Errorf("flatte: kitty image PNG exceeds %d bytes", kittyImageMaxBytes)
	}
	return []byte(out.String()), nil
}

func resizeKitty(dst *image.RGBA, src image.Image, sourceX, sourceY, sourceW, sourceH float64, target image.Rectangle) {
	if target.Empty() || sourceW <= 0 || sourceH <= 0 {
		return
	}
	for y := target.Min.Y; y < target.Max.Y; y++ {
		sy := sourceY + (float64(y-target.Min.Y)+0.5)*sourceH/float64(target.Dy()) - 0.5
		for x := target.Min.X; x < target.Max.X; x++ {
			sx := sourceX + (float64(x-target.Min.X)+0.5)*sourceW/float64(target.Dx()) - 0.5
			dst.SetRGBA(x, y, sampleKittyBilinear(src, sx, sy))
		}
	}
}

func sampleKittyBilinear(src image.Image, x, y float64) color.RGBA {
	bounds := src.Bounds()
	x = min(max(x, float64(bounds.Min.X)), float64(bounds.Max.X-1))
	y = min(max(y, float64(bounds.Min.Y)), float64(bounds.Max.Y-1))
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	x1, y1 := min(x0+1, bounds.Max.X-1), min(y0+1, bounds.Max.Y-1)
	rx, ry := x-float64(x0), y-float64(y0)
	channels := [4]float64{}
	for _, sample := range []struct {
		point  image.Point
		weight float64
	}{
		{image.Pt(x0, y0), (1 - rx) * (1 - ry)},
		{image.Pt(x1, y0), rx * (1 - ry)},
		{image.Pt(x0, y1), (1 - rx) * ry},
		{image.Pt(x1, y1), rx * ry},
	} {
		r, g, b, a := src.At(sample.point.X, sample.point.Y).RGBA()
		channels[0] += float64(r) * sample.weight
		channels[1] += float64(g) * sample.weight
		channels[2] += float64(b) * sample.weight
		channels[3] += float64(a) * sample.weight
	}
	return color.RGBA{
		R: uint8(math.Round(channels[0] / 257)),
		G: uint8(math.Round(channels[1] / 257)),
		B: uint8(math.Round(channels[2] / 257)),
		A: uint8(math.Round(channels[3] / 257)),
	}
}
