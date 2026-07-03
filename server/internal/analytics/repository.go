package analytics

import (
	"context"
	"database/sql"
	"errors"
)

var ErrRepositoryNotConfigured = errors.New("analytics repository is not configured")

type QueryRecord struct {
	UserID             int64
	UserQuestion       string
	NormalizedQuestion string
	Intent             string
	MatchedItemID      *int64
	HitScore           *float64
}

type HotQuestion struct {
	Question string `json:"question"`
	Count    int64  `json:"count"`
	Category string `json:"category"`
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) IncrementKnowledgeAccess(ctx context.Context, itemID int64) error {
	if r == nil || r.db == nil {
		return ErrRepositoryNotConfigured
	}
	if itemID <= 0 {
		return errors.New("knowledge item id is required")
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE knowledge_items
		SET access_count = access_count + 1,
		    last_accessed_at = now()
		WHERE id = $1
	`, itemID)
	return err
}

func (r *Repository) RecordQuery(ctx context.Context, record QueryRecord) error {
	if r == nil || r.db == nil {
		return ErrRepositoryNotConfigured
	}
	if record.UserQuestion == "" {
		return errors.New("user question is required")
	}

	var userID any
	if record.UserID > 0 {
		userID = record.UserID
	}

	var matchedItemID any
	if record.MatchedItemID != nil && *record.MatchedItemID > 0 {
		matchedItemID = *record.MatchedItemID
	}

	var hitScore any
	if record.HitScore != nil {
		hitScore = *record.HitScore
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO query_logs (
			user_id,
			user_question,
			normalized_question,
			intent,
			matched_item_id,
			hit_score
		) VALUES ($1, $2, $3, $4, $5, $6)
	`, userID, record.UserQuestion, record.NormalizedQuestion, record.Intent, matchedItemID, hitScore)
	return err
}

func (r *Repository) ListHotQuestions(ctx context.Context, limit int) ([]HotQuestion, error) {
	if r == nil || r.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			COALESCE(NULLIF(ql.normalized_question, ''), ql.user_question) AS question,
			COUNT(*) AS query_count,
			COALESCE(MAX(NULLIF(ki.category, '')), '') AS category
		FROM query_logs ql
		LEFT JOIN knowledge_items ki ON ki.id = ql.matched_item_id
		GROUP BY COALESCE(NULLIF(ql.normalized_question, ''), ql.user_question)
		ORDER BY COUNT(*) DESC, MAX(ql.created_at) DESC, question ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]HotQuestion, 0)
	for rows.Next() {
		var item HotQuestion
		if err := rows.Scan(&item.Question, &item.Count, &item.Category); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
