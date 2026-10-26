package main

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"

	"github.com/Monska85/PDF-Unseal/internal/i18n"
)

//go:embed assets/icon-72.png assets/icon-108.png assets/icon-144.png assets/icon-256.png assets/eye-*.png
var iconFiles embed.FS

// Icon sizes available for the drop zone, in pixels, ascending.
var iconSizes = []int{72, 108, 144, 256}

// Sizes available for the 16-pixel glyphs ("eye", "eye-off"), one per common DPI scale, ascending.
var glyphSizes = []int{16, 20, 24, 32, 40, 48}

// passwordChar is the bullet that common controls v6 uses for hidden passwords.
const passwordChar = 0x25CF

// Palette, close to the Windows 11 defaults.
var (
	colorBackground = walk.RGB(0xFF, 0xFF, 0xFF)
	colorAccent     = walk.RGB(0x00, 0x67, 0xC0)
	colorHoverFill  = walk.RGB(0xF0, 0xF6, 0xFC)
	colorBorder     = walk.RGB(0xA6, 0xA6, 0xA6)
	colorTitle      = walk.RGB(0x1A, 0x1A, 0x1A)
	colorMuted      = walk.RGB(0x5F, 0x5F, 0x5F)
	colorSuccess    = walk.RGB(0x0F, 0x7B, 0x0F)
)

// walkUI implements ui with native Win32 controls.
type walkUI struct {
	mw          *walk.MainWindow
	zone        *walk.CustomWidget
	status      *walk.Composite
	statusLabel *walk.Label

	titleFont    *walk.Font
	subtitleFont *walk.Font
	icons        map[int]*walk.Bitmap // drop zone icon per pixel size, created on demand
	glyphs       map[string]*walk.Bitmap
	hover        bool
	busy         bool // a PDF is being processed
	closePending bool // the user closed the window while busy
	aboutBitmap  *walk.Bitmap
	savedPath    string
}

func run() {
	u := &walkUI{icons: map[int]*walk.Bitmap{}, glyphs: map[string]*walk.Bitmap{}}
	var err error
	if u.titleFont, err = walk.NewFont("Segoe UI Semibold", 14, 0); err != nil {
		fatal(err)
	}
	if u.subtitleFont, err = walk.NewFont("Segoe UI", 10, 0); err != nil {
		fatal(err)
	}

	if err := (MainWindow{
		AssignTo:   &u.mw,
		Title:      appTitle,
		Icon:       appIcon(),
		Size:       Size{Width: 480, Height: 400},
		MinSize:    Size{Width: 360, Height: 320},
		Background: SolidColorBrush{Color: colorBackground},
		Font:       Font{Family: "Segoe UI", PointSize: 10},
		Layout:     VBox{Margins: Margins{Left: 24, Top: 16, Right: 24, Bottom: 16}, Spacing: 12},
		MenuItems: []MenuItem{
			Menu{
				Text: i18n.T("menu.file"),
				Items: []MenuItem{
					Action{Text: i18n.T("menu.open"), Shortcut: Shortcut{Modifiers: walk.ModControl, Key: walk.KeyO}, OnTriggered: u.open},
				},
			},
			Menu{
				Text: i18n.T("menu.help"),
				Items: []MenuItem{
					Action{Text: i18n.T("menu.about"), OnTriggered: u.showAbout},
				},
			},
		},
		OnDropFiles: func(files []string) {
			if len(files) > 0 {
				u.start(files[0])
			}
		},
		Children: []Widget{
			CustomWidget{
				AssignTo:            &u.zone,
				PaintPixels:         u.paintZone,
				PaintMode:           PaintBuffered,
				InvalidatesOnResize: true,
				ToolTipText:         i18n.T("zone.tooltip"),
				OnMouseMove: func(_, _ int, _ walk.MouseButton) {
					u.setHover(true)
				},
				OnMouseUp: func(x, y int, button walk.MouseButton) {
					if button == walk.LeftButton && contains(u.zone.ClientBoundsPixels(), x, y) {
						u.open()
					}
				},
			},
			Composite{
				AssignTo: &u.status,
				Visible:  false,
				Layout:   HBox{MarginsZero: true},
				Children: []Widget{
					Label{AssignTo: &u.statusLabel, TextColor: colorSuccess, EllipsisMode: EllipsisEnd},
					LinkLabel{
						Text:            "<a>" + i18n.T("status.showInFolder") + "</a>",
						OnLinkActivated: func(*walk.LinkLabelLink) { u.showInFolder() },
					},
				},
			},
		},
	}).Create(); err != nil {
		fatal(err)
	}

	u.zone.SetCursor(walk.CursorHand())
	// Closing during decryption would exit before Unseal removes its temporary file; close once it finishes.
	u.mw.Closing().Attach(func(canceled *bool, _ walk.CloseReason) {
		if u.busy {
			*canceled = true
			u.closePending = true
		}
	})
	centerOnMonitor(u.mw)
	u.mw.Run()
}

