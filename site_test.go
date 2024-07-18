package web

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

func TestBuild(t *testing.T) {
	back := tempDir(t)
	defer back()

	ctx := context.Background()

	initialConfigurer := ConfigureFunc(func(ctx context.Context, s *Site) error {
		s.RenderFunc("index.html", func(ctx context.Context, w io.Writer) error {
			fmt.Fprintf(w, "<!doctype html>\n")
			fmt.Fprintf(w, "<img src=\"%s\">\n", Asset(ctx, "image.png"))
			return nil
		})
		// Follow-up configurer that creates the image.png asset
		s.ConfigureFunc(func(ctx context.Context, s *Site) error {
			if err := os.Mkdir("assets", 0755); err != nil {
				return err
			}
			if err := os.WriteFile("assets/image.png", []byte("image"), 0644); err != nil {
				return err
			}
			// Follow-up configurer that creates the robots.txt static file
			s.Configure(ConfigureFunc(func(ctx context.Context, s *Site) error {
				if err := os.Mkdir("public", 0755); err != nil {
					return err
				}
				if err := os.WriteFile("public/robots.txt", []byte("robot"), 0644); err != nil {
					return err
				}
				return nil
			}))
			return nil
		})
		return nil
	})

	site, err := New(ctx, initialConfigurer)
	if err != nil {
		t.Fatal(err)
	}
	if err := site.Build(contextWithSite(ctx, site)); err != nil {
		t.Fatal(err)
	}

	t.Run("check index.html", func(t *testing.T) {
		data, err := os.ReadFile("output/index.html")
		if err != nil {
			t.Fatal(err)
		}
		str := string(data)
		t.Logf("index.html content: %q", str)
		if !strings.Contains(str, "<!doctype html>") {
			t.Error("unexpected file content")
		}
		if !strings.Contains(str, `<img src="/assets/image.6105d6c.png">`) {
			t.Error("image tag not found in file")
		}
	})

	t.Run("check robots.txt", func(t *testing.T) {
		data, err := os.ReadFile("output/robots.txt")
		if err != nil {
			t.Fatal(err)
		}
		str := string(data)
		t.Logf("robots.txt content: %q", str)
		if str != "robot" {
			t.Error("unexpected file content")
		}
	})

	t.Run("check image.6105d6c.png", func(t *testing.T) {
		data, err := os.ReadFile("output/assets/image.6105d6c.png")
		if err != nil {
			t.Fatal(err)
		}
		str := string(data)
		t.Logf("image.6105d6c.png content: %q", str)
		if str != "image" {
			t.Error("unexpected file content")
		}
	})
}

func tempDir(t *testing.T) func() {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		os.RemoveAll(dir)
		t.Fatal(err)
	}
	return func() {
		if err := os.Chdir(wd); err != nil {
			os.RemoveAll(dir)
			t.Fatal(err)
		}
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
	}
}
