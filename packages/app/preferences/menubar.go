package preferences

// MenuBar holds the items of the menu that opens from Ravenpass's menu bar icon.
type MenuBar struct {
	Open string
	Lock string
	Quit string
}

// MenuBar reports the menu bar menu's wording for the recorded language.
func (s *Store) MenuBar() MenuBar {
	catalog := s.catalog()
	return MenuBar{
		Open: catalog.Text("system.menu.open"),
		Lock: catalog.Text("system.menu.lock"),
		Quit: catalog.Text("system.menu.quit"),
	}
}