// start processes path in the background, so the window stays responsive.
// The ui methods that process calls switch back to the UI thread.
func (u *walkUI) start(path string) {
	if u.busy {
		return
	}
	u.busy = true
	u.status.SetVisible(false)
	u.zone.Invalidate()
	go func() {
		process(u, path)
		u.mw.Synchronize(func() {
			u.busy = false
			if u.closePending {
				u.mw.Close()
				return
			}
			u.zone.Invalidate()
		})
	}()
}

// onUI runs f on the UI thread and waits for it. Call it only from other goroutines.
func (u *walkUI) onUI(f func()) {
	done := make(chan struct{})
	u.mw.Synchronize(func() {
		defer close(done)
		f()
	})
	<-done
}

func (u *walkUI) open() {
	if u.busy {
		return
	}
	dlg := walk.FileDialog{Title: i18n.T("open.title"), Filter: i18n.T("filter.pdf") + "|*.pdf|" + i18n.T("filter.all") + "|*.*"}
	if ok, err := dlg.ShowOpen(u.mw); err != nil {
		u.showError(errorMessage(err))
	} else if ok {
		u.start(dlg.FilePath)
	}
}

// showError shows message from the UI thread.
func (u *walkUI) showError(message string) {
	walk.MsgBox(u.mw, appTitle, message, walk.MsgBoxIconError)
}

func (u *walkUI) ShowError(message string) {
	u.onUI(func() { u.showError(message) })
}

func (u *walkUI) ShowSaved(path string) {
	u.onUI(func() {
		u.savedPath = path
		u.statusLabel.SetText(i18n.T("status.saved", filepath.Base(path)))
		u.status.SetVisible(true)
	})
}

func (u *walkUI) ConfirmOverwrite(path string) bool {
	var answer int
	u.onUI(func() {
		answer = walk.MsgBox(u.mw, appTitle, i18n.T("save.overwrite", filepath.Base(path)), walk.MsgBoxYesNo|walk.MsgBoxIconWarning|walk.MsgBoxDefButton2)
	})
	return answer == win.IDYES
}

func (u *walkUI) AskSavePath(suggested string) (path string, ok bool, err error) {
	u.onUI(func() {
		dlg := walk.FileDialog{
			Title:    i18n.T("save.title"),
			Filter:   i18n.T("filter.pdf") + "|*.pdf",
			FilePath: suggested,
			// Without it Windows may open the last folder used instead of the PDF's folder.
			InitialDirPath: filepath.Dir(suggested),
			Flags:          win.OFN_OVERWRITEPROMPT,
		}
		ok, err = dlg.ShowSave(u.mw)
		path = dlg.FilePath
	})
	return path, ok, err
}

func (u *walkUI) AskPassword(fileName, message string) (password string, ok bool) {
	u.onUI(func() { password, ok = u.passwordDialog(fileName, message) })
	return password, ok
}

func (u *walkUI) passwordDialog(fileName, message string) (string, bool) {
	var dlg *walk.Dialog
	var edit *walk.LineEdit
	var okPB, cancelPB *walk.PushButton
	var eyePB *walk.ToolButton
	var password string
	eye, eyeOff := u.glyph("eye"), u.glyph("eye-off")
	shown := false
	toggle := func() {
		shown = !shown
		if shown {
			edit.SendMessage(win.EM_SETPASSWORDCHAR, 0, 0)
			eyePB.SetImage(eyeOff)
			eyePB.SetToolTipText(i18n.T("password.hide"))
		} else {
			edit.SendMessage(win.EM_SETPASSWORDCHAR, passwordChar, 0)
			eyePB.SetImage(eye)
			eyePB.SetToolTipText(i18n.T("password.show"))
		}
		edit.Invalidate()
		edit.SetFocus()
	}
	result, err := Dialog{
		AssignTo:      &dlg,
		Title:         fileName,
		Icon:          appIcon(),
		DefaultButton: &okPB,
		CancelButton:  &cancelPB,
		MinSize:       Size{Width: 360},
		Background:    SolidColorBrush{Color: colorBackground},
		Font:          Font{Family: "Segoe UI", PointSize: 10},
		Layout:        VBox{Margins: Margins{Left: 20, Top: 16, Right: 20, Bottom: 16}, Spacing: 10},
		Children: []Widget{
			Label{Text: message},
			Composite{
				Layout: HBox{MarginsZero: true, Spacing: 4},
				Children: []Widget{
					LineEdit{AssignTo: &edit, PasswordMode: true},
					ToolButton{AssignTo: &eyePB, Image: eye, ToolTipText: i18n.T("password.show"), OnClicked: toggle},
				},
			},
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					HSpacer{},
					PushButton{AssignTo: &okPB, Text: i18n.T("password.unlock"), OnClicked: func() { password = edit.Text(); dlg.Accept() }},
					PushButton{AssignTo: &cancelPB, Text: i18n.T("password.cancel"), OnClicked: func() { dlg.Cancel() }},
				},
			},
		},
	}.Run(u.mw)
	if err != nil {
		u.showError(errorMessage(err))
		return "", false
	}
	return password, result == walk.DlgCmdOK
}

