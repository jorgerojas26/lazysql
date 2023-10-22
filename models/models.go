package models

import "github.com/rivo/tview"

type Connection struct {
	Name     string
	Provider string
	User     string
	Password string
	Host     string
	Port     string
}

type StateChange struct {
	Value interface{}
	Key   string
}

type ConnectionPages struct {
	*tview.Flex
	*tview.Pages
}
