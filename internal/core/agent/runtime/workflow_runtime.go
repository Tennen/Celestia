package runtime

import (
	"context"

	"github.com/chentianyu/celestia/internal/models"
)

type workflowOutputRuntime interface {
	SendWeComText(context.Context, string, string) error
}

type workflowInputRuntime interface {
	HandleInput(context.Context, models.ProjectInputRequest) (models.ProjectInputResult, error)
}

func (s *Service) SetWorkflowOutputRuntime(output workflowOutputRuntime) {
	s.workflowOutput = output
}

func (s *Service) SetWorkflowInputRuntime(input workflowInputRuntime) {
	s.workflowInput = input
}
