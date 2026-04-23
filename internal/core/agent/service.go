package agent

import (
	"github.com/chentianyu/celestia/internal/core/agent/runtime"
	"github.com/chentianyu/celestia/internal/core/eventbus"
	"github.com/chentianyu/celestia/internal/storage"
)

type Service = runtime.Service
type MarketRunRequest = runtime.MarketRunRequest
type EvolutionGoalRequest = runtime.EvolutionGoalRequest
type WritingTopicRequest = runtime.WritingTopicRequest
type WritingMaterialRequest = runtime.WritingMaterialRequest
type WritingStateUpdateRequest = runtime.WritingStateUpdateRequest

func New(store storage.Store, bus *eventbus.Bus) *Service {
	return runtime.New(store, bus)
}
