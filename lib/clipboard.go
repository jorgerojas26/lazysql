package lib

import "github.com/atotto/clipboard"

type Clipboard struct{}

func NewClipboard() *Clipboard {
	return &Clipboard{}
}

func (c *Clipboard) Write(text string) error {
	return clipboard.WriteAll(text)
}

func (c *Clipboard) Read() (string, error) {
	return clipboard.ReadAll()
}
