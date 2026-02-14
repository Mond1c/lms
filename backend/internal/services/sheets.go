package services

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type SheetsService struct {
	srv       *sheets.Service
	sheetID   string
	sheetName string
}

func NewSheetsService(credentialsFile, sheetID string) (*SheetsService, error) {
	ctx := context.Background()

	srv, err := sheets.NewService(ctx, option.WithCredentialsFile(credentialsFile))
	if err != nil {
		return nil, fmt.Errorf("failed to create sheets service: %w", err)
	}

	return &SheetsService{
		srv:       srv,
		sheetID:   sheetID,
		sheetName: "ReviewRequests", // default sheet name
	}, nil
}

func (s *SheetsService) AppendReviewRequest(fullName, repoURL string, requestedAt time.Time) (int, error) {
	ctx := context.Background()

	values := [][]interface{}{
		{
			fullName,
			repoURL,
			requestedAt.Format("2006-01-02 15:04:05"),
			"Ожидает review",
		},
	}

	valueRange := &sheets.ValueRange{
		Values: values,
	}

	resp, err := s.srv.Spreadsheets.Values.Append(
		s.sheetID,
		s.sheetName+"!A:D",
		valueRange,
	).ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Context(ctx).Do()

	if err != nil {
		return 0, fmt.Errorf("failed to append row: %w", err)
	}

	// Parse the updated range to get row number
	// Format: "Sheet1!A5:D5" -> row 5
	rowNum := parseRowNumber(resp.Updates.UpdatedRange)

	return rowNum, nil
}

func (s *SheetsService) UpdateRowStatus(rowIndex int, status string) error {
	ctx := context.Background()

	// Status is in column D (4th column)
	rangeStr := fmt.Sprintf("%s!D%d", s.sheetName, rowIndex)

	values := [][]interface{}{
		{status},
	}

	valueRange := &sheets.ValueRange{
		Values: values,
	}

	_, err := s.srv.Spreadsheets.Values.Update(
		s.sheetID,
		rangeStr,
		valueRange,
	).ValueInputOption("USER_ENTERED").Context(ctx).Do()

	if err != nil {
		return fmt.Errorf("failed to update row status: %w", err)
	}

	return nil
}

func (s *SheetsService) DeleteRow(rowIndex int) error {
	ctx := context.Background()

	// First, get the sheet ID (not the spreadsheet ID)
	spreadsheet, err := s.srv.Spreadsheets.Get(s.sheetID).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to get spreadsheet: %w", err)
	}

	var sheetGID int64
	for _, sheet := range spreadsheet.Sheets {
		if sheet.Properties.Title == s.sheetName {
			sheetGID = sheet.Properties.SheetId
			break
		}
	}

	requests := []*sheets.Request{
		{
			DeleteDimension: &sheets.DeleteDimensionRequest{
				Range: &sheets.DimensionRange{
					SheetId:    sheetGID,
					Dimension:  "ROWS",
					StartIndex: int64(rowIndex - 1), // 0-indexed
					EndIndex:   int64(rowIndex),
				},
			},
		},
	}

	batchUpdate := &sheets.BatchUpdateSpreadsheetRequest{
		Requests: requests,
	}

	_, err = s.srv.Spreadsheets.BatchUpdate(s.sheetID, batchUpdate).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to delete row: %w", err)
	}

	return nil
}

func parseRowNumber(rangeStr string) int {
	// Parse range like "Sheet1!A5:D5" to get row number 5
	var row int
	for i := len(rangeStr) - 1; i >= 0; i-- {
		if rangeStr[i] >= '0' && rangeStr[i] <= '9' {
			continue
		}
		fmt.Sscanf(rangeStr[i+1:], "%d", &row)
		break
	}
	return row
}
