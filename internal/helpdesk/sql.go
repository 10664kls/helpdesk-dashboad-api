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

var ErrTicketNotFound = errors.New("ticket not found")

type TicketQuery struct {
	ID            string    `json:"id" query:"id"`
	Number        string    `json:"number" query:"number"`
	Category      string    `json:"category" query:"category"`
	Priority      string    `json:"priority" query:"priority"`
	Status        string    `json:"status" query:"status"`
	EmployeeID    string    `json:"employeeId" query:"employeeId"`
	CreatedBefore time.Time `json:"createdBefore" query:"createdBefore"`
	CreatedAfter  time.Time `json:"createdAfter" query:"createdAfter"`
	PageSize      uint64    `json:"pageSize" query:"pageSize"`
	PageToken     string    `json:"pageToken" query:"pageToken"`
}

func (q *TicketQuery) ToSql() (string, []any, error) {
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

	if q.EmployeeID != "" {
		and = append(and, sq.Eq{"creator_number": q.EmployeeID})
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

func listTickets(ctx context.Context, db *sql.DB, in *TicketQuery) ([]*Ticket, error) {
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
		From("v_hepldesk_ticket_report").
		PlaceholderFormat(sq.AtP).
		Where(pred, args...).
		OrderBy("id DESC").
		MustSql()

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	tickets := make([]*Ticket, 0)
	for rows.Next() {
		var s Ticket
		var status string
		err := rows.Scan(
			&s.ID,
			&s.Number,
			&s.Category,
			&s.Priority,
			&status,
			&s.Title,
			&s.Description,
			&s.Employee.ID,
			&s.Employee.DisplayName,
			&s.Employee.Position,
			&s.Employee.Department,
			&s.Employee.Branch,
			&s.Supporter.DisplayName,
			&s.Supporter.Position,
			&s.CreatedAt,
			&s.ClosedDate,
		)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTicketNotFound
		}

		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		s.Status = mapTicketStatus(status)
		tickets = append(tickets, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}

	return tickets, nil
}

func mapTicketStatus(status string) string {
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

type BatchGetTicketsQuery struct {
	ID            string    `json:"id" query:"id"`
	Number        string    `json:"number" query:"number"`
	Category      string    `json:"category" query:"category"`
	Priority      string    `json:"priority" query:"priority"`
	Status        string    `json:"status" query:"status"`
	RequesterID   string    `json:"requesterId" query:"requesterId"`
	CreatedBefore time.Time `json:"createdBefore" query:"createdBefore"`
	CreatedAfter  time.Time `json:"createdAfter" query:"createdAfter"`

	nextID string
}

func (q *BatchGetTicketsQuery) ToSql() (string, []any, error) {
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

	if q.nextID != "" {
		and = append(and, sq.Lt{"id": q.nextID})
	}

	return and.ToSql()
}

func batchGetTickets(ctx context.Context, db *sql.DB, batchSize int, nextID string, in *BatchGetTicketsQuery) ([]*Ticket, error) {
	id := fmt.Sprintf("TOP %d id", batchSize)
	in.nextID = nextID
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
		From("v_hepldesk_ticket_report").
		PlaceholderFormat(sq.AtP).
		Where(pred, args...).
		OrderBy("id DESC").
		MustSql()

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	tickets := make([]*Ticket, 0)
	for rows.Next() {
		var s Ticket
		var status string
		err := rows.Scan(
			&s.ID,
			&s.Number,
			&s.Category,
			&s.Priority,
			&status,
			&s.Title,
			&s.Description,
			&s.Employee.ID,
			&s.Employee.DisplayName,
			&s.Employee.Position,
			&s.Employee.Department,
			&s.Employee.Branch,
			&s.Supporter.DisplayName,
			&s.Supporter.Position,
			&s.CreatedAt,
			&s.ClosedDate,
		)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTicketNotFound
		}

		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		s.Status = mapTicketStatus(status)
		tickets = append(tickets, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}

	return tickets, nil
}

type CategoryReport struct {
	Name       string `json:"name"`
	InProgress int64  `json:"inProgress"`
	Resolved   int64  `json:"resolved"`
	Blank      int64  `json:"blank"`
	Total      int64  `json:"total"`
}

type SupporterReport struct {
	Name       string `json:"name"`
	InProgress int64  `json:"inProgress"`
	Resolved   int64  `json:"resolved"`
	Blank      int64  `json:"blank"`
	Total      int64  `json:"total"`
}

type PriorityReport struct {
	Name   string `json:"name"`
	High   int64  `json:"high"`
	Medium int64  `json:"medium"`
	Low    int64  `json:"low"`
	Blank  int64  `json:"blank"`
	Total  int64  `json:"total"`
}

type ReportQuery struct {
	CreatedBefore time.Time `json:"createdBefore" query:"createdBefore"`
	CreatedAfter  time.Time `json:"createdAfter" query:"createdAfter"`
}

func (q *ReportQuery) ToSql() (string, []any, error) {
	and := sq.And{}
	if !q.CreatedBefore.IsZero() {
		and = append(and, sq.LtOrEq{"created_at": q.CreatedBefore})
	}
	if !q.CreatedAfter.IsZero() {
		and = append(and, sq.GtOrEq{"created_at": q.CreatedAfter})
	}

	return and.ToSql()
}

func listPriorityReports(ctx context.Context, db *sql.DB, in *ReportQuery) ([]*PriorityReport, error) {
	pred, args, err := in.ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to convert to sql: %w", err)
	}

	q, args := sq.
		Select(
			`category`,
			`SUM(CASE WHEN priority = 'HIGHT' THEN 1 ELSE 0 END) AS high`,
			`SUM(CASE WHEN priority = 'MEDIUEM' THEN 1 ELSE 0 END) AS medium`,
			`SUM(CASE WHEN priority = 'LOW' THEN 1 ELSE 0 END) AS low`,
			`SUM(CASE WHEN priority NOT IN ('HIGHT', 'MEDIUEM', 'LOW') THEN 1 ELSE 0 END) AS other`,
			`COUNT(*) AS total`,
		).
		Prefix(`
		WITH priority_report AS (
			SELECT
				category,
				priority,
				created_at
			FROM v_hepldesk_ticket_report
		)`).
		From("priority_report").
		PlaceholderFormat(sq.AtP).
		GroupBy("category").
		OrderBy("category ASC").
		Where(pred, args...).
		MustSql()

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	reports := make([]*PriorityReport, 0)
	for rows.Next() {
		var s PriorityReport
		err := rows.Scan(
			&s.Name,
			&s.High,
			&s.Medium,
			&s.Low,
			&s.Blank,
			&s.Total,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if s.Name == "" {
			s.Name = "(Blank)"
		}
		reports = append(reports, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}

	return reports, nil
}

func listCategoryReports(ctx context.Context, db *sql.DB, in *ReportQuery) ([]*CategoryReport, error) {
	pred, args, err := in.ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to convert to sql: %w", err)
	}

	q, args := sq.Select(
		`category`,
		`SUM(CASE WHEN status = 'FINISHED,MANAGER(APPROVE),IT(IN PROGRESS)' THEN 1 ELSE 0 END) AS in_progress`,
		`SUM(CASE WHEN status = 'FINISHED,MANAGER(APPROVE),IT(RESOLVE)' THEN 1 ELSE 0 END) AS resolved`,
		`SUM(CASE WHEN status NOT IN ('FINISHED,MANAGER(APPROVE),IT(IN PROGRESS)', 'FINISHED,MANAGER(APPROVE),IT(RESOLVE)') THEN 1 ELSE 0 END) AS other`,
		`COUNT(*) AS total`,
	).
		Prefix(`
			WITH category_report AS (
				SELECT
					category,
					status,
					created_at
				FROM v_hepldesk_ticket_report
			)
		`).
		From(`category_report`).
		PlaceholderFormat(sq.AtP).
		GroupBy("category").
		OrderBy("category ASC").
		Where(pred, args...).
		MustSql()

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	reports := make([]*CategoryReport, 0)
	for rows.Next() {
		var s CategoryReport
		err := rows.Scan(
			&s.Name,
			&s.InProgress,
			&s.Resolved,
			&s.Blank,
			&s.Total,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if s.Name == "" {
			s.Name = "(Blank)"
		}
		reports = append(reports, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}

	return reports, nil
}

func listSupporterReports(ctx context.Context, db *sql.DB, in *ReportQuery) ([]*SupporterReport, error) {
	pred, args, err := in.ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to convert to sql: %w", err)
	}

	q, args := sq.
		Select(
			`supporter_name`,
			`SUM(CASE WHEN status = 'FINISHED,MANAGER(APPROVE),IT(IN PROGRESS)' THEN 1 ELSE 0 END) AS in_progress`,
			`SUM(CASE WHEN status = 'FINISHED,MANAGER(APPROVE),IT(RESOLVE)' THEN 1 ELSE 0 END) AS resolved`,
			`SUM(CASE WHEN status NOT IN ('FINISHED,MANAGER(APPROVE),IT(IN PROGRESS)', 'FINISHED,MANAGER(APPROVE),IT(RESOLVE)') THEN 1 ELSE 0 END) AS other`,
			`COUNT(*) AS total`,
		).
		Prefix(`
			WITH supporter_report AS (
				SELECT
					supporter_name,
					status,
					created_at
				FROM v_hepldesk_ticket_report
			)
		`).
		From(`supporter_report`).
		PlaceholderFormat(sq.AtP).
		GroupBy("supporter_name").
		OrderBy("supporter_name ASC").
		Where(pred, args...).
		MustSql()

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	reports := make([]*SupporterReport, 0)
	for rows.Next() {
		var s SupporterReport
		err := rows.Scan(
			&s.Name,
			&s.InProgress,
			&s.Resolved,
			&s.Blank,
			&s.Total,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if s.Name == "" {
			s.Name = "(Blank)"
		}
		reports = append(reports, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}

	return reports, nil
}
