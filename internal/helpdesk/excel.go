package helpdesk

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/xuri/excelize/v2"
	"go.uber.org/zap"
)

func (s *Service) GenExcel(ctx context.Context, in *BatchGetTicketsQuery) (*bytes.Buffer, error) {
	zlog := s.zlog.With(
		zap.String("method", "GenExcel"),
		zap.Any("query", in),
	)

	zlog.Info("starting to gen excel")

	categoryReports, err := listCategoryReports(ctx, s.db, &ReportQuery{
		CreatedBefore: in.CreatedBefore,
		CreatedAfter:  in.CreatedAfter,
	})
	if err != nil {
		zlog.Error("failed to list category reports", zap.Error(err))
		return nil, err
	}

	supporterReports, err := listSupporterReports(ctx, s.db, &ReportQuery{
		CreatedBefore: in.CreatedBefore,
		CreatedAfter:  in.CreatedAfter,
	})
	if err != nil {
		zlog.Error("failed to list supporter reports", zap.Error(err))
		return nil, err
	}

	priorityReports, err := listPriorityReports(ctx, s.db, &ReportQuery{
		CreatedBefore: in.CreatedBefore,
		CreatedAfter:  in.CreatedAfter,
	})
	if err != nil {
		zlog.Error("failed to list priority reports", zap.Error(err))
		return nil, err
	}

	fx := excelize.NewFile()
	defer fx.Close()

	styleHeader, err := fx.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Size: 14,
			Bold: true,
		},
		Fill: excelize.Fill{
			Type: "pattern",
			Color: []string{
				"E0EBF5",
			},
			Pattern: 1,
		},
	})
	if err != nil {
		zlog.Error("failed to create style", zap.Error(err))
		return nil, err
	}

	const sheetTicket = "Help Desk Requests"
	fx.SetSheetName("Sheet1", sheetTicket)

	// add header
	fx.SetCellValue(sheetTicket, "A1", "HelpDesk Number")
	fx.SetCellValue(sheetTicket, "B1", "Type of Form")
	fx.SetCellValue(sheetTicket, "C1", "Title")
	fx.SetCellValue(sheetTicket, "D1", "Description")
	fx.SetCellValue(sheetTicket, "E1", "Request Date")
	fx.SetCellValue(sheetTicket, "F1", "ID Staff Request")
	fx.SetCellValue(sheetTicket, "G1", "Request by (Eng)")
	fx.SetCellValue(sheetTicket, "H1", "Position")
	fx.SetCellValue(sheetTicket, "I1", "Department")
	fx.SetCellValue(sheetTicket, "J1", "Branch")
	fx.SetCellValue(sheetTicket, "K1", "IT Support Name")
	fx.SetCellValue(sheetTicket, "L1", "IT Support Position")
	fx.SetCellValue(sheetTicket, "M1", "Priority")
	fx.SetCellValue(sheetTicket, "N1", "Status")
	fx.SetCellValue(sheetTicket, "O1", "Closed Date")
	fx.SetRowStyle(sheetTicket, 1, 1, styleHeader)

	// Summary sheet
	const sheetSummary = "Summary"
	if _, err := fx.NewSheet(sheetSummary); err != nil {
		zlog.Error("failed to create sheet summary", zap.Error(err))
		return nil, err
	}

	var from, to string
	to = time.Now().Format("02/01/2006")
	from = in.CreatedAfter.Format("02/01/2006")
	if !in.CreatedBefore.IsZero() {
		to = in.CreatedBefore.Format("02/01/2006")
	}
	fx.SetCellValue(sheetSummary, "A1", fmt.Sprintf(`Date update: %s-%s`, from, to))
	fx.MergeCell(sheetSummary, "A1", "D1")
	fx.SetRowStyle(sheetSummary, 1, 1, styleHeader)

	var wg sync.WaitGroup
	const startCategoryReportRow = 4
	fx.SetCellValue(sheetSummary, fmt.Sprintf("A%d", startCategoryReportRow), "Helpdesk Ticket Summary Report Type")
	fx.SetCellValue(sheetSummary, fmt.Sprintf("B%d", startCategoryReportRow), "In Progress")
	fx.SetCellValue(sheetSummary, fmt.Sprintf("C%d", startCategoryReportRow), "Resolved")
	fx.SetCellValue(sheetSummary, fmt.Sprintf("D%d", startCategoryReportRow), "Blank")
	fx.SetCellValue(sheetSummary, fmt.Sprintf("E%d", startCategoryReportRow), "Grand Total")
	fx.SetRowStyle(sheetSummary, startCategoryReportRow, startCategoryReportRow, styleHeader)
	wg.Add(1)
	go genCategoryReportToExcel(fx, &wg, sheetSummary, startCategoryReportRow, categoryReports)

	startSupporterReportRow := 10 + startCategoryReportRow + len(categoryReports)
	fx.SetCellValue(sheetSummary, fmt.Sprintf("A%d", startSupporterReportRow), "IT Technical Summary Report Full Name")
	fx.SetCellValue(sheetSummary, fmt.Sprintf("B%d", startSupporterReportRow), "In Progress")
	fx.SetCellValue(sheetSummary, fmt.Sprintf("C%d", startSupporterReportRow), "Resolved")
	fx.SetCellValue(sheetSummary, fmt.Sprintf("D%d", startSupporterReportRow), "Blank")
	fx.SetCellValue(sheetSummary, fmt.Sprintf("E%d", startSupporterReportRow), "Grand Total")
	fx.SetRowStyle(sheetSummary, startSupporterReportRow, startSupporterReportRow, styleHeader)
	wg.Add(1)
	go genSupporterReportToExcel(fx, &wg, sheetSummary, startSupporterReportRow, supporterReports)

	startPriorityReportRow := 10 + startSupporterReportRow + len(supporterReports)
	fx.SetCellValue(sheetSummary, fmt.Sprintf("A%d", startPriorityReportRow), "Priority Summary Report Type")
	fx.SetCellValue(sheetSummary, fmt.Sprintf("B%d", startPriorityReportRow), "High")
	fx.SetCellValue(sheetSummary, fmt.Sprintf("C%d", startPriorityReportRow), "Medium")
	fx.SetCellValue(sheetSummary, fmt.Sprintf("D%d", startPriorityReportRow), "Low")
	fx.SetCellValue(sheetSummary, fmt.Sprintf("E%d", startPriorityReportRow), "Blank")
	fx.SetCellValue(sheetSummary, fmt.Sprintf("F%d", startPriorityReportRow), "Grand Total")
	fx.SetRowStyle(sheetSummary, startPriorityReportRow, startPriorityReportRow, styleHeader)
	wg.Add(1)
	go genPriorityReportToExcel(fx, &wg, sheetSummary, startPriorityReportRow, priorityReports)

	startTicketsRow := 2
	var nextID string
	for {
		tickets, err := batchGetTickets(ctx, s.db, 200, nextID, in)
		if err != nil {
			zlog.Error("failed to batch get statements", zap.Error(err))
			return nil, err
		}

		if len(tickets) == 0 {
			break
		}

		s.mu.Lock()
		nextID = tickets[len(tickets)-1].ID
		s.mu.Unlock()

		wg.Add(1)
		go genTicketsToExcel(fx, &wg, sheetTicket, startTicketsRow, tickets)

		startTicketsRow += len(tickets)
	}

	wg.Wait()

	buf, err := fx.WriteToBuffer()
	if err != nil {
		zlog.Error("failed to write file to buffer", zap.Error(err))
		return nil, err
	}

	return buf, nil
}

