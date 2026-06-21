// Package config holds the sing-box-ez application configuration.
package config

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"sing-box-ez/internal/framework/fs"
)

// Profiles holds the config list and active selection.
type Profiles struct {
	Configs    []ConfigRecord `yaml:"configs" json:"configs"`
	ActiveName string         `yaml:"active_name" json:"active_name"`
	mu         sync.RWMutex
}

// LoadProfiles loads profiles from profiles.yaml inside root.
// If the file does not exist it returns an empty Profiles struct.
// A legacy profiles.json file is automatically migrated to profiles.yaml.
func LoadProfiles(root fs.Directory) (*Profiles, error) {
	yamlFile := root.File("profiles.yaml")
	jsonFile := root.File("profiles.json")

	data, err := yamlFile.Read()
	if err != nil {
		if os.IsNotExist(err) {
			// Migrate legacy JSON profiles if present.
			if legacy, readErr := jsonFile.Read(); readErr == nil && len(legacy) > 0 {
				var p Profiles
				if jerr := json.Unmarshal(legacy, &p); jerr == nil {
					if p.Configs == nil {
						p.Configs = []ConfigRecord{}
					}
					_ = jsonFile.Remove()
					_ = p.Save(root)
					return &p, nil
				}
			}
			return &Profiles{Configs: []ConfigRecord{}}, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return &Profiles{Configs: []ConfigRecord{}}, nil
	}
	var p Profiles
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	if p.Configs == nil {
		p.Configs = []ConfigRecord{}
	}
	return &p, nil
}

// Save writes profiles to profiles.yaml inside root.
func (p *Profiles) Save(root fs.Directory) error {
	p.mu.RLock()
	data, err := yaml.Marshal(p)
	p.mu.RUnlock()
	if err != nil {
		return err
	}
	return root.File("profiles.yaml").AtomicWrite(data, 0600)
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
