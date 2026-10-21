package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// RepoDir ist der Projekt-Ordner von lazydev im Hauptordner eines Projekts.
const RepoDir = ".lazydev"

// KnowledgeAll steht in knowledge für Dateien, die jeder Schritt liest.
const KnowledgeAll = "all"

// KnowledgeKeys sind die erlaubten Schlüssel unter knowledge: all und die vier Schritte.
var KnowledgeKeys = []string{KnowledgeAll, "create", "implement", "review", "merge"}

// Repo sind die Einstellungen aus .lazydev/config.yaml. Der Ordner ist mit dem Projekt
// versioniert, alle im Team arbeiten damit gleich.
type Repo struct {
	// BaseBranch ist der Ziel-Branch: Von ihm zweigen Worktrees ab, in ihn mergt lazydev.
	BaseBranch string `yaml:"baseBranch,omitempty"`
	Preset     string `yaml:"preset,omitempty"`
	// Knowledge listet je Schritt Pfade relativ zum Projekt, die Claude zuerst liest.
	Knowledge    map[string][]string `yaml:"knowledge,omitempty"`
	AllowedTools []string            `yaml:"allowedTools,omitempty"`
	Hooks        Hooks               `yaml:"hooks,omitempty"`
}

// RepoPath ist .lazydev/config.yaml im Hauptordner projectDir.
func RepoPath(projectDir string) string {
	return filepath.Join(projectDir, RepoDir, "config.yaml")
}

// RepoExists meldet, ob .lazydev/config.yaml im Hauptordner projectDir existiert.
func RepoExists(projectDir string) bool {
	_, err := os.Stat(RepoPath(projectDir))
	return err == nil
}

// Base liefert BaseBranch, ohne Angabe DefaultBaseBranch.
func (r Repo) Base() string {
	if r.BaseBranch == "" {
		return DefaultBaseBranch
	}
	return r.BaseBranch
}

// trusted sind die Abschnitte, die Code ausführen oder Claudes Rechte erweitern.
type trusted struct {
	AllowedTools []string `yaml:"allowedTools,omitempty"`
	Hooks        Hooks    `yaml:"hooks,omitempty"`
}

// TrustSections liefert allowedTools und hooks als YAML, ohne beide leer.
func (r Repo) TrustSections() string {
	if len(r.AllowedTools) == 0 && r.Hooks == (Hooks{}) {
		return ""
	}
	data, _ := yaml.Marshal(trusted{AllowedTools: r.AllowedTools, Hooks: r.Hooks})
	return string(data)
}

