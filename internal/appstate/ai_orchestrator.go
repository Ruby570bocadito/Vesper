package appstate

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

type AIOrchestrator struct {
	mu           sync.RWMutex
	stateHistory []StateTransition
	predictions  []Prediction
	confidence   float64
	agentCount   int
	campaignID   string
	lastDecision time.Time
	rewardModel  *RewardModel
}

type StateTransition struct {
	From      string  `json:"from"`
	To        string  `json:"to"`
	Action    string  `json:"action"`
	Reward    float64 `json:"reward"`
	Delta     float64 `json:"delta"`
	AgentID   string  `json:"agent_id"`
	Timestamp int64   `json:"timestamp"`
}

type Prediction struct {
	NextState  string  `json:"next_state"`
	Confidence float64 `json:"confidence"`
	Action     string  `json:"action"`
	Risk       float64 `json:"risk"`
	Reward     float64 `json:"expected_reward"`
}

type RewardModel struct {
	TotalReward     float64
	EpisodeCount    int
	LastReward      float64
	LearningRate    float64
	ExplorationRate float64
	QTable          map[string]map[string]float64
}

const (
	defaultLearningRate    = 0.01
	defaultExplorationRate = 0.15
	minExplorationRate     = 0.02
	discountFactor         = 0.95
)

func NewAIOrchestrator() *AIOrchestrator {
	return &AIOrchestrator{
		confidence:   0.0,
		lastDecision: time.Now(),
		rewardModel: &RewardModel{
			LearningRate:    defaultLearningRate,
			ExplorationRate: defaultExplorationRate,
			QTable:          make(map[string]map[string]float64),
		},
	}
}

func (a *AIOrchestrator) RecordTransition(from, to, action, agentID string, reward float64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	transition := StateTransition{
		From:      from,
		To:        to,
		Action:    action,
		Reward:    reward,
		Delta:     reward - a.rewardModel.LastReward,
		AgentID:   agentID,
		Timestamp: time.Now().Unix(),
	}

	a.stateHistory = append(a.stateHistory, transition)
	if len(a.stateHistory) > 10000 {
		a.stateHistory = a.stateHistory[len(a.stateHistory)-5000:]
	}

	a.rewardModel.LastReward = reward
	a.rewardModel.TotalReward += reward
	a.rewardModel.EpisodeCount++

	a.updateQTable(from, action, to, reward)
}

func (a *AIOrchestrator) updateQTable(state, action, nextState string, reward float64) {
	if a.rewardModel.QTable[state] == nil {
		a.rewardModel.QTable[state] = make(map[string]float64)
	}

	oldValue := a.rewardModel.QTable[state][action]

	nextMax := 0.0
	if nextActions, ok := a.rewardModel.QTable[nextState]; ok {
		for _, v := range nextActions {
			if v > nextMax {
				nextMax = v
			}
		}
	}

	newValue := oldValue + a.rewardModel.LearningRate*(reward+discountFactor*nextMax-oldValue)
	a.rewardModel.QTable[state][action] = newValue
}

func (a *AIOrchestrator) PredictNextState(currentState string) Prediction {
	a.mu.RLock()
	defer a.mu.RUnlock()

	availableActions := a.getAvailableActions(currentState)

	if len(availableActions) == 0 {
		return Prediction{
			NextState:  currentState,
			Confidence: 0.0,
			Action:     "idle",
			Risk:       0.0,
			Reward:     0.0,
		}
	}

	if rand.Float64() < a.rewardModel.ExplorationRate {
		action := availableActions[rand.Intn(len(availableActions))]
		return Prediction{
			NextState:  a.deriveStateFromAction(currentState, action),
			Confidence: 0.3,
			Action:     action,
			Risk:       0.5,
			Reward:     a.rewardModel.TotalReward / math.Max(float64(a.rewardModel.EpisodeCount), 1),
		}
	}

	var bestAction string
	var bestValue float64 = -1e9

	for _, action := range availableActions {
		if values, ok := a.rewardModel.QTable[currentState]; ok {
			if v, ok := values[action]; ok && v > bestValue {
				bestValue = v
				bestAction = action
			}
		}
	}

	if bestAction == "" {
		bestAction = availableActions[rand.Intn(len(availableActions))]
		bestValue = 0.0
	}

	confidence := math.Min(sigmoid(float64(a.rewardModel.EpisodeCount)/100.0), 0.99)
	risk := a.calculateRisk(currentState, bestAction)

	return Prediction{
		NextState:  a.deriveStateFromAction(currentState, bestAction),
		Confidence: confidence,
		Action:     bestAction,
		Risk:       risk,
		Reward:     bestValue,
	}
}

func (a *AIOrchestrator) getAvailableActions(state string) []string {
	baseActions := []string{
		"recon", "exploit", "privesc", "persist",
		"lateral", "exfil", "evade", "rest",
	}

	stateActions := map[string][]string{
		"idle":         {"recon", "rest"},
		"recon":        {"exploit", "lateral", "rest"},
		"exploiting":   {"privesc", "persist", "exfil"},
		"privesc":      {"persist", "lateral", "exfil"},
		"persisting":   {"lateral", "exfil", "evade"},
		"lateral":      {"exploit", "privesc", "recon"},
		"exfiltrating": {"rest", "evade", "persist"},
		"evading":      {"persist", "lateral", "rest"},
	}

	if actions, ok := stateActions[state]; ok {
		return actions
	}
	return baseActions
}

