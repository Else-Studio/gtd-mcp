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

func isSyncConflictName(name string) bool {
	return strings.Contains(name, ".sync-conflict-")
}

// skipped are on-disk filenames, not entity ids.
// err is directory-level only (ReadDir other than missing dir).
func (r *GenericRepo[T]) ListDetail() (entities []T, skipped []string, decodeErrs []error, err error) {
	entries, readErr := os.ReadDir(r.dir())
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return entities, skipped, decodeErrs, nil
		}
		return nil, nil, nil, readErr
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		if isSyncConflictName(name) {
			skipped = append(skipped, name)
			continue
		}
		id := strings.TrimSuffix(name, ".md")
		entity, getErr := r.Get(id)
		if getErr != nil {
			decodeErrs = append(decodeErrs, fmt.Errorf("%s: %w", name, getErr))
		} else {
			entities = append(entities, entity)
		}
	}
	return entities, skipped, decodeErrs, nil
}

func (r *GenericRepo[T]) List() ([]T, error) {
	entities, _, decodeErrs, err := r.ListDetail()
	if err != nil {
		return entities, err
	}
	return entities, errors.Join(decodeErrs...)
}
