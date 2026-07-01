package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// TaskTier P0 / P1 / P2 rollout gate.
type TaskTier string

const (
	TaskTierP0 TaskTier = "P0"
	TaskTierP1 TaskTier = "P1"
	TaskTierP2 TaskTier = "P2"
)

// TaskTierPolicy global switches (IDIP / config).
type TaskTierPolicy struct {
	P0Enabled bool `json:"p0Enabled"`
	P1Enabled bool `json:"p1Enabled"`
	P2Enabled bool `json:"p2Enabled"`
}

// TasksWelfareFile JSON config (§9 策划).
type TasksWelfareFile struct {
	Version         int            `json:"version"`
	Timezone        string         `json:"timezone"`
	Signin7dRewards []float64      `json:"signin7dRewards"`
	TierPolicy      TaskTierPolicy `json:"tierPolicy"`
	FeaturedGameID  string         `json:"featuredGameId"`
	Tasks           []TaskDefJSON  `json:"tasks"`
}

// TaskDefJSON overridable fields from file.
type TaskDefJSON struct {
	TaskID         string   `json:"taskId"`
	Enabled        *bool    `json:"enabled"`
	RewardGold     *float64 `json:"rewardGold"`
	Target         *float64 `json:"target"`
	AdBonusGold    *float64 `json:"adBonusGold"`
	AdBonusPercent *float64 `json:"adBonusPercent"`
}

var (
	taskRegistryMu sync.RWMutex
	taskCatalog    []TaskDef
	taskTierPolicy = TaskTierPolicy{P0Enabled: true, P1Enabled: false, P2Enabled: false}
	featuredGameID = "featured_game_1"
)

func init() {
	ReloadTaskRegistry("")
}

// ReloadTaskRegistry loads catalog + optional JSON overrides from path (empty = defaults only).
func ReloadTaskRegistry(configPath string) {
	taskRegistryMu.Lock()
	defer taskRegistryMu.Unlock()

	taskCatalog = BuildFullTaskCatalog()
	if path := resolveTasksConfigPath(configPath); path != "" {
		applyTasksWelfareFile(path)
	}
	if len(taskCatalog) == 0 {
		taskCatalog = BuildFullTaskCatalog()
	}
}

func resolveTasksConfigPath(explicit string) string {
	if p := filepath.Clean(strings.TrimSpace(explicit)); p != "" && p != "." {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	wd, _ := os.Getwd()
	for _, rel := range []string{
		"release/configs/tasks_welfare.json",
		"../release/configs/tasks_welfare.json",
		"../../release/configs/tasks_welfare.json",
	} {
		p := filepath.Join(wd, rel)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func applyTasksWelfareFile(path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var f TasksWelfareFile
	if json.Unmarshal(raw, &f) != nil {
		return
	}
	if len(f.Signin7dRewards) == 7 {
		Signin7dGoldRewards = append([]float64(nil), f.Signin7dRewards...)
	}
	taskTierPolicy = f.TierPolicy
	if taskTierPolicy.P0Enabled || taskTierPolicy.P1Enabled || taskTierPolicy.P2Enabled {
		// keep loaded policy
	} else {
		taskTierPolicy.P0Enabled = true
	}
	if strings.TrimSpace(f.FeaturedGameID) != "" {
		featuredGameID = strings.TrimSpace(f.FeaturedGameID)
	}
	byID := make(map[string]TaskDefJSON, len(f.Tasks))
	for _, t := range f.Tasks {
		if strings.TrimSpace(t.TaskID) != "" {
			byID[t.TaskID] = t
		}
	}
	for i := range taskCatalog {
		ov, ok := byID[taskCatalog[i].TaskID]
		if !ok {
			continue
		}
		if ov.Enabled != nil {
			taskCatalog[i].Enabled = *ov.Enabled
		}
		if ov.RewardGold != nil {
			taskCatalog[i].RewardGold = *ov.RewardGold
		}
		if ov.Target != nil {
			taskCatalog[i].Target = *ov.Target
		}
		if ov.AdBonusGold != nil {
			taskCatalog[i].AdBonusGold = *ov.AdBonusGold
		}
		if ov.AdBonusPercent != nil {
			taskCatalog[i].AdBonusPercent = *ov.AdBonusPercent
		}
	}
}

// GetTaskTierPolicy returns a copy of tier policy.
func GetTaskTierPolicy() TaskTierPolicy {
	taskRegistryMu.RLock()
	defer taskRegistryMu.RUnlock()
	return taskTierPolicy
}

// SetTaskTierPolicy updates P0/P1/P2 gates (IDIP).
func SetTaskTierPolicy(p TaskTierPolicy) {
	taskRegistryMu.Lock()
	defer taskRegistryMu.Unlock()
	taskTierPolicy = p
}

// FeaturedGameIDForTasks returns configured featured game id.
func FeaturedGameIDForTasks() string {
	taskRegistryMu.RLock()
	defer taskRegistryMu.RUnlock()
	return featuredGameID
}

func taskVisibleByTier(tier TaskTier) bool {
	p := taskTierPolicy
	switch tier {
	case TaskTierP0:
		return p.P0Enabled
	case TaskTierP1:
		return p.P1Enabled
	case TaskTierP2:
		return p.P2Enabled
	default:
		return false
	}
}

// ListActiveTaskDefs tasks exposed to players after tier + enabled.
func ListActiveTaskDefs() []TaskDef {
	taskRegistryMu.RLock()
	defer taskRegistryMu.RUnlock()
	out := make([]TaskDef, 0, len(taskCatalog))
	for _, d := range taskCatalog {
		if !d.Enabled || !taskVisibleByTier(d.Tier) {
			continue
		}
		out = append(out, d)
	}
	return out
}

// AllTaskDefsForAdmin all catalog entries (IDIP).
func AllTaskDefsForAdmin() []TaskDef {
	taskRegistryMu.RLock()
	defer taskRegistryMu.RUnlock()
	out := make([]TaskDef, len(taskCatalog))
	copy(out, taskCatalog)
	return out
}

func taskDefByID(taskID string) (TaskDef, bool) {
	taskRegistryMu.RLock()
	defer taskRegistryMu.RUnlock()
	for _, d := range taskCatalog {
		if d.TaskID == taskID {
			return d, true
		}
	}
	return TaskDef{}, false
}

// SetTaskEnabled toggles one task (IDIP/tests).
func SetTaskEnabled(taskID string, enabled bool) {
	taskRegistryMu.Lock()
	defer taskRegistryMu.Unlock()
	for i := range taskCatalog {
		if taskCatalog[i].TaskID == taskID {
			taskCatalog[i].Enabled = enabled
			return
		}
	}
}

// UpsertTaskOverride patches reward/enabled on catalog entry (IDIP).
func UpsertTaskOverride(taskID string, enabled *bool, rewardGold, target *float64) bool {
	taskRegistryMu.Lock()
	defer taskRegistryMu.Unlock()
	for i := range taskCatalog {
		if taskCatalog[i].TaskID != taskID {
			continue
		}
		if enabled != nil {
			taskCatalog[i].Enabled = *enabled
		}
		if rewardGold != nil {
			taskCatalog[i].RewardGold = *rewardGold
		}
		if target != nil {
			taskCatalog[i].Target = *target
		}
		return true
	}
	return false
}
