package main

import (
	"embed"
	"fmt"
	"strings"
	"syscall"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"

	"github.com/Monska85/PDF-Unseal/internal/i18n"
)

// notices holds THIRD_PARTY_LICENSES.txt in release builds, and only the placeholder otherwise.
//
//go:embed notices
var notices embed.FS

func thirdPartyNotices() string {
	for _, name := range []string{"notices/THIRD_PARTY_LICENSES.txt", "notices/placeholder.txt"} {
		if data, err := notices.ReadFile(name); err == nil {
			// The text box needs Windows line endings.
			return strings.ReplaceAll(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n", "\r\n")
		}
	}
	return ""
}

// noticesURL points to the notices published with this version, or with the latest release for development builds.
func noticesURL() string {
	if version == "dev" {
		return repoURL + "/releases/latest"
	}
	return fmt.Sprintf("%s/releases/download/v%s/THIRD_PARTY_LICENSES.txt", repoURL, version)
}

func (u *walkUI) showAbout() {
	var dlg *walk.Dialog
	var okPB *walk.PushButton
	if _, err := (Dialog{
		AssignTo:      &dlg,
		Title:         i18n.T("about.title"),
		Icon:          appIcon(),
		DefaultButton: &okPB,
		CancelButton:  &okPB,
		MinSize:       Size{Width: 380},
		Background:    SolidColorBrush{Color: colorBackground},
		Font:          Font{Family: "Segoe UI", PointSize: 10},
		Layout:        VBox{Margins: Margins{Left: 24, Top: 20, Right: 24, Bottom: 16}, Spacing: 14},
		Children: []Widget{
			Composite{
				Layout: HBox{MarginsZero: true, Spacing: 16},
				Children: []Widget{
					ImageView{Image: u.aboutIcon(), Mode: ImageViewModeIdeal},
					Composite{
						Layout: VBox{MarginsZero: true, Spacing: 2},
						Children: []Widget{
							VSpacer{},
							Label{Text: appTitle, Font: Font{Family: "Segoe UI Semibold", PointSize: 14}, TextColor: colorTitle},
							Label{Text: i18n.T("about.version", version), TextColor: colorMuted},
							VSpacer{},
						},
					},
					HSpacer{},
				},
			},
			Label{Text: i18n.T("about.madeBy", author)},
			LinkLabel{
				Text:            fmt.Sprintf(`<a id="repo">%s</a>`, strings.TrimPrefix(repoURL, "https://")),
				OnLinkActivated: func(*walk.LinkLabelLink) { u.openURL(repoURL) },
			},
			LinkLabel{
				Text: fmt.Sprintf(`<a id="license">%s</a>  ·  <a id="notices">%s</a>`, i18n.T("about.license"), i18n.T("about.thirdParty")),
				OnLinkActivated: func(link *walk.LinkLabelLink) {
					if link.Id() == "license" {
						u.openURL(repoURL + "/blob/main/LICENSE")
					} else {
						u.showNotices(dlg)
					}
				},
			},
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					HSpacer{},
					PushButton{AssignTo: &okPB, Text: i18n.T("about.ok"), OnClicked: func() { dlg.Accept() }},
				},
			},
		},
	}).Run(u.mw); err != nil {
		u.showError(errorMessage(err))
	}
}

func (u *walkUI) showNotices(owner walk.Form) {
	var dlg *walk.Dialog
	var closePB *walk.PushButton
	if _, err := (Dialog{
		AssignTo:      &dlg,
		Title:         i18n.T("licenses.title"),
		Icon:          appIcon(),
		DefaultButton: &closePB,
		CancelButton:  &closePB,
		Size:          Size{Width: 640, Height: 520},
		MinSize:       Size{Width: 400, Height: 300},
		Background:    SolidColorBrush{Color: colorBackground},
		Font:          Font{Family: "Segoe UI", PointSize: 10},
		Layout:        VBox{Margins: Margins{Left: 16, Top: 16, Right: 16, Bottom: 12}, Spacing: 10},
		Children: []Widget{
			TextEdit{Text: thirdPartyNotices(), ReadOnly: true, VScroll: true, Font: Font{Family: "Consolas", PointSize: 9}},
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					LinkLabel{
						Text:            "<a>" + i18n.T("licenses.github") + "</a>",
						OnLinkActivated: func(*walk.LinkLabelLink) { u.openURL(noticesURL()) },
					},
					HSpacer{},
					PushButton{AssignTo: &closePB, Text: i18n.T("licenses.close"), OnClicked: func() { dlg.Accept() }},
				},
			},
		},
	}).Run(owner); err != nil {
		u.showError(errorMessage(err))
	}
}

// aboutIcon returns the app icon at 64 logical pixels for the About dialog, created once.
func (u *walkUI) aboutIcon() walk.Image {
	if u.aboutBitmap != nil {
		return u.aboutBitmap
	}
	img, err := loadIcon(256)
	if err != nil {
		return nil
	}
	// The bitmap DPI makes walk treat the 256-pixel image as 64 logical pixels.
	bmp, err := walk.NewBitmapFromImageForDPI(img, 256*96/64)
	if err != nil {
		return nil
	}
	u.aboutBitmap = bmp
	return bmp
}

// openURL opens url in the default browser.
func (u *walkUI) openURL(url string) {
	verb, _ := syscall.UTF16PtrFromString("open")
	target, _ := syscall.UTF16PtrFromString(url)
	if !win.ShellExecute(u.mw.Handle(), verb, target, nil, nil, win.SW_SHOWNORMAL) {
		u.showError(i18n.T("error.unexpected", url))
	}
}
