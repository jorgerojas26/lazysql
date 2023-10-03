package main

import (
	"lazysql/app"
	"lazysql/pages"
)

func main() {
	if err := app.App.SetRoot(pages.AllPages, true).Run(); err != nil {
		panic(err)
	}
}
