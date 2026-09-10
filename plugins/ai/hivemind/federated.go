package hivemind

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sync"
	"time"
)

type FederatedModel struct {
	Weights      map[string][]float64
	GlobalGrads  map[string][]float64
	AgentUpdates map[string]*AgentUpdate
	Round        int
	TotalAgents  int
	LearningRate float64
	MinAgents    int
	mu           sync.RWMutex
}

type AgentUpdate struct {
	AgentID   string
	Gradients map[string][]float64
	Timestamp int64
	DataSize  int
	Loss      float64
}

type VictimProfile struct {
	UserID        string
	LoginTimes    []float64
	TypingSpeed   float64
	AppUsage      map[string]float64
	Vulnerability float64
	PhishTime     []float64
	ProfileHash   string
	LastUpdated   int64
}

type FedAvgResult struct {
	GlobalLoss  float64
	AgentLosses map[string]float64
	Converged   bool
	Round       int
	Duration    time.Duration
}

func NewFederatedModel(learningRate float64, minAgents int) *FederatedModel {
	return &FederatedModel{
		Weights:      make(map[string][]float64),
		GlobalGrads:  make(map[string][]float64),
		AgentUpdates: make(map[string]*AgentUpdate),
		LearningRate: learningRate,
		MinAgents:    minAgents,
	}
}

func (fm *FederatedModel) RegisterAgent(agentID string) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.TotalAgents++
}

func (fm *FederatedModel) SubmitUpdate(agentID string, gradients map[string][]float64, loss float64, dataSize int) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	update := &AgentUpdate{
		AgentID:   agentID,
		Gradients: make(map[string][]float64),
		Timestamp: time.Now().Unix(),
		DataSize:  dataSize,
		Loss:      loss,
	}

	for k, v := range gradients {
		cp := make([]float64, len(v))
		copy(cp, v)
		update.Gradients[k] = cp
	}

	fm.AgentUpdates[agentID] = update
}

func (fm *FederatedModel) FedAvg() (*FedAvgResult, error) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if len(fm.AgentUpdates) < fm.MinAgents {
		return nil, fmt.Errorf("insufficient agents: %d < %d", len(fm.AgentUpdates), fm.MinAgents)
	}

	start := time.Now()
	layerCounts := make(map[string]int)
	layerSums := make(map[string][]float64)

	for _, update := range fm.AgentUpdates {
		for layer, grads := range update.Gradients {
			if layerSums[layer] == nil {
				layerSums[layer] = make([]float64, len(grads))
			}
			for i, g := range grads {
				layerSums[layer][i] += g
			}
			layerCounts[layer]++
		}
	}

	fm.GlobalGrads = make(map[string][]float64)
	for layer, sums := range layerSums {
		count := float64(layerCounts[layer])
		avg := make([]float64, len(sums))
		for i, s := range sums {
			avg[i] = s / count
		}
		fm.GlobalGrads[layer] = avg
	}

	for layer, grads := range fm.GlobalGrads {
		if fm.Weights[layer] == nil {
			fm.Weights[layer] = make([]float64, len(grads))
		}
		for i := range grads {
			fm.Weights[layer][i] -= fm.LearningRate * grads[i]
		}
	}

	totalLoss := 0.0
	agentLosses := make(map[string]float64)
	for _, update := range fm.AgentUpdates {
		totalLoss += update.Loss
		agentLosses[update.AgentID] = update.Loss
	}
	avgLoss := totalLoss / float64(len(fm.AgentUpdates))

	fm.Round++
	result := &FedAvgResult{
		GlobalLoss:  avgLoss,
		AgentLosses: agentLosses,
		Converged:   avgLoss < 0.01 && fm.Round > 5,
		Round:       fm.Round,
		Duration:    time.Since(start),
	}

	return result, nil
}

func (fm *FederatedModel) ProfileVictim(rawData map[string]interface{}) *VictimProfile {
	userID, _ := rawData["user_id"].(string)
	if userID == "" {
		id := make([]byte, 16)
		rand.Read(id)
		userID = fmt.Sprintf("%x", id[:8])
	}

	loginTimes := extractFloatArray(rawData, "login_times", []float64{9.0, 10.0, 14.0, 18.0})
	typingSpeed, _ := toFloat64(rawData["typing_speed"], 45.0)
	vulnerability, _ := toFloat64(rawData["vulnerability_score"], 0.5)

	appUsage := make(map[string]float64)
	if au, ok := rawData["app_usage"].(map[string]interface{}); ok {
		for k, v := range au {
			appUsage[k], _ = toFloat64(v, 0.0)
		}
	}

	phishTimes := extractFloatArray(rawData, "phish_times", []float64{9.5, 10.5, 14.5, 16.0})

	profile := &VictimProfile{
		UserID:        userID,
		LoginTimes:    loginTimes,
		TypingSpeed:   typingSpeed,
		AppUsage:      appUsage,
		Vulnerability: vulnerability,
		PhishTime:     phishTimes,
		LastUpdated:   time.Now().Unix(),
	}

	profile.ProfileHash = computeProfileHash(profile)
	return profile
}

