package analytics

import (
	"context"
	"errors"
	"strings"
)

const (
	defaultHotQuestionLimit = 10
	maxHotQuestionLimit     = 50
)

var ErrServiceNotConfigured = errors.New("analytics service is not configured")

type repository interface {
	IncrementKnowledgeAccess(ctx context.Context, itemID int64) error
	RecordQuery(ctx context.Context, record QueryRecord) error
	ListHotQuestions(ctx context.Context, limit int) ([]HotQuestion, error)
}

type Service struct {
	repository repository
}

func NewService(repository repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) IncrementKnowledgeAccess(ctx context.Context, itemID int64) error {
	if s == nil || s.repository == nil {
		return ErrServiceNotConfigured
	}
	return s.repository.IncrementKnowledgeAccess(ctx, itemID)
}

func (s *Service) RecordQuery(ctx context.Context, userID int64, userQuestion string, matchedItemID *int64, hitScore *float64) error {
	if s == nil || s.repository == nil {
		return ErrServiceNotConfigured
	}
	userQuestion = strings.TrimSpace(userQuestion)
	if userQuestion == "" {
		return errors.New("user question is required")
	}
	return s.repository.RecordQuery(ctx, QueryRecord{
		UserID:             userID,
		UserQuestion:       userQuestion,
		NormalizedQuestion: normalizeQuestion(userQuestion),
		MatchedItemID:      matchedItemID,
		HitScore:           hitScore,
	})
}

func (s *Service) HotQuestions(ctx context.Context, limit int) ([]HotQuestion, error) {
	if s == nil || s.repository == nil {
		return nil, ErrServiceNotConfigured
	}
	if limit <= 0 {
		limit = defaultHotQuestionLimit
	}
	if limit > maxHotQuestionLimit {
		limit = maxHotQuestionLimit
	}
	items, err := s.repository.ListHotQuestions(ctx, limit)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []HotQuestion{}
	}
	return items, nil
}

func normalizeQuestion(question string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(question)), " ")
}
