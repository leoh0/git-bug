package termui

import (
	"github.com/MichaelMure/git-bug/cache"
	"github.com/MichaelMure/git-bug/input"
	"github.com/MichaelMure/git-bug/repository"
	"github.com/jroimartin/gocui"
	"github.com/pkg/errors"
)

var errTerminateMainloop = errors.New("terminate gocui mainloop")

type termUI struct {
	g      *gocui.Gui
	gError chan error
	cache  cache.RepoCacher

	activeWindow window

	bugTable   *bugTable
	showBug    *showBug
	msgPopup   *msgPopup
	inputPopup *inputPopup
}

func (tui *termUI) activateWindow(window window) error {
	if err := tui.activeWindow.disable(tui.g); err != nil {
		return err
	}

	tui.activeWindow = window

	return nil
}

var ui *termUI

type window interface {
	keybindings(g *gocui.Gui) error
	layout(g *gocui.Gui) error
	disable(g *gocui.Gui) error
}

// Run will launch the termUI in the terminal
func Run(repo repository.Repo) error {
	c := cache.NewRepoCache(repo)

	ui = &termUI{
		gError:     make(chan error, 1),
		cache:      c,
		bugTable:   newBugTable(c),
		showBug:    newShowBug(c),
		msgPopup:   newMsgPopup(),
		inputPopup: newInputPopup(),
	}

	ui.activeWindow = ui.bugTable

	initGui(nil)

	err := <-ui.gError

	if err != nil && err != gocui.ErrQuit {
		return err
	}

	return nil
}

func initGui(action func(ui *termUI) error) {
	g, err := gocui.NewGui(gocui.OutputNormal)

	if err != nil {
		ui.gError <- err
		return
	}

	ui.g = g

	ui.g.SetManagerFunc(layout)

	err = keybindings(ui.g)

	if err != nil {
		ui.g.Close()
		ui.g = nil
		ui.gError <- err
		return
	}

	if action != nil {
		err = action(ui)
		if err != nil {
			ui.g.Close()
			ui.g = nil
			ui.gError <- err
			return
		}
	}

	err = g.MainLoop()

	if err != nil && err != errTerminateMainloop {
		if ui.g != nil {
			ui.g.Close()
		}
		ui.gError <- err
	}

	return
}

func layout(g *gocui.Gui) error {
	g.Cursor = false

	if err := ui.activeWindow.layout(g); err != nil {
		return err
	}

	if err := ui.msgPopup.layout(g); err != nil {
		return err
	}

	if err := ui.inputPopup.layout(g); err != nil {
		return err
	}

	return nil
}

func keybindings(g *gocui.Gui) error {
	// Quit
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		return err
	}

	if err := ui.bugTable.keybindings(g); err != nil {
		return err
	}

	if err := ui.showBug.keybindings(g); err != nil {
		return err
	}

	if err := ui.msgPopup.keybindings(g); err != nil {
		return err
	}

	if err := ui.inputPopup.keybindings(g); err != nil {
		return err
	}

	return nil
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func newBugWithEditor(repo cache.RepoCacher) error {
	// This is somewhat hacky.
	// As there is no way to pause gocui, run the editor and restart gocui,
	// we have to stop it entirely and start a new one later.
	//
	// - an error channel is used to route the returned error of this new
	// 		instance into the original launch function
	// - a custom error (errTerminateMainloop) is used to terminate the original
	//		instance's mainLoop. This error is then filtered.

	ui.g.Close()
	ui.g = nil

	title, message, err := input.BugCreateEditorInput(ui.cache.Repository(), "", "")

	if err != nil && err != input.ErrEmptyTitle {
		return err
	}

	var b cache.BugCacher
	if err == input.ErrEmptyTitle {
		ui.msgPopup.Activate(msgPopupErrorTitle, "Empty title, aborting.")
	} else {
		b, err = repo.NewBug(title, message)
		if err != nil {
			return err
		}
	}

	initGui(func(ui *termUI) error {
		ui.showBug.SetBug(b)
		return ui.activateWindow(ui.showBug)
	})

	return errTerminateMainloop
}

func addCommentWithEditor(bug cache.BugCacher) error {
	// This is somewhat hacky.
	// As there is no way to pause gocui, run the editor and restart gocui,
	// we have to stop it entirely and start a new one later.
	//
	// - an error channel is used to route the returned error of this new
	// 		instance into the original launch function
	// - a custom error (errTerminateMainloop) is used to terminate the original
	//		instance's mainLoop. This error is then filtered.

	ui.g.Close()
	ui.g = nil

	message, err := input.BugCommentEditorInput(ui.cache.Repository())

	if err != nil && err != input.ErrEmptyMessage {
		return err
	}

	if err == input.ErrEmptyMessage {
		ui.msgPopup.Activate(msgPopupErrorTitle, "Empty message, aborting.")
	} else {
		err := bug.AddComment(message)
		if err != nil {
			return err
		}
	}

	initGui(nil)

	return errTerminateMainloop
}

func setTitleWithEditor(bug cache.BugCacher) error {
	// This is somewhat hacky.
	// As there is no way to pause gocui, run the editor and restart gocui,
	// we have to stop it entirely and start a new one later.
	//
	// - an error channel is used to route the returned error of this new
	// 		instance into the original launch function
	// - a custom error (errTerminateMainloop) is used to terminate the original
	//		instance's mainLoop. This error is then filtered.

	ui.g.Close()
	ui.g = nil

	title, err := input.BugTitleEditorInput(ui.cache.Repository(), bug.Snapshot().Title)

	if err != nil && err != input.ErrEmptyTitle {
		return err
	}

	if err == input.ErrEmptyTitle {
		ui.msgPopup.Activate(msgPopupErrorTitle, "Empty title, aborting.")
	} else {
		err := bug.SetTitle(title)
		if err != nil {
			return err
		}
	}

	initGui(nil)

	return errTerminateMainloop
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a > b {
		return b
	}
	return a
}
