package helpdesk

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"

	"github.com/10664kls/helpdesk-dashboad-api/internal/pager"
	"go.uber.org/zap"
)

type Service struct {
	db   *sql.DB
	zlog *zap.Logger
	mu   *sync.Mutex
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
		mu:   new(sync.Mutex),
	}, nil
}

type ListTicketsResult struct {
	Tickets       []*Ticket `json:"tickets"`
	NextPageToken string    `json:"nextPageToken"`
}

func (s *Service) ListTickets(ctx context.Context, in *TicketQuery) (*ListTicketsResult, error) {
	zlog := s.zlog.With(
		zap.String("method", "ListTickets"),
		zap.Any("query", in),
	)

	zlog.Info("starting to list tickets")

	tickets, err := listTickets(ctx, s.db, in)
	if err != nil {
		zlog.Error("failed to list tickets", zap.Error(err))
		return nil, err
	}

	var pageToken string
	if l := len(tickets); l > 0 && l == int(pager.Size(in.PageSize)) {
		last := tickets[l-1]
		pageToken = pager.EncodeCursor(&pager.Cursor{
			ID:   last.ID,
			Time: last.CreatedAt,
		})
	}

	return &ListTicketsResult{
		Tickets:       tickets,
		NextPageToken: pageToken,
	}, nil
}

type Ticket struct {
	ID          string    `json:"id"`
	Number      string    `json:"number"`
	Category    string    `json:"category"`
	Priority    string    `json:"priority"` // HIGH, MEDIUM, LOW
	Status      string    `json:"status"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Employee    Employee  `json:"employee"`
	Supporter   Supporter `json:"supporter"`
	CreatedAt   time.Time `json:"createdAt"`
	ClosedDate  time.Time `json:"closedDate"`
}

type Employee struct {
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