// showInFolder opens Explorer with the last saved file selected.
func (u *walkUI) showInFolder() {
	cmd := exec.Command("explorer.exe")
	// Explorer needs the path quoted after the comma, which exec's argument quoting cannot produce.
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: fmt.Sprintf(`explorer.exe /select,"%s"`, u.savedPath)}
	if err := cmd.Start(); err != nil {
		u.showError(errorMessage(err))
		return
	}
	// Explorer hands the window to its running instance and exits; reap the process.
	go cmd.Wait()
}

// setHover updates the hover highlight. While hovered it polls the cursor,
// because the window receives no event when the mouse leaves the zone.
func (u *walkUI) setHover(hover bool) {
	if u.hover == hover {
		return
	}
	u.hover = hover
	u.zone.Invalidate()
	if hover {
		u.watchMouseLeave()
	}
}

func (u *walkUI) watchMouseLeave() {
	time.AfterFunc(100*time.Millisecond, func() {
		u.mw.Synchronize(func() {
			if !u.hover {
				return
			}
			var pt win.POINT
			win.GetCursorPos(&pt)
			win.ScreenToClient(u.zone.Handle(), &pt)
			if !contains(u.zone.ClientBoundsPixels(), int(pt.X), int(pt.Y)) {
				u.setHover(false)
				return
			}
			u.watchMouseLeave()
		})
	})
}

// paintZone draws the dashed drop zone with the icon and its two lines of text.
func (u *walkUI) paintZone(c *walk.Canvas, _ walk.Rectangle) error {
	dpi := u.zone.DPI()
	px := func(v int) int { return v * dpi / 96 }
	b := u.zone.ClientBoundsPixels()

	bg, err := walk.NewSolidColorBrush(colorBackground)
	if err != nil {
		return err
	}
	defer bg.Dispose()
	if err := c.FillRectanglePixels(bg, b); err != nil {
		return err
	}

	inset := px(2)
	r := walk.Rectangle{X: b.X + inset, Y: b.Y + inset, Width: b.Width - 2*inset, Height: b.Height - 2*inset}
	corner := walk.Size{Width: px(16), Height: px(16)}
	border := colorBorder
	if u.hover {
		border = colorAccent
		tint, err := walk.NewSolidColorBrush(colorHoverFill)
		if err != nil {
			return err
		}
		defer tint.Dispose()
		if err := c.FillRoundedRectanglePixels(tint, r, corner); err != nil {
			return err
		}
	}
	borderBrush, err := walk.NewSolidColorBrush(border)
	if err != nil {
		return err
	}
	defer borderBrush.Dispose()
	pen, err := walk.NewGeometricPen(walk.PenDash|walk.PenCapFlat|walk.PenJoinRound, px(2), borderBrush)
	if err != nil {
		return err
	}
	defer pen.Dispose()
	if err := c.DrawRoundedRectanglePixels(pen, r, corner); err != nil {
		return err
	}

	// Center icon, title and subtitle as one block.
	const format = walk.TextCenter | walk.TextSingleLine | walk.TextEndEllipsis
	iconSize := px(72)
	titleText, subtitleText := i18n.T("zone.title"), i18n.T("zone.subtitle")
	if u.busy {
		titleText, subtitleText = i18n.T("zone.busy"), ""
	}
	title, _, err := c.MeasureTextPixels(titleText, u.titleFont, r, format)
	if err != nil {
		return err
	}
	// walk reports an error when asked to measure or draw empty text.
	var subtitle walk.Rectangle
	gapIcon, gapText := px(16), 0
	if subtitleText != "" {
		if subtitle, _, err = c.MeasureTextPixels(subtitleText, u.subtitleFont, r, format); err != nil {
			return err
		}
		gapText = px(4)
	}
	y := r.Y + (r.Height-(iconSize+gapIcon+title.Height+gapText+subtitle.Height))/2

	icon, err := u.zoneIcon(iconSize)
	if err != nil {
		return err
	}
	if err := c.DrawImageStretchedPixels(icon, walk.Rectangle{X: r.X + (r.Width-iconSize)/2, Y: y, Width: iconSize, Height: iconSize}); err != nil {
		return err
	}
	y += iconSize + gapIcon
	if err := c.DrawTextPixels(titleText, u.titleFont, colorTitle, walk.Rectangle{X: r.X, Y: y, Width: r.Width, Height: title.Height}, format); err != nil {
		return err
	}
	if subtitleText == "" {
		return nil
	}
	y += title.Height + gapText
	return c.DrawTextPixels(subtitleText, u.subtitleFont, colorAccent, walk.Rectangle{X: r.X, Y: y, Width: r.Width, Height: subtitle.Height}, format)
}

