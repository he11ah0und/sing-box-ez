package giogui

import (
	_ "embed"
	"fmt"
	"image/color"
	"os"
	"time"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core"
	"sing-box-ez/internal/framework/localengine"
	"sing-box-ez/internal/framework/updater"
	"sing-box-ez/internal/framework/version"
	"sing-box-ez/internal/gui/gio/pages"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/font/opentype"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// GUI holds the new Gio-based adaptive UI.
type GUI struct {
	cfg *config.AppConfig

	// Theme
	th *material.Theme

	// Navigation state
	shell *Shell

	// Window reference
	win *app.Window

	// Core controller (business logic)
	ctrl *core.InteractiveController

	// Dialog reference for startup sequences
	dialog *Dialog
}

// New creates a new Gio GUI instance.
func New(cfg *config.AppConfig) *GUI {
	th := material.NewTheme()
	// Dark theme by default
	th.Palette.Bg = color.NRGBA{R: 18, G: 18, B: 18, A: 255}
	th.Palette.Fg = color.NRGBA{R: 255, G: 255, B: 255, A: 255}

	// Build a font collection with Go font as base + optional emoji support.
	collection := gofont.Collection()
	if emoji := tryLoadEmojiFont(); len(emoji) > 0 {
		collection = append(collection, emoji...)
	}
	th.Shaper = text.NewShaper(text.WithCollection(collection))

	g := &GUI{
		cfg: cfg,
		th:  th,
	}

	// Initialize core controller (encapsulates manager + logger + i18n)
	g.ctrl = core.NewInteractiveController(cfg)

	dialog := NewDialog()
	g.dialog = dialog

	// Wire startup callbacks (UI-specific dialogs will be shown by the GUI)
	g.ctrl.OnFirstRun = func() {
		coreInstalled := g.ctrl.CoreExists()

		var urlEditor widget.Editor
		urlEditor.SingleLine = true

		var downloadBtn widget.Clickable
		var addBtn widget.Clickable

		dialog.ShowCustom(localengine.T("first_run.title"), func(gtx layout.Context) layout.Dimensions {
			if downloadBtn.Clicked(gtx) {
				go func() {
					dialog.ShowLoading(localengine.T("progress.checking_version"))
					_, err := g.ctrl.DownloadCoreWithProgressWithLog(nil)
					dialog.HideLoading()
					if err != nil {
						return
					}
					// Re-show first-run dialog with updated state.
					g.ctrl.OnFirstRun()
				}()
			}
			if addBtn.Clicked(gtx) {
				url := urlEditor.Text()
				go func() {
					dialog.ShowLoading(localengine.T("progress.adding_config"))
					err := g.ctrl.AddFirstConfigWithLog("default", url)
					dialog.HideLoading()
					if err != nil {
						return
					}
					dialog.HideCustom()
				}()
			}

			children := []layout.FlexChild{
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Body1(th, localengine.T("first_run.welcome")).Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Body2(th, localengine.T("first_run.description")).Layout(gtx)
				}),
			}

			status := localengine.T("first_run.core.not_installed")
			if coreInstalled {
				status = localengine.T("first_run.core.installed")
				if ver, err := g.ctrl.GetInstalledCoreVersion(); err == nil && ver != "" {
					status += "\n" + fmt.Sprintf(localengine.T("first_run.core.version"), ver)
				}
			}
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Body2(th, status).Layout(gtx)
			}))

			if !coreInstalled {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return material.Button(th, &downloadBtn, localengine.T("first_run.btn.download_core")).Layout(gtx)
					})
				}))
			}

			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					ed := material.Editor(th, &urlEditor, "https://example.com/config.json")
					return ed.Layout(gtx)
				})
			}))

			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return material.Button(th, &addBtn, localengine.T("first_run.btn.add_config")).Layout(gtx)
				})
			}))

			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	}

	g.ctrl.OnSelfUpdateAvailable = func(info *updater.UpdateInfo) {
		currentText := version.Commit
		if currentText == "unknown" || currentText == "" {
			currentText = info.Current
		}

		var currentDateStr, latestDateStr, diffStr string
		if dt, err := version.BuildDateTime(); err == nil {
			currentDateStr = dt.Format("2006-01-02 15:04:05") + " (" + version.HumanDuration(dt) + ")"
		} else {
			currentDateStr = localengine.T("dialog.unknown")
		}
		if !info.LatestDate.IsZero() {
			lt := info.LatestDate.Local()
			latestDateStr = lt.Format("2006-01-02 15:04:05") + " (" + version.HumanDuration(info.LatestDate) + ")"
			if dt, err := version.BuildDateTime(); err == nil {
				diff := info.LatestDate.Sub(dt)
				if diff < 0 {
					diff = -diff
				}
				diffStr = fmt.Sprintf(localengine.T("dialog.self_update.behind"), humanDuration(diff))
			}
		} else {
			latestDateStr = localengine.T("dialog.unknown")
		}

		body := fmt.Sprintf("**%s** %s\n  %s\n\n**%s** %s\n  %s\n\n",
			localengine.T("dialog.self_update.current"), currentText, currentDateStr,
			localengine.T("dialog.self_update.latest"), info.Latest, latestDateStr)
		if diffStr != "" {
			body += diffStr + "\n\n"
		}
		body += "## " + localengine.T("dialog.self_update.changelog") + "\n\n" + info.LatestBody

		dialog.ShowConfirmMarkdown(localengine.T("dialog.self_update.title"), body, func() {
			dialog.ShowLoading(localengine.T("progress.downloading_update"))
			go func() {
				if err := g.ctrl.ApplySelfUpdateWithLog(info.AssetURL, nil); err != nil {
					dialog.HideLoading()
					dialog.Show(localengine.T("dialog.self_update.title"), "Update failed: "+err.Error())
					return
				}
				dialog.HideLoading()
				dialog.Show(localengine.T("dialog.self_update.title"), "Update complete. Please restart.")
			}()
		})
	}

	aboutPage := pages.NewAboutPage(th, g.ctrl, dialog)
	mainPage := pages.NewMainPage(th, g.ctrl, dialog)
	logPage := pages.NewLogPage(th, g.ctrl.Controller)

	settingsPage := pages.NewSettingsPage(th, g.ctrl, dialog)
	settingsPage.OnLanguageChange = func() {
		g.shell.RebuildNav()
	}

	primary := []pages.Page{mainPage, pages.NewConfigsPage(th, g.ctrl, dialog)}
	secondary := []pages.Page{
		pages.NewCorePage(th, g.ctrl, dialog),
		settingsPage,
		logPage,
	}
	if cfg.GetPluginsEnabled() {
		secondary = append(secondary, pages.NewPluginsPage(th, g.ctrl.Controller))
	}
	secondary = append(secondary, aboutPage)

	g.shell = NewShell(th, cfg, g.ctrl.Controller, primary, secondary)
	g.shell.dialog = dialog

	return g
}

