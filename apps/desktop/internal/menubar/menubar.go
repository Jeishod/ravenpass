// Package menubar puts Ravenpass's icon and menu in the macOS menu bar.
package menubar

import (
	"github.com/dortanes/ravenpass/packages/app/preferences"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Actions are what the icon and the menu items run; Unlocked decides whether the menu offers Lock.
type Actions struct {
	Toggle   func()
	Open     func()
	Lock     func()
	Quit     func()
	Unlocked func() bool
}

// Item is the menu bar icon's menu.
type Item struct {
	open *application.MenuItem
	lock *application.MenuItem
	quit *application.MenuItem
}

// New adds the icon: a click runs Toggle and a right click opens the menu.
func New(app *application.App, icon []byte, labels preferences.MenuBar, actions Actions) *Item {
	menu := application.NewMenu()
	item := &Item{
		open: menu.Add(labels.Open).OnClick(func(*application.Context) { actions.Open() }),
		lock: menu.Add(labels.Lock).OnClick(func(*application.Context) { actions.Lock() }),
	}
	menu.AddSeparator()
	item.quit = menu.Add(labels.Quit).OnClick(func(*application.Context) { actions.Quit() })

	tray := app.SystemTray.New().SetTemplateIcon(icon).SetMenu(menu)
	tray.OnClick(actions.Toggle)
	tray.OnRightClick(func() {
		item.lock.SetHidden(!actions.Unlocked())
		tray.OpenMenu()
	})
	return item
}

// Relabel shows the menu in another language.
func (i *Item) Relabel(labels preferences.MenuBar) {
	i.open.SetLabel(labels.Open)
	i.lock.SetLabel(labels.Lock)
	i.quit.SetLabel(labels.Quit)
}
