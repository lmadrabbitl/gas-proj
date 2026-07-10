package main

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type workbook struct {
	Sheets []sheetRef `xml:"sheets>sheet"`
}

type sheetRef struct {
	Name string `xml:"name,attr"`
	Rel  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
}

type rels struct {
	Relations []relation `xml:"Relationship"`
}

type relation struct {
	ID     string `xml:"Id,attr"`
	Target string `xml:"Target,attr"`
	Type   string `xml:"Type,attr"`
}

type stylesXML struct {
	NumFmts struct {
		NumFmts []numFmt `xml:"numFmt"`
	} `xml:"numFmts"`
	CellXfs struct {
		Xfs []cellXf `xml:"xf"`
	} `xml:"cellXfs"`
}

type numFmt struct {
	ID   int    `xml:"numFmtId,attr"`
	Code string `xml:"formatCode,attr"`
}

type cellXf struct {
	NumFmtID int `xml:"numFmtId,attr"`
}

type sharedStrings struct {
	Items []sharedStringItem `xml:"si"`
}

type sharedStringItem struct {
	Text string `xml:"t"`
	Runs []struct {
		Text string `xml:"t"`
	} `xml:"r"`
}

type sheetCell struct {
	Ref     string `xml:"r,attr"`
	Type    string `xml:"t,attr"`
	StyleID int    `xml:"s,attr"`
	Value   string `xml:"v"`
	Inline  struct {
		Text string `xml:"t"`
	} `xml:"is"`
}

func readStatementXLSX(path, sheetName string) ([]StatementRow, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	files := map[string]*zip.File{}
	for i := range r.File {
		files[r.File[i].Name] = r.File[i]
	}

	wbData, err := readZipFile(files, "xl/workbook.xml")
	if err != nil {
		return nil, err
	}
	var wb workbook
	if err := xml.Unmarshal(wbData, &wb); err != nil {
		return nil, err
	}

	relData, err := readZipFile(files, "xl/_rels/workbook.xml.rels")
	if err != nil {
		return nil, err
	}
	var relsDoc rels
	if err := xml.Unmarshal(relData, &relsDoc); err != nil {
		return nil, err
	}
	relMap := map[string]string{}
	for _, rel := range relsDoc.Relations {
		target := rel.Target
		if !strings.HasPrefix(target, "xl/") {
			target = filepath.Clean(filepath.Join("xl", target))
		}
		relMap[rel.ID] = target
	}

	sheetTarget := ""
	if sheetName != "" {
		for _, sheet := range wb.Sheets {
			if strings.EqualFold(sheet.Name, sheetName) {
				sheetTarget = relMap[sheet.Rel]
				break
			}
		}
	}
	if sheetTarget == "" {
		if len(wb.Sheets) == 0 {
			return nil, fmt.Errorf("no sheets found in workbook")
		}
		sheetTarget = relMap[wb.Sheets[0].Rel]
	}
	if sheetTarget == "" {
		return nil, fmt.Errorf("unable to resolve sheet target")
	}

	shared, _ := readSharedStrings(files)
	styleDateMap, _ := readDateStyles(files)

	sheetData, err := readZipFile(files, sheetTarget)
	if err != nil {
		return nil, err
	}

	rows, err := parseSheetRows(sheetData, shared, styleDateMap)
	if err != nil {
		return nil, err
	}
	return parseStatementTable(rows)
}

func readSharedStrings(files map[string]*zip.File) ([]string, error) {
	data, err := readZipFile(files, "xl/sharedStrings.xml")
	if err != nil {
		return nil, nil
	}
	var ss sharedStrings
	if err := xml.Unmarshal(data, &ss); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(ss.Items))
	for _, item := range ss.Items {
		if item.Text != "" {
			out = append(out, strings.TrimSpace(item.Text))
			continue
		}
		var parts []string
		for _, run := range item.Runs {
			if run.Text != "" {
				parts = append(parts, run.Text)
			}
		}
		out = append(out, strings.Join(parts, ""))
	}
	return out, nil
}

