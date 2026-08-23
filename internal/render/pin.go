package render

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/ianunruh/deploybot/internal/image"
	"github.com/ianunruh/deploybot/internal/spec"
	"github.com/ianunruh/deploybot/internal/yamlx"
)

// Pin writes the image override into the stage overlay kustomization only.
func Pin(tree Tree, d *spec.Deployable, stage string, ref image.Ref) error {
	if _, err := d.Stage(stage); err != nil {
		return err
	}
	p := OverlayKustomizationPath(d, stage)
	pinned, err := PinOverlay(tree[p], d.Spec.Image.Repository, ref)
	if err != nil {
		return fmt.Errorf("pin %s: %w", p, err)
	}
	tree[p] = pinned
	return nil
}

// PinOverlay upserts an images: entry on an overlay kustomization, preserving
// unrelated keys (configMapGenerator, patches, comments) when the file exists.
func PinOverlay(existing []byte, repository string, ref image.Ref) ([]byte, error) {
	img := kustomizeImage{
		Name:   repository,
		NewTag: ref.Tag,
		Digest: ref.Digest,
	}
	if ref.Repository != "" && ref.Repository != repository {
		img.NewName = ref.Repository
	}
	if len(bytes.TrimSpace(existing)) == 0 {
		return yamlx.Marshal(kustomization{
			APIVersion: "kustomize.config.k8s.io/v1beta1",
			Kind:       "Kustomization",
			Resources:  []string{"../../base"},
			Images:     []kustomizeImage{img},
		})
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(existing, &doc); err != nil {
		return nil, err
	}
	if err := upsertImageNode(&doc, img); err != nil {
		return nil, err
	}
	root := mappingRoot(&doc)
	if root == nil {
		return nil, fmt.Errorf("kustomization is not a mapping")
	}
	return yamlx.Marshal(root)
}

func upsertImageNode(doc *yaml.Node, img kustomizeImage) error {
	root := mappingRoot(doc)
	if root == nil {
		return fmt.Errorf("kustomization is not a mapping")
	}
	seq := mappingValue(root, "images")
	if seq == nil {
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "images"},
			&yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: []*yaml.Node{imageNode(img)}},
		)
		return nil
	}
	if seq.Kind != yaml.SequenceNode {
		return fmt.Errorf("images is not a sequence")
	}
	for i, item := range seq.Content {
		if item.Kind != yaml.MappingNode {
			continue
		}
		if scalarValue(item, "name") == img.Name {
			seq.Content[i] = imageNode(img)
			return nil
		}
	}
	seq.Content = append(seq.Content, imageNode(img))
	return nil
}

func mappingRoot(doc *yaml.Node) *yaml.Node {
	switch doc.Kind {
	case yaml.DocumentNode:
		if len(doc.Content) == 0 {
			return nil
		}
		if doc.Content[0].Kind == yaml.MappingNode {
			return doc.Content[0]
		}
	case yaml.MappingNode:
		return doc
	}
	return nil
}

func mappingValue(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func scalarValue(m *yaml.Node, key string) string {
	v := mappingValue(m, key)
	if v == nil {
		return ""
	}
	return v.Value
}

func imageNode(img kustomizeImage) *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	addKV := func(k, v string) {
		if v == "" {
			return
		}
		n.Content = append(n.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v},
		)
	}
	addKV("name", img.Name)
	addKV("newName", img.NewName)
	addKV("newTag", img.NewTag)
	addKV("digest", img.Digest)
	return n
}