// Run starts the Gio event loop.
func (g *GUI) Run() {
	go func() {
		w := new(app.Window)
		w.Option(
			app.Title("sing-box-ez"),
			app.Size(unit.Dp(800), unit.Dp(600)),
		)
		g.win = w

		var ops op.Ops
		for {
			switch e := w.Event().(type) {
			case app.DestroyEvent:
				if g.ctrl != nil {
					g.ctrl.Close()
				}
				os.Exit(0)
			case app.FrameEvent:
				gtx := app.NewContext(&ops, e)
				g.shell.Layout(gtx)
				e.Frame(gtx.Ops)
			}
		}
	}()

	// Show global initialization loading dialog immediately so it renders
	// while the startup sequence runs.
	g.dialog.ShowLoading(localengine.T("progress.initializing"))

	go func() {
		g.ctrl.RunStartupSequence()
		g.dialog.HideLoading()
	}()

	app.Main()
}

//go:embed assets/NotoEmoji.ttf
var notoEmojiFontData []byte

func tryLoadEmojiFont() []font.FontFace {
	face, err := opentype.Parse(notoEmojiFontData)
	if err != nil {
		return nil
	}
	return []font.FontFace{{Font: face.Font(), Face: face}}
}

func humanDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	if days < 30 {
		return fmt.Sprintf("%dd", days)
	}
	months := days / 30
	if months < 12 {
		return fmt.Sprintf("%dmo", months)
	}
	return fmt.Sprintf("%dy", months/12)
}
