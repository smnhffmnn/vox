package windowctx

// Context holds information about the currently focused window.
type Context struct {
	AppName      string
	AppID        string
	WindowTitle  string
	SelectedText string
}
