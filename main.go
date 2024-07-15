package web

import (
	"context"
	"fmt"
	"log"
	"os"
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: web build\n")
	fmt.Fprintf(os.Stderr, "       web serve\n")
	os.Exit(2)
}

func Main(configurer Configurer) {
	log.SetFlags(0)
	log.SetPrefix("web: ")

	ctx := context.Background()

	site, err := New(ctx, configurer)
	if err != nil {
		log.Fatalln(err)
	}
	ctx = contextWithSite(context.Background(), site)

	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "build":
		log.Println("building")
		if err := site.Build(ctx); err != nil {
			log.Fatalf("build: %s\n", err)
		}
	case "serve":
		log.Println("serving on http://localhost:8080")
		if err := site.Serve(":8080"); err != nil {
			log.Fatalf("serve: %s\n", err)
		}
	default:
		usage()
	}
}
