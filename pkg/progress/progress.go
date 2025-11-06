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
	progresses       map[string]*PhaseProgress
	mutex            sync.RWMutex
	subscribers      map[int]chan ProgressEvent
	nextSubscriberID int
}

// NewProgressManager 創建進度管理器
func NewProgressManager() *ProgressManager {
	return &ProgressManager{
		progresses:  make(map[string]*PhaseProgress),
		subscribers: make(map[int]chan ProgressEvent),
	}
}

// ProgressEvent 進度事件
type ProgressEvent struct {
	Type     string         `json:"type"`
	Phase    string         `json:"phase"`
	Progress *PhaseProgress `json:"progress,omitempty"`
}

// Subscribe 訂閱進度事件
func (pm *ProgressManager) Subscribe() (<-chan ProgressEvent, func()) {
	pm.mutex.Lock()
	id := pm.nextSubscriberID
	pm.nextSubscriberID++
	ch := make(chan ProgressEvent, 16)
	pm.subscribers[id] = ch
	pm.mutex.Unlock()

	// 傳送當前快照
	snapshot := pm.GetAllProgress()
	go func() {
		for _, progress := range snapshot {
			pm.safeSend(ch, ProgressEvent{
				Type:     "progress",
				Phase:    progress.Phase,
				Progress: progress,
			})
		}
	}()

	unsubscribe := func() {
		pm.mutex.Lock()
		if existing, ok := pm.subscribers[id]; ok && existing == ch {
			delete(pm.subscribers, id)
		}
		pm.mutex.Unlock()
		close(ch)
	}

	return ch, unsubscribe
}

func (pm *ProgressManager) safeSend(ch chan ProgressEvent, event ProgressEvent) {
	defer func() {
		if recover() != nil {
		}
	}()
	select {
	case ch <- event:
	default:
	}
}

func (pm *ProgressManager) broadcast(event ProgressEvent) {
	pm.mutex.RLock()
	subs := make([]chan ProgressEvent, 0, len(pm.subscribers))
	for _, ch := range pm.subscribers {
		subs = append(subs, ch)
	}
	pm.mutex.RUnlock()

	for _, ch := range subs {
		pm.safeSend(ch, event)
	}
}

func (pm *ProgressManager) cloneProgress(progress *PhaseProgress) *PhaseProgress {
	if progress == nil {
		return nil
	}
	progressCopy := *progress
	if len(progress.Logs) > 0 {
		logsCopy := make([]LogEntry, len(progress.Logs))
		copy(logsCopy, progress.Logs)
		progressCopy.Logs = logsCopy
	}
	return &progressCopy
}

func (pm *ProgressManager) broadcastProgress(progress *PhaseProgress) {
	if progress == nil {
		return
	}
	pm.broadcast(ProgressEvent{
		Type:     "progress",
		Phase:    progress.Phase,
		Progress: progress,
	})
}

// StartPhase 開始一個 phase
func (pm *ProgressManager) StartPhase(phase string, totalSteps int) {
	pm.mutex.Lock()
	progress := &PhaseProgress{
		Phase:          phase,
		Status:         StatusRunning,
		Progress:       0,
		Message:        "Starting " + phase,
		StartTime:      time.Now(),
		TotalSteps:     totalSteps,
		CurrentStepNum: 0,
		Logs:           []LogEntry{},
	}
	pm.progresses[phase] = progress
	progressClone := pm.cloneProgress(progress)
	pm.mutex.Unlock()

	pm.broadcastProgress(progressClone)
}

// AddLog 添加日誌條目
func (pm *ProgressManager) AddLog(phase string, level string, message string) {
	pm.mutex.Lock()
	progress, exists := pm.progresses[phase]
	var progressClone *PhaseProgress
	if exists {
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
		progressClone = pm.cloneProgress(progress)
	}
	pm.mutex.Unlock()

	if exists {
		pm.broadcastProgress(progressClone)
	}
}

// UpdateProgress 更新進度
func (pm *ProgressManager) UpdateProgress(phase string, stepNum int, message string) {
	pm.mutex.Lock()
	progress, exists := pm.progresses[phase]
	var progressClone *PhaseProgress
	if exists {
		progress.CurrentStepNum = stepNum
		progress.CurrentStep = message
		progress.Message = message
		if progress.TotalSteps > 0 {
			progress.Progress = float64(stepNum) / float64(progress.TotalSteps) * 100
		}
		progressClone = pm.cloneProgress(progress)
	}
	pm.mutex.Unlock()

	if exists {
		pm.broadcastProgress(progressClone)
	}
}

// SetTotalSteps 更新總步驟數
func (pm *ProgressManager) SetTotalSteps(phase string, totalSteps int) {
	pm.mutex.Lock()
	progress, exists := pm.progresses[phase]
	var progressClone *PhaseProgress
	if exists {
		progress.TotalSteps = totalSteps
		if totalSteps > 0 {
			progress.Progress = float64(progress.CurrentStepNum) / float64(totalSteps) * 100
		} else {
			progress.Progress = 0
		}
		progressClone = pm.cloneProgress(progress)
	}
	pm.mutex.Unlock()

	if exists {
		pm.broadcastProgress(progressClone)
	}
}

// CompletePhase 完成 phase
func (pm *ProgressManager) CompletePhase(phase string) {
	pm.mutex.Lock()
	progress, exists := pm.progresses[phase]
	var progressClone *PhaseProgress
	if exists {
		progress.Status = StatusCompleted
		progress.Progress = 100
		progress.Message = phase + " completed successfully"
		progress.EndTime = time.Now()
		progressClone = pm.cloneProgress(progress)
	}
	pm.mutex.Unlock()

	if exists {
		pm.broadcastProgress(progressClone)
	}
}

// FailPhase phase 失敗
func (pm *ProgressManager) FailPhase(phase string, err error) {
	pm.mutex.Lock()
	progress, exists := pm.progresses[phase]
	var progressClone *PhaseProgress
	if exists {
		progress.Status = StatusFailed
		progress.Error = err.Error()
		progress.Message = phase + " failed: " + err.Error()
		progress.EndTime = time.Now()
		progressClone = pm.cloneProgress(progress)
	}
	pm.mutex.Unlock()

	if exists {
		pm.broadcastProgress(progressClone)
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
	return pm.cloneProgress(progress), true
}

// GetAllProgress 獲取所有 phase 進度
func (pm *ProgressManager) GetAllProgress() map[string]*PhaseProgress {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	result := make(map[string]*PhaseProgress)
	for phase, progress := range pm.progresses {
		// 返回副本避免競態條件
		result[phase] = pm.cloneProgress(progress)
	}
	return result
}

// ResetProgress 重置進度
func (pm *ProgressManager) ResetProgress(phase string) {
	pm.mutex.Lock()
	delete(pm.progresses, phase)
	pm.mutex.Unlock()

	pm.broadcast(ProgressEvent{Type: "progress", Phase: phase})
}
