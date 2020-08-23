package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	port := getenv("PORT", "4000")
	user := getenv("TYFLOPRZEGLAD_USER", "user")
	pass := getenv("TYFLOPRZEGLAD_PASS", "pass")
	filename := getenv("TYFLOPRZEGLAD_FILENAME", "tyfloprzeglad.json")

	repo, err := NewRepo(filename)
	if err != nil {
		log.Fatalf("Error when opening data file: %s", err)
	}

	c := NewController(repo, user, pass)

	log.Println("Running on port ", port)
	http.ListenAndServe(":"+port, c)
}

func getenv(varName string, defaultVal string) string {
	v, ok := os.LookupEnv(varName)
	if !ok {
		return defaultVal
	}
	return v
}
