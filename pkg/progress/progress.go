package progress

import (
	"sync"
	"time"
)

// ProgressStatus 進度狀態
type ProgressStatus string

const (
	StatusIdle      ProgressStatus = "idle"
	StatusRunning   ProgressStatus = "running"
	StatusCompleted ProgressStatus = "completed"
	StatusFailed    ProgressStatus = "failed"
)

// PhaseProgress Phase 進度信息
type PhaseProgress struct {
	Phase          string         `json:"phase"`
	Status         ProgressStatus `json:"status"`
	Progress       float64        `json:"progress"` // 0-100
	Message        string         `json:"message"`
	StartTime      time.Time      `json:"start_time,omitempty"`
	EndTime        time.Time      `json:"end_time,omitempty"`
	Error          string         `json:"error,omitempty"`
	CurrentStep    string         `json:"current_step,omitempty"`
	TotalSteps     int            `json:"total_steps,omitempty"`
	CurrentStepNum int            `json:"current_step_num,omitempty"`
	Logs           []LogEntry     `json:"logs,omitempty"`
}

// LogEntry 日誌條目
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
}

// ProgressManager 進度管理器
type ProgressManager struct {
	progresses map[string]*PhaseProgress
	mutex      sync.RWMutex
}

// NewProgressManager 創建進度管理器
func NewProgressManager() *ProgressManager {
	return &ProgressManager{
		progresses: make(map[string]*PhaseProgress),
	}
}

// StartPhase 開始一個 phase
func (pm *ProgressManager) StartPhase(phase string, totalSteps int) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	pm.progresses[phase] = &PhaseProgress{
		Phase:          phase,
		Status:         StatusRunning,
		Progress:       0,
		Message:        "Starting " + phase,
		StartTime:      time.Now(),
		TotalSteps:     totalSteps,
		CurrentStepNum: 0,
		Logs:           []LogEntry{},
	}
}

// AddLog 添加日誌條目
func (pm *ProgressManager) AddLog(phase string, level string, message string) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if progress, exists := pm.progresses[phase]; exists {
		logEntry := LogEntry{
			Timestamp: time.Now(),
			Level:     level,
			Message:   message,
		}
		progress.Logs = append(progress.Logs, logEntry)

		// 限制日誌數量，避免記憶體過度使用
		if len(progress.Logs) > 1000 {
			progress.Logs = progress.Logs[len(progress.Logs)-1000:]
		}
	}
}

// UpdateProgress 更新進度
func (pm *ProgressManager) UpdateProgress(phase string, stepNum int, message string) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if progress, exists := pm.progresses[phase]; exists {
		progress.CurrentStepNum = stepNum
		progress.CurrentStep = message
		progress.Message = message
		if progress.TotalSteps > 0 {
			progress.Progress = float64(stepNum) / float64(progress.TotalSteps) * 100
		}
	}
}

// CompletePhase 完成 phase
func (pm *ProgressManager) CompletePhase(phase string) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if progress, exists := pm.progresses[phase]; exists {
		progress.Status = StatusCompleted
		progress.Progress = 100
		progress.Message = phase + " completed successfully"
		progress.EndTime = time.Now()
	}
}

// FailPhase phase 失敗
func (pm *ProgressManager) FailPhase(phase string, err error) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if progress, exists := pm.progresses[phase]; exists {
		progress.Status = StatusFailed
		progress.Error = err.Error()
		progress.Message = phase + " failed: " + err.Error()
		progress.EndTime = time.Now()
	}
}

// GetProgress 獲取 phase 進度
func (pm *ProgressManager) GetProgress(phase string) (*PhaseProgress, bool) {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	progress, exists := pm.progresses[phase]
	if !exists {
		return &PhaseProgress{
			Phase:  phase,
			Status: StatusIdle,
		}, false
	}

	// 返回副本避免競態條件
	progressCopy := *progress
	return &progressCopy, true
}

// GetAllProgress 獲取所有 phase 進度
func (pm *ProgressManager) GetAllProgress() map[string]*PhaseProgress {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	result := make(map[string]*PhaseProgress)
	for phase, progress := range pm.progresses {
		// 返回副本避免競態條件
		progressCopy := *progress
		result[phase] = &progressCopy
	}
	return result
}

// ResetProgress 重置進度
func (pm *ProgressManager) ResetProgress(phase string) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	delete(pm.progresses, phase)
}
