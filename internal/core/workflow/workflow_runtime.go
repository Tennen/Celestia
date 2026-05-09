package workflow

import (
	"context"

	"github.com/chentianyu/celestia/internal/models"
)

type OutputRuntime interface {
	SendWeComText(context.Context, string, string) error
}

type InputRuntime interface {
	HandleInput(context.Context, models.ProjectInputRequest) (models.ProjectInputResult, error)
}

func (s *Service) SetWorkflowOutputRuntime(output OutputRuntime) {
	s.workflowOutput = output
}

func (s *Service) SetWorkflowInputRuntime(input InputRuntime) {
	s.workflowInput = input
}
