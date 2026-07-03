package analytics

import (
	"context"
	"errors"
	"testing"
)

func TestServiceRecordQueryNormalizesQuestion(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(repo)

	matchedItemID := int64(42)
	hitScore := 0.81
	if err := service.RecordQuery(context.Background(), 7, "  食堂   几点关门？ \n", &matchedItemID, &hitScore); err != nil {
		t.Fatalf("record query: %v", err)
	}

	if repo.record.UserID != 7 {
		t.Fatalf("expected user id 7, got %d", repo.record.UserID)
	}
	if repo.record.UserQuestion != "食堂   几点关门？" {
		t.Fatalf("unexpected user question: %q", repo.record.UserQuestion)
	}
	if repo.record.NormalizedQuestion != "食堂 几点关门？" {
		t.Fatalf("unexpected normalized question: %q", repo.record.NormalizedQuestion)
	}
	if repo.record.MatchedItemID == nil || *repo.record.MatchedItemID != 42 {
		t.Fatalf("unexpected matched item id: %#v", repo.record.MatchedItemID)
	}
	if repo.record.HitScore == nil || *repo.record.HitScore != 0.81 {
		t.Fatalf("unexpected hit score: %#v", repo.record.HitScore)
	}
}

func TestServiceHotQuestionsNormalizesLimit(t *testing.T) {
	repo := &fakeRepository{items: []HotQuestion{{Question: "食堂几点关门？", Count: 3}}}
	service := NewService(repo)

	items, err := service.HotQuestions(context.Background(), 100)
	if err != nil {
		t.Fatalf("hot questions: %v", err)
	}
	if repo.limit != maxHotQuestionLimit {
		t.Fatalf("expected capped limit %d, got %d", maxHotQuestionLimit, repo.limit)
	}
	if len(items) != 1 || items[0].Question != "食堂几点关门？" {
		t.Fatalf("unexpected items: %#v", items)
	}
}

func TestServiceHotQuestionsUsesDefaultLimit(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(repo)

	if _, err := service.HotQuestions(context.Background(), 0); err != nil {
		t.Fatalf("hot questions: %v", err)
	}
	if repo.limit != defaultHotQuestionLimit {
		t.Fatalf("expected default limit %d, got %d", defaultHotQuestionLimit, repo.limit)
	}
}

type fakeRepository struct {
	record QueryRecord
	items  []HotQuestion
	limit  int
	err    error
}

func (r *fakeRepository) IncrementKnowledgeAccess(ctx context.Context, itemID int64) error {
	return r.err
}

func (r *fakeRepository) RecordQuery(ctx context.Context, record QueryRecord) error {
	r.record = record
	return r.err
}

func (r *fakeRepository) ListHotQuestions(ctx context.Context, limit int) ([]HotQuestion, error) {
	r.limit = limit
	if r.err != nil {
		return nil, r.err
	}
	return r.items, nil
}

func TestServiceHotQuestionsReturnsRepositoryError(t *testing.T) {
	repo := &fakeRepository{err: errors.New("db unavailable")}
	service := NewService(repo)

	if _, err := service.HotQuestions(context.Background(), 5); err == nil || err.Error() != "db unavailable" {
		t.Fatalf("expected repository error, got %v", err)
	}
}
