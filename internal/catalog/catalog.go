package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ianunruh/deploybot/internal/spec"
)

type Catalog struct {
	Dir   string
	items map[string]*spec.Deployable
}

func Load(dir string) (*Catalog, error) {
	c := &Catalog{Dir: dir, items: map[string]*spec.Deployable{}}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("specs dir %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		d, err := spec.Load(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		c.items[d.Metadata.Name] = d
	}
	return c, nil
}

func (c *Catalog) Get(name string) (*spec.Deployable, error) {
	d, ok := c.items[name]
	if !ok {
		return nil, fmt.Errorf("unknown deployable %q", name)
	}
	return d, nil
}

func (c *Catalog) List() []*spec.Deployable {
	names := make([]string, 0, len(c.items))
	for n := range c.items {
		names = append(names, n)
	}
	slices.Sort(names)
	out := make([]*spec.Deployable, 0, len(names))
	for _, n := range names {
		out = append(out, c.items[n])
	}
	return out
}
