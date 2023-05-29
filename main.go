package main

import (
	"lazysql/pages"
)

func main() {

	if err := pages.App.SetRoot(pages.AllPages, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}
