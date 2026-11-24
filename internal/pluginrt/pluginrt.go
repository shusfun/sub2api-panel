package pluginrt

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sort"
	"sync"

	toml "github.com/pelletier/go-toml/v2"
)

type Setting struct {
	Key         string   `json:"key"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Kind        string   `json:"kind"` // bool | int | string
	Default     any      `json:"default"`
	Min         int      `json:"min,omitempty"`
	Max         int      `json:"max,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

func (s Setting) Validate(value any) (any, bool) {
	switch s.Kind {
	case "bool":
		v, ok := value.(bool)
		return v, ok
	case "int":
		var n int
		switch v := value.(type) {
		case int:
			n = v
		case int64:
			n = int(v)
		case float64:
			if v != float64(int(v)) {
				return nil, false
			}
			n = int(v)
		default:
			return nil, false
		}
		if (s.Min != 0 || s.Max != 0) && (n < s.Min || n > s.Max) {
			return nil, false
		}
		return n, true
	case "string":
		v, ok := value.(string)
		if !ok || len(v) > 2048 {
			return nil, false
		}
		if len(s.Enum) > 0 {
			found := false
			for _, item := range s.Enum {
				if item == v {
					found = true
					break
				}
			}
			if !found {
				return nil, false
			}
		}
		return v, true
	default:
		return nil, false
	}
}

type Manifest struct {
	ID          string
	Name        string
	Description string
	Category    string
	DefaultOn   bool
	Settings    []Setting
	Apply       func(*Ctx) error
}

func (m Manifest) Compose(given map[string]any) map[string]any {
	out := make(map[string]any, len(m.Settings))
	for _, s := range m.Settings {
		out[s.Key] = s.Default
		if raw, ok := given[s.Key]; ok {
			if valid, ok := s.Validate(raw); ok {
				out[s.Key] = valid
			}
		}
	}
	return out
}

type Ctx struct {
	context.Context
	Config   map[string]any
	rt       *Runtime
	id       string
	disposer []func()
	provided []string
}

func (c *Ctx) OnDispose(fn func()) { c.disposer = append(c.disposer, fn) }

func (c *Ctx) Provide(name string, value any) {
	c.rt.services[name] = value
	c.provided = append(c.provided, name)
	c.OnDispose(func() { delete(c.rt.services, name) })
}

func (c *Ctx) Service(name string) any { return c.rt.services[name] }

func (c *Ctx) Note(text string) { c.rt.notes[c.id] = text }

type Status string

const (
	Off    Status = "off"
	Active Status = "active"
	Failed Status = "failed"
)

type State struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Category    string         `json:"category"`
	Enabled     bool           `json:"enabled"`
	Status      Status         `json:"status"`
	Reason      string         `json:"reason,omitempty"`
	Settings    []Setting      `json:"settings"`
	Config      map[string]any `json:"config"`
}

var (
	regMu    sync.Mutex
	registry = map[string]Manifest{}
)

func Register(m Manifest) {
	regMu.Lock()
	defer regMu.Unlock()
	if m.ID == "" || m.Apply == nil {
		panic("pluginrt: invalid manifest")
	}
	registry[m.ID] = m
}

func Registered() []Manifest {
	regMu.Lock()
	defer regMu.Unlock()
	out := make([]Manifest, 0, len(registry))
	for _, m := range registry {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

type Choices struct {
	Schema  int                       `toml:"schema"`
	Enabled map[string]bool           `toml:"enabled,omitempty"`
	Config  map[string]map[string]any `toml:"config,omitempty"`
}

func LoadChoices(path string) (Choices, error) {
	empty := Choices{Schema: 1, Enabled: map[string]bool{}, Config: map[string]map[string]any{}}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return empty, nil
		}
		return empty, err
	}
	var parsed Choices
	dec := toml.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&parsed); err != nil || parsed.Schema != 1 {
		return empty, err
	}
	if parsed.Enabled == nil {
		parsed.Enabled = map[string]bool{}
	}
	if parsed.Config == nil {
		parsed.Config = map[string]map[string]any{}
	}
	return parsed, nil
}