func (a *AIOrchestrator) deriveStateFromAction(currentState, action string) string {
	actionToState := map[string]string{
		"recon":   "recon",
		"exploit": "exploiting",
		"privesc": "privesc",
		"persist": "persisting",
		"lateral": "lateral",
		"exfil":   "exfiltrating",
		"evade":   "evading",
		"rest":    "idle",
	}
	if nextState, ok := actionToState[action]; ok {
		return nextState
	}
	return currentState
}

func (a *AIOrchestrator) calculateRisk(state, action string) float64 {
	highRiskActions := []string{"exploit", "privesc", "lateral"}
	for _, a := range highRiskActions {
		if action == a {
			return 0.7 + rand.Float64()*0.2
		}
	}
	return 0.1 + rand.Float64()*0.2
}

func (a *AIOrchestrator) DecideNextAction(currentState string, availableModules []string) (string, Prediction) {
	prediction := a.PredictNextState(currentState)

	bestAction := prediction.Action
	bestMatch := 0.0

	for _, module := range availableModules {
		for _, action := range a.getAvailableActions(currentState) {
			if strings.Contains(strings.ToLower(module), action) {
				if matched, ok := a.rewardModel.QTable[currentState]; ok {
					if v, ok := matched[action]; ok && v > bestMatch {
						bestMatch = v
						bestAction = action
					}
				}
			}
		}
	}

	prediction.Action = bestAction
	a.lastDecision = time.Now()

	return bestAction, prediction
}

func (a *AIOrchestrator) GetStateHistory() []StateTransition {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]StateTransition, len(a.stateHistory))
	copy(result, a.stateHistory)
	return result
}

func (a *AIOrchestrator) GetRewardStats() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return map[string]interface{}{
		"total_reward":     a.rewardModel.TotalReward,
		"episodes":         a.rewardModel.EpisodeCount,
		"average_reward":   a.rewardModel.TotalReward / math.Max(float64(a.rewardModel.EpisodeCount), 1),
		"exploration_rate": a.rewardModel.ExplorationRate,
		"learning_rate":    a.rewardModel.LearningRate,
		"q_table_entries":  countQTableEntries(a.rewardModel.QTable),
	}
}

func countQTableEntries(qt map[string]map[string]float64) int {
	count := 0
	for _, actions := range qt {
		count += len(actions)
	}
	return count
}

func (a *AIOrchestrator) ExportQTable(path string) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	data, err := json.MarshalIndent(a.rewardModel.QTable, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (a *AIOrchestrator) ImportQTable(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var qTable map[string]map[string]float64
	if err := json.Unmarshal(data, &qTable); err != nil {
		return err
	}

	a.mu.Lock()
	a.rewardModel.QTable = qTable
	a.mu.Unlock()

	return nil
}

func (a *AIOrchestrator) SimulateEpisode(steps int) []StateTransition {
	states := []string{"idle", "recon", "exploiting", "privesc", "persisting", "lateral", "exfiltrating", "evading"}
	state := states[rand.Intn(len(states))]

	var episode []StateTransition
	for i := 0; i < steps; i++ {
		actions := a.getAvailableActions(state)
		action := actions[rand.Intn(len(actions))]
		nextState := a.deriveStateFromAction(state, action)
		reward := 0.1 + rand.Float64()*0.9

		transition := StateTransition{
			From:      state,
			To:        nextState,
			Action:    action,
			Reward:    reward,
			Timestamp: time.Now().Unix(),
		}
		episode = append(episode, transition)
		a.RecordTransition(state, nextState, action, "agent-sim-"+fmt.Sprintf("%d", rand.Intn(100)), reward)
		state = nextState
	}

	return episode
}

func (a *AIOrchestrator) GetTopActions(state string, topN int) []map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	type actionValue struct {
		action string
		value  float64
	}

	var pairs []actionValue
	if entries, ok := a.rewardModel.QTable[state]; ok {
		for action, value := range entries {
			pairs = append(pairs, actionValue{action, value})
		}
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].value > pairs[j].value
	})

	var top []map[string]interface{}
	for i := 0; i < len(pairs) && i < topN; i++ {
		top = append(top, map[string]interface{}{
			"action": pairs[i].action,
			"value":  pairs[i].value,
		})
	}

	return top
}

func (a *AIOrchestrator) FullAIOrchSuite() map[string]interface{} {
	result := make(map[string]interface{})

	stats := a.GetRewardStats()
	result["reward_stats"] = stats

	episode := a.SimulateEpisode(20)
	result["episode_steps"] = len(episode)

	prediction := a.PredictNextState("idle")
	result["prediction_idle"] = map[string]interface{}{
		"next_state": prediction.NextState,
		"action":     prediction.Action,
		"confidence": prediction.Confidence,
		"risk":       prediction.Risk,
	}

	topActions := a.GetTopActions("idle", 5)
	result["top_actions_idle"] = topActions

	result["state_history_size"] = len(a.stateHistory)
	result["platform"] = runtime.GOOS

	return result
}

func sigmoid(x float64) float64 {
	return 1.0 / (1.0 + math.Exp(-x))
}

var (
	_ = sort.Slice
	_ = json.MarshalIndent
)
