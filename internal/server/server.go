package server

import (
	"errors"
	"net/http"

	"github.com/10664kls/helpdesk-dashboad-api/internal/helpdesk"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	edpb "google.golang.org/genproto/googleapis/rpc/errdetails"
)

type Server struct {
	hdSvc *helpdesk.Service
}

func NewServer(helpdesk *helpdesk.Service) (*Server, error) {
	if helpdesk == nil {
		return nil, errors.New("helpdesk service is nil")
	}

	s := &Server{
		hdSvc: helpdesk,
	}
	return s, nil
}

func (s *Server) Install(e *echo.Echo, mdw ...echo.MiddlewareFunc) error {
	if e == nil {
		return errors.New("echo is nil")
	}

	v1 := e.Group("/v1")

	hd := v1.Group("/helpdesk")
	hd.GET("/tickets", s.listTickets, mdw...)
	hd.GET("/tickets/export-to-excel", s.exportToExcel, mdw...)

	return nil
}

// badJSON is a helper function to create an error when c.Bind return an error.
func badJSON() error {
	s, _ := status.New(codes.InvalidArgument, "Request body must be a valid JSON.").
		WithDetails(&edpb.ErrorInfo{
			Reason: "BINDING_ERROR",
			Domain: "http",
		})
	zap.L().Error("failed to bind json", zap.Error(s.Err()))
	return s.Err()
}

func (s *Server) listTickets(c echo.Context) error {
	req := new(helpdesk.TicketQuery)
	if err := c.Bind(req); err != nil {
		return badJSON()
	}

	ctx := c.Request().Context()
	tickets, err := s.hdSvc.ListTickets(ctx, req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, tickets)
}

func (s *Server) exportToExcel(c echo.Context) error {
	req := new(helpdesk.BatchGetTicketsQuery)
	if err := c.Bind(req); err != nil {
		return badJSON()
	}

	ctx := c.Request().Context()
	buf, err := s.hdSvc.GenExcel(ctx, req)
	if err != nil {
		return err
	}

	c.Response().Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header().Set("Content-Disposition", "attachment; filename=\"help-desk-tickets.xlsx\"")

	return c.Blob(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buf.Bytes())
}
