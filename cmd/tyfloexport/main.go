package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"os"

	"github.com/markbates/pkger"

	"github.com/mikolysz/tyfloprzeglad"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: tyfloexport <source.json> <destination.md>")
		return
	}

	repo, err := tyfloprzeglad.NewRepo(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't open the source file: %s\n", err)
		return
	}

	out, err := os.Create(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't create the output file: %s", err)
		return
	}
	defer out.Close()

	f, err := pkger.Open("/templates/export.tmpl")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening template file: %s", err)
		return
	}
	defer f.Close()

	tmplContents, err := ioutil.ReadAll(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't read the template file in: %s", err)
		return
	}

	t := template.New("export")
	template.Must(t.Parse(string(tmplContents)))

	eps, err := repo.EpisodeList()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't retrieve the episode list")
		return
	}

	if err := t.Execute(out, eps); err != nil {
		fmt.Fprintf(os.Stderr, "Can't render template: %s", err)
		return
	}
}
