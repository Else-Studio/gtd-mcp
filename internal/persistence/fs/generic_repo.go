package fs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gtd/internal/domain"
)

type MarkdownCodec[T any] interface {
	Encode(entity T, now time.Time) (frontmatter []byte, title, desc string, err error)
	Decode(id, title, desc string, frontmatter []byte, now time.Time) (T, error)
}

type GenericRepo[T any] struct {
	rootDir string
	subDir  string
	codec   MarkdownCodec[T]
	clock   func() time.Time
}

func NewGenericRepo[T any](rootDir, subDir string, codec MarkdownCodec[T]) *GenericRepo[T] {
	return &GenericRepo[T]{
		rootDir: rootDir,
		subDir:  subDir,
		codec:   codec,
		clock:   time.Now,
	}
}

func (r *GenericRepo[T]) now() time.Time {
	if r.clock != nil {
		return r.clock()
	}
	return time.Now()
}

func (r *GenericRepo[T]) dir() string {
	return filepath.Join(r.rootDir, r.subDir)
}

func (r *GenericRepo[T]) Save(entity T, id string) error {
	now := r.now()
	frontmatter, title, desc, err := r.codec.Encode(entity, now)
	if err != nil {
		return err
	}

	fileContent := formatMarkdown(frontmatter, title, desc)

	dir := r.dir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := filepath.Join(dir, id+".md")
	return atomicWrite(path, fileContent)
}

func (r *GenericRepo[T]) Get(id string) (T, error) {
	var zero T
	path := filepath.Join(r.dir(), id+".md")
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return zero, fmt.Errorf("%w: %v", domain.ErrNotFound, err)
		}
		return zero, err
	}

	frontmatter, title, desc, err := parseMarkdown(content)
	if err != nil {
		return zero, err
	}

	now := r.now()
	return r.codec.Decode(id, title, desc, frontmatter, now)
}

func (r *GenericRepo[T]) Delete(id string) error {
	path := filepath.Join(r.dir(), id+".md")
	return os.Remove(path)
}

func (r *GenericRepo[T]) List() ([]T, error) {
	var entities []T
	var errs []error

	dir := r.dir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return entities, nil
		}
		return nil, err
	}

	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			id := strings.TrimSuffix(e.Name(), ".md")
			entity, err := r.Get(id)
			if err != nil {
				errs = append(errs, err)
			} else {
				entities = append(entities, entity)
			}
		}
	}
	return entities, errors.Join(errs...)
}
