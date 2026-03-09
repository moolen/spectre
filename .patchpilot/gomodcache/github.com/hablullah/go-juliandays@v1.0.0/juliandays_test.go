package juliandays_test

import (
	"math"
	"testing"
	"time"

	"github.com/hablullah/go-juliandays"
)

var testData []TestData

func init() {
	var err error
	testData, err = openTestFile("test-data.csv")
	if err != nil {
		panic(err)
	}
}

func Test_JD_FromTime(t *testing.T) {
	if len(testData) == 0 {
		t.Fatal("no tests available")
	}

	for _, data := range testData {
		jd, _ := juliandays.FromTime(data.DateTime)
		if math.Round((data.JulianDays-jd)*100_000) != 0 {
			t.Errorf("%s: want %f got %f\n",
				data.DateTime.Format("2006-01-02 15:04:05"),
				data.JulianDays, jd)
		}
	}
}

func Test_JD_ToTime(t *testing.T) {
	if len(testData) == 0 {
		t.Fatal("no tests available")
	}

	for _, data := range testData {
		if data.JulianDays == 0 {
			continue
		}

		dt := juliandays.ToTime(data.JulianDays)
		diff := data.DateTime.Sub(dt).Seconds()
		if math.Abs(diff) > 1 {
			t.Errorf("%f: want %s got %s (%v)\n",
				data.JulianDays,
				data.DateTime.Format("2006-01-02 15:04:05"),
				dt.Format("2006-01-02 15:04:05"),
				data.DateTime.Sub(dt))
		}
	}
}

func Test_JD_Bidirectional(t *testing.T) {
	dt := time.Date(-4712, 1, 1, 12, 0, 0, 0, time.UTC)
	for dt.Year() <= 2120 {
		// Convert date time to Julian Days
		jd, err := juliandays.FromTime(dt)
		if err != nil {
			dt = dt.AddDate(0, 0, 1)
			continue
		}

		// Convert back Julian Days to date time
		newDt := juliandays.ToTime(jd)

		// Compare original and new date time
		diff := dt.Sub(newDt).Seconds()
		if math.Abs(diff) > 1 {
			t.Errorf("Original %s: JD %f, reverted into %s (%v)\n",
				dt.Format("2006-01-02 15:04:05"), jd,
				newDt.Format("2006-01-02 15:04:05"), diff)
		}

		// Increase date
		dt = dt.AddDate(0, 0, 1)
	}
}
