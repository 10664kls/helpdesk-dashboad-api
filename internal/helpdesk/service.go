package helpdesk

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/10664kls/helpdesk-dashboad-api/internal/pager"
	"go.uber.org/zap"
)

type Service struct {
	db   *sql.DB
	zlog *zap.Logger
}

func NewService(_ context.Context, db *sql.DB, zlog *zap.Logger) (*Service, error) {
	if db == nil {
		return nil, errors.New("db is nil")
	}

	if zlog == nil {
		return nil, errors.New("zlog is nil")
	}

	return &Service{
		db:   db,
		zlog: zlog,
	}, nil
}

type ListHelpDesksResult struct {
	HelpDesks     []*HelpDesk `json:"helpDesks"`
	NextPageToken string      `json:"nextToken"`
}

func (s *Service) ListHelpDesks(ctx context.Context, in *HelpDeskQuery) (*ListHelpDesksResult, error) {
	zlog := s.zlog.With(
		zap.String("method", "ListHelpDesks"),
		zap.Any("query", in),
	)

	zlog.Info("starting to list help desks")

	helpDesks, err := listHelpDesks(ctx, s.db, in)
	if err != nil {
		zlog.Error("failed to list help desks", zap.Error(err))
		return nil, err
	}

	var pageToken string
	if l := len(helpDesks); l > 0 && l == int(pager.Size(in.PageSize)) {
		last := helpDesks[l-1]
		pageToken = pager.EncodeCursor(&pager.Cursor{
			ID:   last.ID,
			Time: last.CreatedAt,
		})
	}

	return &ListHelpDesksResult{
		HelpDesks:     helpDesks,
		NextPageToken: pageToken,
	}, nil
}

type HelpDesk struct {
	ID          string    `json:"id"`
	Number      string    `json:"number"`
	Category    string    `json:"category"`
	Priority    string    `json:"priority"` // HIGH, MEDIUM, LOW
	Status      string    `json:"status"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Requester   Staff     `json:"requester"`
	Supporter   Supporter `json:"supporter"`
	CreatedAt   time.Time `json:"createdAt"`
	ClosedDate  string    `json:"closedDate"`
}

type Staff struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Position    string `json:"position"`
	Department  string `json:"department"`
	Branch      string `json:"branch"`
}

type Supporter struct {
	DisplayName string `json:"displayName"`
	Position    string `json:"position"`
}