// TrustHash ist der SHA-256 über allowedTools und hooks. Kommentare und andere Felder ändern ihn
// nicht. Ohne beide Abschnitte ist er leer.
func (r Repo) TrustHash() string {
	s := r.TrustSections()
	if s == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// Marshal liefert den Inhalt von .lazydev/config.yaml.
func (r Repo) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(r); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// LoadRepo liest .lazydev/config.yaml im Hauptordner projectDir. exists meldet, ob die Datei
// existiert. Unbekannte Felder und unbekannte Schritte unter knowledge sind ein Fehler.
func LoadRepo(projectDir string) (r Repo, exists bool, err error) {
	path := RepoPath(projectDir)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Repo{}, false, nil
	}
	if err != nil {
		return Repo{}, true, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return Repo{}, true, fmt.Errorf("%s: %w", path, err)
	}
	if err := checkRepoFields(&doc); err != nil {
		return Repo{}, true, fmt.Errorf("%s: %w", path, err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&r); err != nil && !errors.Is(err, io.EOF) {
		return Repo{}, true, fmt.Errorf("%s: %w", path, err)
	}
	if r.Preset != "" && !IsKnownPreset(r.Preset) {
		return Repo{}, true, fmt.Errorf("%s: unbekanntes Preset %q", path, r.Preset)
	}
	return r, true, nil
}

// checkRepoFields nennt das erste unbekannte Feld mit Namen. KnownFields allein liefert eine
// englische Meldung über mehrere Zeilen.
func checkRepoFields(doc *yaml.Node) error {
	root := mappingOf(doc)
	if root == nil {
		return nil
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		key, value := root.Content[i].Value, root.Content[i+1]
		switch key {
		case "baseBranch", "preset", "allowedTools":
		case "hooks":
			for _, k := range mappingKeys(value) {
				if !slices.Contains([]string{"setup", "teardown", "check", "dev"}, k) {
					return fmt.Errorf("unbekanntes Feld %q", "hooks."+k)
				}
			}
		case "knowledge":
			for _, k := range mappingKeys(value) {
				if !slices.Contains(KnowledgeKeys, k) {
					return fmt.Errorf("unbekannter Schritt %q unter knowledge", k)
				}
			}
		default:
			return fmt.Errorf("unbekanntes Feld %q", key)
		}
	}
	return nil
}

// mappingOf liefert die Zuordnung auf oberster Ebene eines Dokuments, ohne eine nil.
func mappingOf(doc *yaml.Node) *yaml.Node {
	n := doc
	if n.Kind == yaml.DocumentNode {
		if len(n.Content) == 0 {
			return nil
		}
		n = n.Content[0]
	}
	if n.Kind != yaml.MappingNode {
		return nil
	}
	return n
}

func mappingKeys(n *yaml.Node) []string {
	if n.Kind != yaml.MappingNode {
		return nil
	}
	var keys []string
	for i := 0; i+1 < len(n.Content); i += 2 {
		keys = append(keys, n.Content[i].Value)
	}
	return keys
}

// SetRepoBaseBranch schreibt baseBranch in .lazydev/config.yaml. Der Standard main steht dort
// nicht, das Feld entfällt dann. Andere Felder, Reihenfolge und Kommentare bleiben erhalten.
func SetRepoBaseBranch(projectDir, branch string) error {
	path := RepoPath(projectDir)
	_, root, err := readRepoNode(path)
	if err != nil {
		return err
	}
	_, knowledge, err := repoSettings(root)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	_, err = SetRepoSettings(projectDir, branch, knowledge)
	return err
}

// SetRepoSettings schreibt baseBranch und knowledge in .lazydev/config.yaml. Der Standard main und
// eine leere knowledge stehen dort nicht, die Felder entfallen dann. Die Schlüssel unter knowledge
// folgen KnowledgeKeys, die Dateien je Schlüssel bleiben in ihrer Reihenfolge. Andere Felder,
// Reihenfolge und Kommentare bleiben erhalten. Ergeben beide Werte denselben Inhalt wie vorher,
// schreibt SetRepoSettings nichts und liefert changed == false.
func SetRepoSettings(projectDir, base string, knowledge map[string][]string) (changed bool, err error) {
	path := RepoPath(projectDir)
	doc, root, err := readRepoNode(path)
	if err != nil {
		return false, err
	}
	oldBase, oldKnowledge, err := repoSettings(root)
	if err != nil {
		return false, fmt.Errorf("%s: %w", path, err)
	}
	if base == "" {
		base = DefaultBaseBranch
	}
	knowledge = compactKnowledge(knowledge)
	sameKnowledge := equalKnowledge(oldKnowledge, knowledge)
	if base == oldBase && sameKnowledge {
		return false, nil
	}
	setBaseBranchNode(root, base)
	if !sameKnowledge {
		setKnowledgeNode(root, knowledge)
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return false, err
	}
	if err := enc.Close(); err != nil {
		return false, err
	}
	out := buf.Bytes()
	// Eine leere Zuordnung schreibt yaml als "{}".
	if len(root.Content) == 0 && root.HeadComment == "" && root.FootComment == "" && doc.HeadComment == "" && doc.FootComment == "" {
		out = nil
	}
	if err := writeFileAtomic(path, out, 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// readRepoNode liest .lazydev/config.yaml unter path als Dokument und liefert dazu die Zuordnung
// auf oberster Ebene. Eine leere Datei gilt als leere Zuordnung.
func readRepoNode(path string) (doc, root *yaml.Node, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	doc = &yaml.Node{}
	if err := yaml.Unmarshal(data, doc); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", path, err)
	}
	if doc.Kind == 0 {
		doc = &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	}
	root = mappingOf(doc)
	if root == nil {
		return nil, nil, fmt.Errorf("%s: keine YAML-Zuordnung", path)
	}
	return doc, root, nil
}

// repoSettings liefert baseBranch, ohne Angabe DefaultBaseBranch, und knowledge ohne leere Listen.
func repoSettings(root *yaml.Node) (base string, knowledge map[string][]string, err error) {
	base = DefaultBaseBranch
	if i := keyIndex(root, "baseBranch"); i >= 0 && root.Content[i+1].Value != "" {
		base = root.Content[i+1].Value
	}
	if i := keyIndex(root, "knowledge"); i >= 0 {
		if err := root.Content[i+1].Decode(&knowledge); err != nil {
			return "", nil, err
		}
	}
	return base, compactKnowledge(knowledge), nil
}

// keyIndex liefert die Position des Schlüssels key in der Zuordnung n, sonst -1.
func keyIndex(n *yaml.Node, key string) int {
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return i
		}
	}
	return -1
}