func readDateStyles(files map[string]*zip.File) (map[int]bool, error) {
	data, err := readZipFile(files, "xl/styles.xml")
	if err != nil {
		return map[int]bool{}, nil
	}
	var styles stylesXML
	if err := xml.Unmarshal(data, &styles); err != nil {
		return nil, err
	}

	formatCodes := map[int]string{}
	for _, nf := range styles.NumFmts.NumFmts {
		formatCodes[nf.ID] = nf.Code
	}

	out := map[int]bool{}
	for idx, xf := range styles.CellXfs.Xfs {
		if isDateStyle(xf.NumFmtID, formatCodes) {
			out[idx] = true
		}
	}
	return out, nil
}

func isDateStyle(numFmtID int, customFormats map[int]string) bool {
	switch numFmtID {
	case 14, 15, 16, 17, 18, 19, 20, 21, 22, 27, 30, 36, 45, 46, 47, 50, 51, 52, 53, 54, 55, 56, 57, 58:
		return true
	}
	if format, ok := customFormats[numFmtID]; ok {
		format = strings.ToLower(format)
		if strings.Contains(format, "yy") || strings.Contains(format, "dd") || strings.Contains(format, "mm") || strings.Contains(format, "mmm") {
			return true
		}
	}
	return false
}

func parseSheetRows(data []byte, shared []string, dateStyles map[int]bool) ([][]string, error) {
	type row struct {
		Index int         `xml:"r,attr"`
		Cells []sheetCell `xml:"c"`
	}
	type sheet struct {
		Rows []row `xml:"sheetData>row"`
	}

	var s sheet
	if err := xml.Unmarshal(data, &s); err != nil {
		return nil, err
	}

	out := make([][]string, 0, len(s.Rows))
	for _, r := range s.Rows {
		if r.Index < 15 || r.Index > 717 {
			continue
		}
		rowVals := []string{}
		for _, cell := range r.Cells {
			col := excelColumnIndex(cell.Ref)
			for len(rowVals) < col {
				rowVals = append(rowVals, "")
			}
			rowVals[col-1] = excelCellValue(cell, shared, dateStyles)
		}
		out = append(out, trimTrailingEmpty(rowVals))
	}
	return out, nil
}

func excelCellValue(cell sheetCell, shared []string, dateStyles map[int]bool) string {
	switch cell.Type {
	case "s":
		idx, err := strconv.Atoi(strings.TrimSpace(cell.Value))
		if err != nil || idx < 0 || idx >= len(shared) {
			return ""
		}
		return shared[idx]
	case "inlineStr":
		if cell.Inline.Text != "" {
			return cell.Inline.Text
		}
		return cell.Value
	case "d":
		if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(cell.Value)); err == nil {
			return parsed.Format("02/01/2006")
		}
	}

	if cell.Value == "" {
		return ""
	}

	if dateStyles != nil && dateStyles[cell.StyleID] {
		f, err := strconv.ParseFloat(cell.Value, 64)
		if err != nil {
			return cell.Value
		}
		return excelSerialToDate(f).Format("02/01/2006")
	}
	return cell.Value
}

func readZipFile(files map[string]*zip.File, name string) ([]byte, error) {
	f, ok := files[name]
	if !ok {
		return nil, fmt.Errorf("%s not found in xlsx", name)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func excelColumnIndex(ref string) int {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return 1
	}
	letters := ""
	for _, r := range ref {
		if r < 'A' || r > 'Z' {
			if r >= 'a' && r <= 'z' {
				letters += strings.ToUpper(string(r))
				continue
			}
			break
		}
		letters += string(r)
	}
	if letters == "" {
		return 1
	}
	n := 0
	for _, ch := range letters {
		n = n*26 + int(ch-'A'+1)
	}
	return n
}

func trimTrailingEmpty(values []string) []string {
	i := len(values)
	for i > 0 && strings.TrimSpace(values[i-1]) == "" {
		i--
	}
	return values[:i]
}

func excelSerialToDate(serial float64) time.Time {
	base := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	whole := int64(serial)
	fraction := serial - float64(whole)
	days := time.Duration(whole) * 24 * time.Hour
	secs := time.Duration(fraction * float64(24*time.Hour))
	return base.Add(days + secs)
}