func SaveChoices(path string, c Choices) error {
	c.Schema = 1
	raw, err := toml.Marshal(c)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

type runningEntry struct {
	ctx    *Ctx
	cancel context.CancelFunc
}

type Runtime struct {
	mu        sync.RWMutex
	dataDir   string
	manifests []Manifest
	services  map[string]any
	running   map[string]*runningEntry
	failed    map[string]string
	notes     map[string]string
	enabled   map[string]bool
	config    map[string]map[string]any
}

func New(dataDir string, manifests []Manifest, core map[string]any) *Runtime {
	if dataDir == "" {
		dataDir = "./data"
	}
	rt := &Runtime{
		dataDir:   dataDir,
		manifests: manifests,
		services:  map[string]any{"data-dir": dataDir},
		running:   map[string]*runningEntry{},
		failed:    map[string]string{},
		notes:     map[string]string{},
		enabled:   map[string]bool{},
		config:    map[string]map[string]any{},
	}
	for k, v := range core {
		rt.services[k] = v
	}
	choices, _ := LoadChoices(rt.configPath())
	rt.Reconcile(choices.Enabled, choices.Config)
	return rt
}

func (r *Runtime) configPath() string {
	return filepath.Join(r.dataDir, "managed", "plugins.toml")
}

func (r *Runtime) Service(name string) any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.services[name]
}

func (r *Runtime) ConfigFor(id string) (bool, map[string]any) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	on := r.enabled[id]
	cfg := make(map[string]any, len(r.config[id]))
	for k, v := range r.config[id] {
		cfg[k] = v
	}
	return on, cfg
}

func (r *Runtime) Reconcile(enabled map[string]bool, cfg map[string]map[string]any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, run := range r.running {
		for i := len(run.ctx.disposer) - 1; i >= 0; i-- {
			run.ctx.disposer[i]()
		}
		run.cancel()
		delete(r.running, id)
	}
	r.failed = map[string]string{}
	r.notes = map[string]string{}
	r.enabled = map[string]bool{}
	r.config = map[string]map[string]any{}

	for _, m := range r.manifests {
		on := m.DefaultOn
		if explicit, ok := enabled[m.ID]; ok {
			on = explicit
		}
		composed := m.Compose(cfg[m.ID])
		r.enabled[m.ID] = on
		r.config[m.ID] = composed
		if !on {
			continue
		}
		ctx, cancel := context.WithCancel(context.Background())
		pctx := &Ctx{Context: ctx, Config: composed, rt: r, id: m.ID}
		if err := m.Apply(pctx); err != nil {
			cancel()
			r.failed[m.ID] = err.Error()
			continue
		}
		r.running[m.ID] = &runningEntry{ctx: pctx, cancel: cancel}
	}
}

func (r *Runtime) Update(id string, enabled *bool, patch map[string]any) ([]State, error) {
	var target *Manifest
	for i := range r.manifests {
		if r.manifests[i].ID == id {
			target = &r.manifests[i]
			break
		}
	}
	if target == nil {
		return nil, os.ErrNotExist
	}
	choices, _ := LoadChoices(r.configPath())
	if enabled != nil {
		choices.Enabled[id] = *enabled
	} else if _, ok := choices.Enabled[id]; !ok {
		choices.Enabled[id] = target.DefaultOn
	}
	if patch != nil {
		merged := target.Compose(choices.Config[id])
		for _, s := range target.Settings {
			val, given := patch[s.Key]
			if !given {
				continue
			}
			valid, ok := s.Validate(val)
			if !ok {
				return nil, os.ErrInvalid
			}
			merged[s.Key] = valid
		}
		choices.Config[id] = merged
	}
	if err := SaveChoices(r.configPath(), choices); err != nil {
		return nil, err
	}
	r.Reconcile(choices.Enabled, choices.Config)
	return r.States(), nil
}

func (r *Runtime) States() []State {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]State, 0, len(r.manifests))
	for _, m := range r.manifests {
		on := r.enabled[m.ID]
		st := Off
		reason := ""
		if on {
			if failReason, bad := r.failed[m.ID]; bad {
				st = Failed
				reason = failReason
			} else {
				st = Active
				reason = r.notes[m.ID]
			}
		}
		settings := m.Settings
		if settings == nil {
			settings = []Setting{}
		}
		cfg := make(map[string]any, len(r.config[m.ID]))
		for k, v := range r.config[m.ID] {
			cfg[k] = v
		}
		out = append(out, State{
			ID:          m.ID,
			Name:        m.Name,
			Description: m.Description,
			Category:    m.Category,
			Enabled:     on,
			Status:      st,
			Reason:      reason,
			Settings:    settings,
			Config:      cfg,
		})
	}
	return out
}