func genSupporterReportToExcel(fx *excelize.File, wg *sync.WaitGroup, sheetName string, startRow int, supporters []*SupporterReport) {
	defer wg.Done()
	for i, r := range supporters {
		fx.SetCellValue(sheetName, fmt.Sprintf("A%d", startRow+i+1), r.Name)
		fx.SetCellValue(sheetName, fmt.Sprintf("B%d", startRow+i+1), r.InProgress)
		fx.SetCellValue(sheetName, fmt.Sprintf("C%d", startRow+i+1), r.Resolved)
		fx.SetCellValue(sheetName, fmt.Sprintf("D%d", startRow+i+1), r.Blank)
		fx.SetCellValue(sheetName, fmt.Sprintf("E%d", startRow+i+1), r.Total)
	}
}

func genCategoryReportToExcel(fx *excelize.File, wg *sync.WaitGroup, sheetName string, startRow int, categories []*CategoryReport) {
	defer wg.Done()
	for i, r := range categories {
		fx.SetCellValue(sheetName, fmt.Sprintf("A%d", startRow+i+1), r.Name)
		fx.SetCellValue(sheetName, fmt.Sprintf("B%d", startRow+i+1), r.InProgress)
		fx.SetCellValue(sheetName, fmt.Sprintf("C%d", startRow+i+1), r.Resolved)
		fx.SetCellValue(sheetName, fmt.Sprintf("D%d", startRow+i+1), r.Blank)
		fx.SetCellValue(sheetName, fmt.Sprintf("E%d", startRow+i+1), r.Total)
	}
}

