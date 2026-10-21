package repo

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// Settings returns config.yaml of the project at dir with baseBranch and knowledge set. The
// default base branch main is left out. Other fields, order and comments stay. changed is false if
// nothing differs.
func Settings(dir, base string, knowledge map[string][]string) (out []byte, changed bool, err error) {
	path := ConfigPath(dir)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	doc := &yaml.Node{}
	if err := yaml.Unmarshal(data, doc); err != nil {
		return nil, false, fmt.Errorf("config.yaml: %w", err)
	}
	if doc.Kind == 0 {
		doc = &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, false, fmt.Errorf("config.yaml: no YAML mapping")
	}
	root := doc.Content[0]

	oldBase := DefaultBaseBranch
	if i := keyIndex(root, "baseBranch"); i >= 0 && root.Content[i+1].Value != "" {
		oldBase = root.Content[i+1].Value
	}
	var oldKnowledge map[string][]string
	if i := keyIndex(root, "knowledge"); i >= 0 {
		if err := root.Content[i+1].Decode(&oldKnowledge); err != nil {
			return nil, false, fmt.Errorf("config.yaml: %w", err)
		}
	}
	if base == "" {
		base = DefaultBaseBranch
	}
	knowledge, oldKnowledge = compactKnowledge(knowledge), compactKnowledge(oldKnowledge)
	sameKnowledge := equalKnowledge(oldKnowledge, knowledge)
	if base == oldBase && sameKnowledge {
		return data, false, nil
	}
	setBaseBranchNode(root, base)
	if !sameKnowledge {
		setKnowledgeNode(root, knowledge)
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, false, err
	}
	if err := enc.Close(); err != nil {
		return nil, false, err
	}
	// An empty mapping encodes as "{}".
	if len(root.Content) == 0 && root.HeadComment == "" && root.FootComment == "" && doc.HeadComment == "" && doc.FootComment == "" {
		return nil, true, nil
	}
	return buf.Bytes(), true, nil
}

// keyIndex is the position of key in the mapping n, -1 if it is missing.
func keyIndex(n *yaml.Node, key string) int {
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return i
		}
	}
	return -1
}

// compactKnowledge drops empty lists, nil without entries.
func compactKnowledge(k map[string][]string) map[string][]string {
	var out map[string][]string
	for key, files := range k {
		if len(files) == 0 {
			continue
		}
		if out == nil {
			out = map[string][]string{}
		}
		out[key] = files
	}
	return out
}

// equalKnowledge compares two knowledge sections including the order of files.
func equalKnowledge(a, b map[string][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, files := range a {
		if !slices.Equal(files, b[key]) {
			return false
		}
	}
	return true
}

// setBaseBranchNode sets baseBranch in root. The default main is left out.
func setBaseBranchNode(root *yaml.Node, branch string) {
	idx := keyIndex(root, "baseBranch")
	switch {
	case branch == DefaultBaseBranch && idx >= 0:
		deleteKey(root, idx)
	case branch == DefaultBaseBranch:
	case idx >= 0:
		if root.Content[idx+1].Value != branch {
			root.Content[idx+1].SetString(branch)
		}
	default:
		value := &yaml.Node{}
		value.SetString(branch)
		key := &yaml.Node{}
		key.SetString("baseBranch")
		// The comment at the top of the file stays at the top.
		if len(root.Content) > 0 {
			key.HeadComment, root.Content[0].HeadComment = root.Content[0].HeadComment, ""
		}
		root.Content = append([]*yaml.Node{key, value}, root.Content...)
	}
}

// setKnowledgeNode replaces the value of knowledge in root. The key keeps its comment, a new key
// goes to the end. An empty knowledge is left out.
func setKnowledgeNode(root *yaml.Node, knowledge map[string][]string) {
	idx := keyIndex(root, "knowledge")
	if len(knowledge) == 0 {
		if idx >= 0 {
			deleteKey(root, idx)
		}
		return
	}
	value := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, k := range KnowledgeKeys {
		files := knowledge[k]
		if len(files) == 0 {
			continue
		}
		key := &yaml.Node{}
		key.SetString(k)
		list := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, f := range files {
			item := &yaml.Node{}
			item.SetString(f)
			list.Content = append(list.Content, item)
		}
		value.Content = append(value.Content, key, list)
	}
	if idx >= 0 {
		root.Content[idx+1] = value
		return
	}
	key := &yaml.Node{}
	key.SetString("knowledge")
	root.Content = append(root.Content, key, value)
}

// deleteKey removes key and value at idx of root. A comment above the field moves to the next one.
func deleteKey(root *yaml.Node, idx int) {
	if c := root.Content[idx].HeadComment; c != "" && idx+2 < len(root.Content) {
		next := root.Content[idx+2]
		next.HeadComment = strings.TrimSuffix(c+"\n"+next.HeadComment, "\n")
	} else if c != "" {
		root.FootComment = strings.TrimPrefix(root.FootComment+"\n"+c, "\n")
	}
	root.Content = slices.Delete(root.Content, idx, idx+2)
}
