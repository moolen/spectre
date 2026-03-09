package juliandays_test

import (
	"encoding/csv"
	"io"
	"os"
	"strconv"
	"time"
)

type TestData struct {
	DateTime   time.Time
	JulianDays float64
}

func openTestFile(csvFilePath string) ([]TestData, error) {
	// Open test file
	f, err := os.Open(csvFilePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Parse test file
	dataList := []TestData{}
	csvReader := csv.NewReader(f)

	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		dt, err := time.Parse("2006-01-02 15:04:05", record[0])
		if err != nil {
			continue
		}

		jd, err := strconv.ParseFloat(record[1], 64)
		if err != nil {
			continue
		}

		dataList = append(dataList, TestData{
			DateTime:   dt,
			JulianDays: jd,
		})
	}

	return dataList, nil
}