func genPriorityReportToExcel(fx *excelize.File, wg *sync.WaitGroup, sheetName string, startRow int, priorities []*PriorityReport) {
	defer wg.Done()
	for i, r := range priorities {
		fx.SetCellValue(sheetName, fmt.Sprintf("A%d", startRow+i+1), r.Name)
		fx.SetCellValue(sheetName, fmt.Sprintf("B%d", startRow+i+1), r.High)
		fx.SetCellValue(sheetName, fmt.Sprintf("C%d", startRow+i+1), r.Medium)
		fx.SetCellValue(sheetName, fmt.Sprintf("D%d", startRow+i+1), r.Low)
		fx.SetCellValue(sheetName, fmt.Sprintf("E%d", startRow+i+1), r.Blank)
		fx.SetCellValue(sheetName, fmt.Sprintf("F%d", startRow+i+1), r.Total)
	}
}

func genTicketsToExcel(fx *excelize.File, wg *sync.WaitGroup, sheetName string, startRow int, tickets []*Ticket) {
	defer wg.Done()
	for i, s := range tickets {
		var closedDate string
		if s.ClosedDate.Format("2006-01-02") != "1900-01-01" {
			closedDate = s.ClosedDate.Format("02/01/2006")
		}
		fx.SetCellValue(sheetName, fmt.Sprintf("A%d", startRow+i), s.Number)
		fx.SetCellValue(sheetName, fmt.Sprintf("B%d", startRow+i), s.Category)
		fx.SetCellValue(sheetName, fmt.Sprintf("C%d", startRow+i), s.Title)
		fx.SetCellValue(sheetName, fmt.Sprintf("D%d", startRow+i), s.Description)
		fx.SetCellValue(sheetName, fmt.Sprintf("E%d", startRow+i), s.CreatedAt.Format("02/01/2006 15:04:05"))
		fx.SetCellValue(sheetName, fmt.Sprintf("F%d", startRow+i), s.Employee.ID)
		fx.SetCellValue(sheetName, fmt.Sprintf("G%d", startRow+i), s.Employee.DisplayName)
		fx.SetCellValue(sheetName, fmt.Sprintf("H%d", startRow+i), s.Employee.Position)
		fx.SetCellValue(sheetName, fmt.Sprintf("I%d", startRow+i), s.Employee.Department)
		fx.SetCellValue(sheetName, fmt.Sprintf("J%d", startRow+i), s.Employee.Branch)
		fx.SetCellValue(sheetName, fmt.Sprintf("K%d", startRow+i), s.Supporter.DisplayName)
		fx.SetCellValue(sheetName, fmt.Sprintf("L%d", startRow+i), s.Supporter.Position)
		fx.SetCellValue(sheetName, fmt.Sprintf("M%d", startRow+i), s.Priority)
		fx.SetCellValue(sheetName, fmt.Sprintf("N%d", startRow+i), s.Status)
		fx.SetCellValue(sheetName, fmt.Sprintf("O%d", startRow+i), closedDate)
	}
}
