package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Configurer interface {
	Configure(ctx context.Context, s *Site) error
}

type ConfigureFunc func(ctx context.Context, s *Site) error

func (cf ConfigureFunc) Configure(ctx context.Context, s *Site) error {
	return cf(ctx, s)
}

type Renderer interface {
	Render(ctx context.Context, w io.Writer) error
}

type RenderFunc func(ctx context.Context, w io.Writer) error

func (rf RenderFunc) Render(ctx context.Context, w io.Writer) error {
	return rf(ctx, w)
}

type Site struct {
	OutputDir string
	PublicDir string
	AssetsDir string

	configurers []Configurer
	renderers   map[string]Renderer
	assets      map[string]string
	commands    []string
}

func New(ctx context.Context, initialConfigurer Configurer) (*Site, error) {
	s := &Site{
		OutputDir: "output",
		PublicDir: "public",
		AssetsDir: "assets",

		configurers: []Configurer{initialConfigurer},
		renderers:   map[string]Renderer{},
		assets:      map[string]string{},
	}
	for len(s.configurers) > 0 {
		configurer := s.configurers[0]
		s.configurers = s.configurers[1:]
		if err := configurer.Configure(ctx, s); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Site) Configure(configurer Configurer) {
	s.configurers = append(s.configurers, configurer)
}

func (s *Site) ConfigureFunc(configurer func(ctx context.Context, s *Site) error) {
	s.Configure(ConfigureFunc(configurer))
}

func (s *Site) Render(path string, renderer Renderer) {
	s.renderers[path] = renderer
}

func (s *Site) RenderFunc(path string, renderer func(ctx context.Context, w io.Writer) error) {
	s.Render(path, RenderFunc(renderer))
}

func (s *Site) Run(command string) {
	s.commands = append(s.commands, command)
}

func (s *Site) Build(ctx context.Context) error {
	if err := os.RemoveAll(s.OutputDir); err != nil {
		return fmt.Errorf("remove output dir: %w", err)
	}
	if err := os.MkdirAll(s.OutputDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	if err := s.runCommands(ctx); err != nil {
		return fmt.Errorf("run commands: %w", err)
	}
	if err := s.copyAssets(); err != nil {
		return fmt.Errorf("copy assets: %w", err)
	}
	if err := s.copyPublicFiles(); err != nil {
		return fmt.Errorf("copy public files: %w", err)
	}
	for file, renderer := range s.renderers {
		if err := s.render(ctx, file, renderer); err != nil {
			log.Printf("error rendering %s: %s\n", file, err)
		}
	}
	return nil
}

func (s *Site) Asset(name string) string {
	name, ok := s.assets[name]
	if !ok {
		panic(fmt.Sprintf("asset %q not found", name))
	}
	return "/" + s.AssetsDir + "/" + name
}

func (s *Site) Serve(addr string) error {
	var (
		dir = http.Dir(s.OutputDir)
		fs  = http.FileServer(dir)
	)
	return http.ListenAndServe(addr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, ".html") && !strings.HasSuffix(r.URL.Path, "/") {
			p := r.URL.Path[1:] + ".html"
			file, err := dir.Open(p)
			if err == nil {
				file.Close()
				r.URL.Path += ".html"
			}
		}
		fs.ServeHTTP(w, r)
	}))
}

func (s *Site) render(ctx context.Context, file string, renderer Renderer) error {
	log.Printf("rendering %s\n", file)
	out := s.OutputDir + "/" + file
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		return err
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	if err := renderer.Render(ctx, f); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return nil
}

func (s *Site) runCommands(ctx context.Context) error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "sh"
	}
	for _, command := range s.commands {
		log.Printf("running %q\n", command)
		cmd := exec.CommandContext(ctx, shell, "-c", command)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%q: %w\n", command, err)
		}
	}
	return nil
}

func (s *Site) copyAssets() error {
	if _, err := os.Stat(s.AssetsDir); os.IsNotExist(err) {
		return nil
	}
	return filepath.Walk(s.AssetsDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return os.MkdirAll(filepath.Join(s.OutputDir, path), 0755)
		}
		name := strings.TrimPrefix(path, s.AssetsDir+"/")
		log.Printf("copying asset %s\n", name)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var (
			sum      = sha256.Sum256(data)
			digest   = hex.EncodeToString(sum[:])[:7]
			fileName = filepath.Base(path)
		)
		if index := strings.LastIndex(fileName, "."); index != -1 {
			fileName = fileName[0:index] + "." + digest + fileName[index:]
		} else {
			fileName += "." + digest
		}
		destPath := filepath.Join(s.OutputDir, filepath.Dir(path), fileName)
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return err
		}
		s.assets[name] = filepath.Join(filepath.Dir(name), fileName)
		return nil
	})
}

func (s *Site) copyPublicFiles() error {
	if _, err := os.Stat(s.PublicDir); os.IsNotExist(err) {
		return nil
	}
	return filepath.Walk(s.PublicDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == s.PublicDir {
			return nil
		}
		destPath := s.OutputDir + "/" + strings.TrimPrefix(path, s.PublicDir+"/")
		if info.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}
		dest, err := os.Create(destPath)
		if err != nil {
			return err
		}
		src, err := os.Open(path)
		if err != nil {
			dest.Close()
			return err
		}
		if _, err := io.Copy(dest, src); err != nil {
			dest.Close()
			src.Close()
			return err
		}
		if err := dest.Close(); err != nil {
			src.Close()
			return err
		}
		if err := src.Close(); err != nil {
			return err
		}
		return nil
	})
}