// compactKnowledge entfernt leere Listen. Ohne Einträge liefert es nil.
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

// equalKnowledge vergleicht zwei knowledge-Zuordnungen samt Reihenfolge der Dateien.
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

// setBaseBranchNode setzt baseBranch in der Zuordnung root. Der Standard main entfällt.
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
		// Der Kommentar am Anfang der Datei bleibt am Anfang.
		if len(root.Content) > 0 {
			key.HeadComment, root.Content[0].HeadComment = root.Content[0].HeadComment, ""
		}
		root.Content = append([]*yaml.Node{key, value}, root.Content...)
	}
}

// setKnowledgeNode ersetzt den Wert von knowledge in der Zuordnung root. Der Schlüssel behält
// seinen Kommentar, ohne Schlüssel kommt er ans Ende. Eine leere knowledge entfällt.
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

// deleteKey entfernt Schlüssel und Wert an Position idx der Zuordnung root. Ein Kommentar über dem
// Feld bleibt über dem nächsten Feld stehen.
func deleteKey(root *yaml.Node, idx int) {
	if c := root.Content[idx].HeadComment; c != "" && idx+2 < len(root.Content) {
		next := root.Content[idx+2]
		next.HeadComment = strings.TrimSuffix(c+"\n"+next.HeadComment, "\n")
	} else if c != "" {
		root.FootComment = strings.TrimPrefix(root.FootComment+"\n"+c, "\n")
	}
	root.Content = slices.Delete(root.Content, idx, idx+2)
}

// Effective liefert die wirksamen Einstellungen eines Projekts. Mit .lazydev/config.yaml gelten nur
// deren Werte, ignored nennt dann die lokalen Felder, die in der zentralen config.yaml noch stehen.
// Ohne die Datei gelten die Übergangsfelder der zentralen config.yaml.
func Effective(p Project) (r Repo, ignored []string, err error) {
	repo, exists, err := LoadRepo(p.AbsPath())
	if err != nil {
		return Repo{}, nil, err
	}
	if !exists {
		return Repo{BaseBranch: p.BaseBranch, Preset: p.Preset, AllowedTools: p.AllowedTools, Hooks: p.Hooks}, nil, nil
	}
	if p.BaseBranch != "" {
		ignored = append(ignored, "baseBranch")
	}
	if p.Preset != "" {
		ignored = append(ignored, "preset")
	}
	if len(p.AllowedTools) > 0 {
		ignored = append(ignored, "allowedTools")
	}
	if p.Hooks != (Hooks{}) {
		ignored = append(ignored, "hooks")
	}
	return repo, ignored, nil
}

// RemoveLocalProjectSettings leert die Übergangsfelder des Projekts an Position i, nachdem
// .lazydev/config.yaml sie übernommen hat.
func (c *Config) RemoveLocalProjectSettings(i int) {
	if i < 0 || i >= len(c.Projects) {
		return
	}
	p := &c.Projects[i]
	p.BaseBranch, p.Preset, p.AllowedTools, p.Hooks = "", "", nil, Hooks{}
}

// writeFileAtomic schreibt über eine temporäre Datei im selben Ordner mit den Rechten perm.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
