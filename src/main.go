package main

import (
	"log"
	"os"
	"rest"
)

func main() {
	server := rest.NewRestServerWithLogger(log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile))
	server.Run()
}
