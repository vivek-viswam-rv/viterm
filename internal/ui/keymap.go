package ui

import (
	tea "charm.land/bubbletea/v2"
)

// prefixAction names a command reachable through the prefix key.
type prefixAction int

const (
	actionNone prefixAction = iota
	actionNewTab
	actionCloseTab
	actionNextTab
	actionPrevTab
	actionSelectTab // uses key digit
	actionSplitRight
	actionSplitDown
	actionClosePane
	actionFocusLeft
	actionFocusDown
	actionFocusUp
	actionFocusRight
	actionMaximize
	actionCopyMode
	actionClear
	actionDiffOpen
	actionDiffClose
	actionNewSession
	actionDetach
	actionDelete
	actionNextSession
	actionPrevSession
	actionOpenURL
	actionHelp
	actionQuit
	actionSendPrefix
)

// lookupPrefixAction maps a key pressed after the prefix to its action and,
// for tab selection, the tab index.
func lookupPrefixAction(k tea.KeyPressMsg, prefix string) (prefixAction, int) {
	if k.Keystroke() == prefix {
		return actionSendPrefix, 0
	}
	switch k.Keystroke() {
	case "c":
		return actionNewTab, 0
	case "x":
		return actionCloseTab, 0
	case "n":
		return actionNextTab, 0
	case "p":
		return actionPrevTab, 0
	case "v":
		return actionSplitRight, 0
	case "s":
		return actionSplitDown, 0
	case "shift+x", "X":
		return actionClosePane, 0
	case "h":
		return actionFocusLeft, 0
	case "j":
		return actionFocusDown, 0
	case "k":
		return actionFocusUp, 0
	case "l":
		return actionFocusRight, 0
	case "z":
		return actionMaximize, 0
	case "[":
		return actionCopyMode, 0
	case "shift+l", "L":
		return actionClear, 0
	case "g":
		return actionDiffOpen, 0
	case "shift+g", "G":
		return actionDiffClose, 0
	case "shift+n", "N":
		return actionNewSession, 0
	case "d":
		return actionDetach, 0
	case "shift+d", "D":
		return actionDelete, 0
	case "tab":
		return actionNextSession, 0
	case "shift+tab":
		return actionPrevSession, 0
	case "o":
		return actionOpenURL, 0
	case "?", "shift+/":
		return actionHelp, 0
	case "shift+q", "Q":
		return actionQuit, 0
	}
	if k.Code >= '1' && k.Code <= '9' && k.Mod == 0 {
		return actionSelectTab, int(k.Code - '1')
	}
	return actionNone, 0
}