// zoneIcon returns the smallest embedded icon at least size pixels wide, so it is only ever scaled down.
func (u *walkUI) zoneIcon(size int) (*walk.Bitmap, error) {
	i := sort.SearchInts(iconSizes, size)
	if i == len(iconSizes) {
		i--
	}
	s := iconSizes[i]
	if bmp, ok := u.icons[s]; ok {
		return bmp, nil
	}
	img, err := loadIcon(s)
	if err != nil {
		return nil, err
	}
	bmp, err := walk.NewBitmapFromImageForDPI(img, 96)
	if err != nil {
		return nil, err
	}
	u.icons[s] = bmp
	return bmp, nil
}

// glyph returns the named 16-pixel glyph scaled for the window's DPI.
func (u *walkUI) glyph(name string) walk.Image {
	want := 16 * u.mw.DPI() / 96
	i := sort.SearchInts(glyphSizes, want)
	if i == len(glyphSizes) {
		i--
	}
	size := glyphSizes[i]
	key := fmt.Sprintf("%s-%d", name, size)
	if bmp, ok := u.glyphs[key]; ok {
		return bmp
	}
	img, err := loadImage(key)
	if err != nil {
		return nil
	}
	// The bitmap DPI makes walk treat it as 16 logical pixels.
	bmp, err := walk.NewBitmapFromImageForDPI(img, size*96/16)
	if err != nil {
		return nil
	}
	u.glyphs[key] = bmp
	return bmp
}

func loadIcon(size int) (image.Image, error) {
	return loadImage(fmt.Sprintf("icon-%d", size))
}

func loadImage(name string) (image.Image, error) {
	data, err := iconFiles.ReadFile("assets/" + name + ".png")
	if err != nil {
		return nil, err
	}
	return png.Decode(bytes.NewReader(data))
}

var (
	appIconOnce sync.Once
	appIconImg  *walk.Icon
)

// appIcon returns the icon embedded in the exe by release builds, or one built from the PNG otherwise.
// It is created once and shared by all windows.
func appIcon() *walk.Icon {
	appIconOnce.Do(func() {
		if icon, err := walk.NewIconFromResource("APP"); err == nil {
			appIconImg = icon
			return
		}
		if img, err := loadIcon(256); err == nil {
			appIconImg, _ = walk.NewIconFromImageForDPI(img, 96)
		}
	})
	return appIconImg
}

// centerOnMonitor moves the window to the center of the work area of its monitor.
func centerOnMonitor(mw *walk.MainWindow) {
	var mi win.MONITORINFO
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	if !win.GetMonitorInfo(win.MonitorFromWindow(mw.Handle(), win.MONITOR_DEFAULTTONEAREST), &mi) {
		return
	}
	wa := mi.RcWork
	b := mw.BoundsPixels()
	b.X = int(wa.Left) + (int(wa.Right-wa.Left)-b.Width)/2
	b.Y = int(wa.Top) + (int(wa.Bottom-wa.Top)-b.Height)/2
	mw.SetBoundsPixels(b)
}

func contains(r walk.Rectangle, x, y int) bool {
	return x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height
}

func fatal(err error) {
	walk.MsgBox(nil, appTitle, err.Error(), walk.MsgBoxIconError)
	os.Exit(1)
}
