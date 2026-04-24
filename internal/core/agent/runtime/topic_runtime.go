package runtime

import "context"

type topicOutputRuntime interface {
	SendWeComText(context.Context, string, string) error
}

func (s *Service) SetTopicOutputRuntime(output topicOutputRuntime) {
	s.topicOutput = output
}
