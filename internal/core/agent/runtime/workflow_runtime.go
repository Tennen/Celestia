package runtime

import "context"

type workflowOutputRuntime interface {
	SendWeComText(context.Context, string, string) error
}

func (s *Service) SetWorkflowOutputRuntime(output workflowOutputRuntime) {
	s.workflowOutput = output
}
