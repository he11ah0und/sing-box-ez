package config

import (
	"encoding/json"
	"os"
	"sing-box-ez/internal/paths"
	"sync"
	"time"
)

// Profiles holds the config list and active selection.
type Profiles struct {
	Configs    []ConfigRecord `json:"configs"`
	ActiveName string         `json:"active_name"`
	mu         sync.RWMutex
}

// LoadProfiles loads profiles from profiles.json.
// If the file does not exist it returns an empty Profiles struct.
func LoadProfiles() (*Profiles, error) {
	data, err := os.ReadFile(paths.Profiles())
	if err != nil {
		if os.IsNotExist(err) {
			return &Profiles{Configs: []ConfigRecord{}}, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return &Profiles{Configs: []ConfigRecord{}}, nil
	}
	var p Profiles
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	if p.Configs == nil {
		p.Configs = []ConfigRecord{}
	}
	return &p, nil
}

func (p *Profiles) Save() error {
	p.mu.RLock()
	data, err := json.MarshalIndent(p, "", "  ")
	p.mu.RUnlock()
	if err != nil {
		return err
	}
	return os.WriteFile(paths.Profiles(), data, 0600)
}

func (p *Profiles) GetConfigs() []ConfigRecord {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]ConfigRecord, len(p.Configs))
	copy(out, p.Configs)
	return out
}

func (p *Profiles) GetActiveConfig() *ConfigRecord {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.ActiveName == "" && len(p.Configs) > 0 {
		return &p.Configs[0]
	}
	for i := range p.Configs {
		if p.Configs[i].Name == p.ActiveName {
			return &p.Configs[i]
		}
	}
	return nil
}

func (p *Profiles) GetConfigByName(name string) *ConfigRecord {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for i := range p.Configs {
		if p.Configs[i].Name == name {
			return &p.Configs[i]
		}
	}
	return nil
}

func (p *Profiles) GetConfigsByParent(parent string) []ConfigRecord {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var out []ConfigRecord
	for i := range p.Configs {
		if p.Configs[i].Parent == parent {
			out = append(out, p.Configs[i])
		}
	}
	return out
}

func (p *Profiles) GetConfigByNameAndParent(name, parent string) *ConfigRecord {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for i := range p.Configs {
		if p.Configs[i].Name == name && p.Configs[i].Parent == parent {
			return &p.Configs[i]
		}
	}
	return nil
}

func (p *Profiles) SetActiveName(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ActiveName = name
}

func (p *Profiles) AddConfig(rec ConfigRecord) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Configs = append(p.Configs, rec)
}

func (p *Profiles) UpdateConfig(name string, rec ConfigRecord) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.Configs {
		if p.Configs[i].Name == name {
			p.Configs[i] = rec
			return
		}
	}
}

func (p *Profiles) RemoveConfig(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.Configs {
		if p.Configs[i].Name == name {
			p.Configs = append(p.Configs[:i], p.Configs[i+1:]...)
			if p.ActiveName == name {
				p.ActiveName = ""
			}
			return
		}
	}
}

func (p *Profiles) SetLastUpdateFor(name string, t time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.Configs {
		if p.Configs[i].Name == name {
			p.Configs[i].LastUpdate = Timestamp{Time: t}
			return
		}
	}
}

func (p *Profiles) RenameConfig(oldName, newName string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.Configs {
		if p.Configs[i].Name == oldName {
			p.Configs[i].Name = newName
			break
		}
	}
	if p.ActiveName == oldName {
		p.ActiveName = newName
	}
}
