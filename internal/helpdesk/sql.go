package helpdesk

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/10664kls/helpdesk-dashboad-api/internal/pager"
	sq "github.com/Masterminds/squirrel"
)

var ErrHelpDeskNotFound = errors.New("helpdesk not found")

type HelpDeskQuery struct {
	ID            string    `json:"id"`
	Number        string    `json:"number"`
	Category      string    `json:"category"`
	Priority      string    `json:"priority"`
	Status        string    `json:"status"`
	RequesterID   string    `json:"requesterId"`
	CreatedBefore time.Time `json:"createdBefore"`
	CreatedAfter  time.Time `json:"createdAfter"`
	PageSize      uint64    `json:"pageSize"`
	PageToken     string    `json:"pageToken"`
}

func (q *HelpDeskQuery) ToSql() (string, []any, error) {
	and := sq.And{}

	if q.Status != "" {
		and = append(and, sq.Expr("status LIKE %?%", q.Status))
	}

	if q.Priority != "" {
		and = append(and, sq.Eq{"priority": q.Priority})
	}

	if q.Category != "" {
		and = append(and, sq.Eq{"category": q.Category})
	}

	if q.Number != "" {
		and = append(and, sq.Eq{"number": q.Number})
	}

	if q.RequesterID != "" {
		and = append(and, sq.Eq{"creator_number": q.RequesterID})
	}
	if !q.CreatedBefore.IsZero() {
		and = append(and, sq.LtOrEq{"created_at": q.CreatedBefore})
	}
	if !q.CreatedAfter.IsZero() {
		and = append(and, sq.GtOrEq{"created_at": q.CreatedAfter})
	}

	if q.PageToken != "" {
		cursor, err := pager.DecodeCursor(q.PageToken)
		if err != nil {
			return "", nil, err
		}
		and = append(and, sq.Expr("id < ?", cursor.ID))
	}

	return and.ToSql()
}

func listHelpDesks(ctx context.Context, db *sql.DB, in *HelpDeskQuery) ([]*HelpDesk, error) {
	id := fmt.Sprintf("TOP %d id", pager.Size(in.PageSize))
	pred, args, err := in.ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to convert to sql: %w", err)
	}

	q, args := sq.
		Select(
			id,
			"number",
			"category",
			"priority",
			"status",
			"title",
			"description",
			"creator_number",
			"creator_display_name",
			"position",
			"department",
			"branch",
			"supporter_name",
			"supporter_position",
			"created_at",
			"closed_date",
		).
		From("v_helpdesk_report").
		PlaceholderFormat(sq.AtP).
		Where(pred, args...).
		OrderBy("id DESC").
		MustSql()

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	statements := make([]*HelpDesk, 0)
	for rows.Next() {
		var s HelpDesk
		var status, closedDate string
		err := rows.Scan(
			&s.ID,
			&s.Number,
			&s.Category,
			&s.Priority,
			&status,
			&s.Title,
			&s.Description,
			&s.Requester.ID,
			&s.Requester.DisplayName,
			&s.Requester.Position,
			&s.Requester.Department,
			&s.Requester.Branch,
			&s.Supporter.DisplayName,
			&s.Supporter.Position,
			&s.CreatedAt,
			&closedDate,
		)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrHelpDeskNotFound
		}

		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if closedDate != "1900-01-01" {
			s.ClosedDate = closedDate
		}
		s.Status = mapStatus(status)
		statements = append(statements, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}

	return statements, nil
}

func mapStatus(status string) string {
	switch status {
	case "PENDING",
		"FINISHED,MANAGER(APPROVE),IT(PENDING)":
		return "PENDING"

	case "FINISHED,MANAGER(APPROVE),IT(CANCEL)":
		return "CANCELED"

	case "FINISHED,MANAGER(APPROVE),IT(IN PROGRESS)":
		return "IN_PROGRESS"

	case "REQUEST":
		return "REQUEST"

	case "FINISHED,MANAGER(APPROVE),IT(RESOLVE)":
		return "RESOLVED"

	case "FINISHED,MANAGER(APPROVE),IT(SENDING)":
		return "SENDING"

	case "FINISHED,MANAGER(APPROVE),IT(REPENDING)":
		return "RE_PENDING"

	case "REJECT,MANAGER(REJECT)":
		return "REJECTED"

	default:
		return "PENDING"
	}
}
