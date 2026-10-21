package window

import (
	"strings"

	"github.com/ebitengine/purego/objc"

	"github.com/lunguini/flatte"
)

const (
	nsModalResponseOK     = 1
	nsAlertFirstButtonRet = 1000
	nsDragOperationCopy   = 1
	nsPasteboardTypeURL   = "public.file-url"
)

// nativeDialogs is the window's dialogBackend: the system open and save
// panels and alerts, run modally.
type nativeDialogs struct{}

func (nativeDialogs) run(d flatte.Dialog) flatte.DialogResult {
	var r flatte.DialogResult
	withPool(func() {
		switch {
		case d.Open != nil:
			r = runOpen(*d.Open)
		case d.Save != nil:
			r = runSave(*d.Save)
		case d.Alert != nil:
			r = runAlert(*d.Alert)
		}
	})
	return r
}

// nsStrings is an autoreleased NSArray of strs.
func nsStrings(strs []string) objc.ID {
	a := class("NSMutableArray").Send(sel("array"))
	for _, s := range strs {
		a.Send(sel("addObject:"), nsString(s))
	}
	return a
}

// fileTypes is Types without leading dots.
func fileTypes(types []string) []string {
	out := make([]string, len(types))
	for i, t := range types {
		out[i] = strings.TrimPrefix(t, ".")
	}
	return out
}

// setUpPanel applies what open and save panels share.
func setUpPanel(panel objc.ID, title, dir string, types []string) {
	if title != "" {
		panel.Send(sel("setTitle:"), nsString(title))
		panel.Send(sel("setMessage:"), nsString(title))
	}
	if dir != "" {
		panel.Send(sel("setDirectoryURL:"), class("NSURL").Send(sel("fileURLWithPath:isDirectory:"), nsString(expandTilde(dir)), true))
	}
	if len(types) > 0 {
		panel.Send(sel("setAllowedFileTypes:"), nsStrings(fileTypes(types)))
	}
}

// expandTilde expands a leading "~" as the shell does.
func expandTilde(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home := plainString(class("NSString").Send(sel("stringWithString:"), nsString("~")).Send(sel("stringByExpandingTildeInPath")))
		return home + p[1:]
	}
	return p
}

func runOpen(d flatte.OpenDialog) flatte.DialogResult {
	panel := class("NSOpenPanel").Send(sel("openPanel"))
	setUpPanel(panel, d.Title, d.Directory, d.Types)
	panel.Send(sel("setCanChooseFiles:"), !d.Folders)
	panel.Send(sel("setCanChooseDirectories:"), d.Folders)
	panel.Send(sel("setAllowsMultipleSelection:"), d.Multiple)
	if objc.Send[int](panel, sel("runModal")) != nsModalResponseOK {
		return flatte.DialogResult{}
	}
	return flatte.DialogResult{Paths: urlPaths(panel.Send(sel("URLs")))}
}

func runSave(d flatte.SaveDialog) flatte.DialogResult {
	panel := class("NSSavePanel").Send(sel("savePanel"))
	setUpPanel(panel, d.Title, d.Directory, d.Types)
	if d.Name != "" {
		panel.Send(sel("setNameFieldStringValue:"), nsString(d.Name))
	}
	if objc.Send[int](panel, sel("runModal")) != nsModalResponseOK {
		return flatte.DialogResult{}
	}
	return flatte.DialogResult{Paths: []string{plainString(panel.Send(sel("URL")).Send(sel("path")))}}
}

func runAlert(d flatte.AlertDialog) flatte.DialogResult {
	alert := alloc("NSAlert").Send(sel("init"))
	defer alert.Send(sel("release"))
	alert.Send(sel("setMessageText:"), nsString(d.Title))
	alert.Send(sel("setInformativeText:"), nsString(d.Message))
	buttons := d.Buttons
	if len(buttons) == 0 {
		buttons = []string{"OK"}
	}
	for _, b := range buttons {
		alert.Send(sel("addButtonWithTitle:"), nsString(b))
	}
	i := objc.Send[int](alert, sel("runModal")) - nsAlertFirstButtonRet
	if i < 0 || i >= len(buttons) {
		i = -1
	}
	return flatte.DialogResult{Button: i}
}

// urlPaths is the file paths of an NSArray of NSURLs.
func urlPaths(urls objc.ID) []string {
	if urls == 0 {
		return nil
	}
	n := objc.Send[int](urls, sel("count"))
	paths := make([]string, 0, n)
	for i := range n {
		if p := urls.Send(sel("objectAtIndex:"), i).Send(sel("path")); p != 0 {
			paths = append(paths, plainString(p))
		}
	}
	return paths
}

// dropped reads a drag's pasteboard: its file paths, else its text.
func dropped(pb objc.ID) (paths []string, text string) {
	opts := class("NSDictionary").Send(sel("dictionaryWithObject:forKey:"),
		class("NSNumber").Send(sel("numberWithBool:"), true), nsString("NSPasteboardURLReadingFileURLsOnlyKey"))
	classes := class("NSArray").Send(sel("arrayWithObject:"), objc.ID(class("NSURL")))
	paths = urlPaths(pb.Send(sel("readObjectsForClasses:options:"), classes, opts))
	if len(paths) == 0 {
		if s := pb.Send(sel("stringForType:"), nsString(nsPasteboardTypeString)); s != 0 {
			text = plainString(s)
		}
	}
	return paths, text
}

// dragTypes are the pasteboard types the view accepts drops of.
func dragTypes() objc.ID { return nsStrings([]string{nsPasteboardTypeURL, nsPasteboardTypeString}) }