func (fm *FederatedModel) PredictOptimalPhishTime(profile *VictimProfile) time.Time {
	if len(profile.PhishTime) == 0 {
		return time.Now().Add(time.Duration(9+int(time.Now().Unix()%6)) * time.Hour)
	}

	var bestHour float64
	var bestVuln float64

	for _, hour := range profile.PhishTime {
		vulnScore := profile.Vulnerability * (1.0 - math.Abs(hour-10.0)/10.0)
		if vulnScore > bestVuln {
			bestVuln = vulnScore
			bestHour = hour
		}
	}

	now := time.Now()
	phishTime := time.Date(now.Year(), now.Month(), now.Day()+1,
		int(bestHour), int((bestHour-float64(int(bestHour)))*60), 0, 0, now.Location())

	return phishTime.Add(time.Duration(24) * time.Hour)
}

func (fm *FederatedModel) AggregateProfiles(profiles []*VictimProfile) map[string]interface{} {
	agg := map[string]interface{}{
		"total_profiles":    len(profiles),
		"avg_vulnerability": 0.0,
		"most_common_hour":  0.0,
	}

	if len(profiles) == 0 {
		return agg
	}

	totalVuln := 0.0
	hourCount := make(map[int]int)

	for _, p := range profiles {
		totalVuln += p.Vulnerability
		for _, t := range p.LoginTimes {
			hourCount[int(t)]++
		}
	}

	agg["avg_vulnerability"] = totalVuln / float64(len(profiles))

	maxHour := 9
	maxCount := 0
	for h, c := range hourCount {
		if c > maxCount {
			maxCount = c
			maxHour = h
		}
	}
	agg["most_common_hour"] = float64(maxHour)

	return agg
}

func (fm *FederatedModel) ExportModel(path string) error {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	data, err := json.MarshalIndent(map[string]interface{}{
		"weights": fm.Weights,
		"round":   fm.Round,
		"grads":   fm.GlobalGrads,
		"lr":      fm.LearningRate,
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (fm *FederatedModel) ImportModel(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var saved map[string]interface{}
	if err := json.Unmarshal(data, &saved); err != nil {
		return err
	}

	fm.mu.Lock()
	defer fm.mu.Unlock()

	return nil
}

func (fm *FederatedModel) FullFedLearnSuite(numAgents int) map[string]interface{} {
	result := make(map[string]interface{})

	for i := 0; i < numAgents; i++ {
		agentID := fmt.Sprintf("agent-fed-%d", i)
		fm.RegisterAgent(agentID)

		grads := map[string][]float64{
			"fc1": {randFloat(), randFloat(), randFloat()},
			"fc2": {randFloat(), randFloat()},
		}
		fm.SubmitUpdate(agentID, grads, randFloat()*0.5, 100)
	}

	fedResult, err := fm.FedAvg()
	if err != nil {
		result["fedavg_error"] = err.Error()
	} else {
		result["round"] = fedResult.Round
		result["global_loss"] = fedResult.GlobalLoss
		result["converged"] = fedResult.Converged
		result["duration_ms"] = fedResult.Duration.Milliseconds()
	}

	profile := fm.ProfileVictim(map[string]interface{}{
		"user_id":             "user-001",
		"login_times":         []float64{8.5, 9.0, 9.5, 13.0, 14.0, 18.0},
		"typing_speed":        52.5,
		"vulnerability_score": 0.7,
		"phish_times":         []float64{9.5, 10.5, 14.0},
	})

	result["profile_hash"] = profile.ProfileHash[:8] + "..."
	result["optimal_phish"] = fm.PredictOptimalPhishTime(profile).Format("15:04")

	return result
}

func computeProfileHash(p *VictimProfile) string {
	h := sha256.New()
	h.Write([]byte(p.UserID))
	for _, t := range p.LoginTimes {
		binary.Write(h, binary.LittleEndian, t)
	}
	binary.Write(h, binary.LittleEndian, p.TypingSpeed)
	binary.Write(h, binary.LittleEndian, p.Vulnerability)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func extractFloatArray(data map[string]interface{}, key string, defaultVal []float64) []float64 {
	if arr, ok := data[key].([]float64); ok {
		return arr
	}
	if arr, ok := data[key].([]interface{}); ok {
		result := make([]float64, 0, len(arr))
		for _, v := range arr {
			if f, ok := toFloat64(v, 0); ok {
				result = append(result, f)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return defaultVal
}

func toFloat64(v interface{}, defaultVal float64) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	}
	return defaultVal, false
}

func randFloat() float64 {
	b := make([]byte, 8)
	rand.Read(b)
	bits := binary.LittleEndian.Uint64(b)
	return float64(bits%1000) / 1000.0
}
