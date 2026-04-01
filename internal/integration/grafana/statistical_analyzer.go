package grafana

import (
	"math"

	"github.com/moolen/spectre/internal/graph"
	"gonum.org/v1/gonum/stat"
	"gonum.org/v1/gonum/stat/distuv"
)

// CorrelationResult holds the results of statistical analysis
type CorrelationResult struct {
	// Did we find significant correlation?
	IsSignificant bool

	// Individual method results
	TTest      TTestResult
	EffectSize EffectSizeResult
	Threshold  ThresholdResult

	// Raw statistics
	MeanBefore    float64
	MeanAfter     float64
	StddevBefore  float64
	StddevAfter   float64
	SamplesBefore int
	SamplesAfter  int
}

// TTestResult holds Welch's t-test results
type TTestResult struct {
	TStatistic  float64
	PValue      float64
	Significant bool // p < threshold
}

// EffectSizeResult holds Cohen's d effect size results
type EffectSizeResult struct {
	CohensD     float64
	Significant bool // |d| > threshold
}

// ThresholdResult holds simple threshold check results
type ThresholdResult struct {
	SigmaChange float64 // How many stddevs the mean shifted
	Exceeded    bool    // |mean_after - mean_before| > threshold * stddev_before
}

// StatisticalAnalyzer computes various statistical measures to detect
// significant changes between before/after metric windows.
type StatisticalAnalyzer struct {
	pValueThreshold  float64
	cohensDThreshold float64
	sigmaThreshold   float64
}

// NewStatisticalAnalyzer creates a new StatisticalAnalyzer with configurable thresholds.
func NewStatisticalAnalyzer(pValueThreshold, cohensDThreshold, sigmaThreshold float64) *StatisticalAnalyzer {
	return &StatisticalAnalyzer{
		pValueThreshold:  pValueThreshold,
		cohensDThreshold: cohensDThreshold,
		sigmaThreshold:   sigmaThreshold,
	}
}

// Analyze compares before and after windows using multiple statistical methods.
// IsSignificant is true if ANY method indicates significance.
func (a *StatisticalAnalyzer) Analyze(before, after *MetricWindow) CorrelationResult {
	result := CorrelationResult{
		SamplesBefore: len(before.Values),
		SamplesAfter:  len(after.Values),
	}

	// Calculate basic statistics
	result.MeanBefore = stat.Mean(before.Values, nil)
	result.MeanAfter = stat.Mean(after.Values, nil)
	result.StddevBefore = stat.StdDev(before.Values, nil)
	result.StddevAfter = stat.StdDev(after.Values, nil)

	// Perform each analysis
	result.TTest = a.welchTTest(before.Values, after.Values)
	result.EffectSize = a.cohensD(before.Values, after.Values)
	result.Threshold = a.thresholdCheck(before.Values, after.Values)

	// Significant if ANY method indicates it
	result.IsSignificant = result.TTest.Significant ||
		result.EffectSize.Significant ||
		result.Threshold.Exceeded

	return result
}

// welchTTest performs Welch's t-test for unequal variances.
// This test is robust when the two samples have different sizes and variances.
func (a *StatisticalAnalyzer) welchTTest(before, after []float64) TTestResult {
	result := TTestResult{}

	n1 := float64(len(before))
	n2 := float64(len(after))

	if n1 < 2 || n2 < 2 {
		return result
	}

	mean1 := stat.Mean(before, nil)
	mean2 := stat.Mean(after, nil)
	var1 := stat.Variance(before, nil)
	var2 := stat.Variance(after, nil)

	// Welch's t-statistic
	// t = (mean1 - mean2) / sqrt(var1/n1 + var2/n2)
	denominator := math.Sqrt(var1/n1 + var2/n2)
	if denominator == 0 {
		return result
	}

	result.TStatistic = (mean1 - mean2) / denominator

	// Welch-Satterthwaite degrees of freedom
	// df = (var1/n1 + var2/n2)^2 / ((var1/n1)^2/(n1-1) + (var2/n2)^2/(n2-1))
	v1n1 := var1 / n1
	v2n2 := var2 / n2
	numerator := (v1n1 + v2n2) * (v1n1 + v2n2)
	dfDenom := (v1n1*v1n1)/(n1-1) + (v2n2*v2n2)/(n2-1)

	if dfDenom == 0 {
		return result
	}

	df := numerator / dfDenom

	// Calculate two-tailed p-value using t-distribution
	tDist := distuv.StudentsT{Mu: 0, Sigma: 1, Nu: df}
	result.PValue = 2 * tDist.CDF(-math.Abs(result.TStatistic))

	result.Significant = result.PValue < a.pValueThreshold

	return result
}

// cohensD calculates Cohen's d effect size.
// This measures the standardized difference between two means.
// |d| > 0.8 is typically considered a large effect.
func (a *StatisticalAnalyzer) cohensD(before, after []float64) EffectSizeResult {
	result := EffectSizeResult{}

	if len(before) < 2 || len(after) < 2 {
		return result
	}

	mean1 := stat.Mean(before, nil)
	mean2 := stat.Mean(after, nil)

	// Pooled standard deviation
	pooledSD := pooledStddev(before, after)
	if pooledSD == 0 {
		return result
	}

	result.CohensD = (mean2 - mean1) / pooledSD
	result.Significant = math.Abs(result.CohensD) > a.cohensDThreshold

	return result
}

// thresholdCheck performs simple threshold check (mean shift > n*sigma).
// This is a simpler approach that doesn't require distribution assumptions.
func (a *StatisticalAnalyzer) thresholdCheck(before, after []float64) ThresholdResult {
	result := ThresholdResult{}

	if len(before) < 2 {
		return result
	}

	mean1 := stat.Mean(before, nil)
	mean2 := stat.Mean(after, nil)
	stddev1 := stat.StdDev(before, nil)

	if stddev1 == 0 {
		// If stddev is 0, any change is significant
		if mean1 != mean2 {
			result.SigmaChange = math.Inf(1)
			result.Exceeded = true
		}
		return result
	}

	result.SigmaChange = math.Abs(mean2-mean1) / stddev1
	result.Exceeded = result.SigmaChange > a.sigmaThreshold

	return result
}

// pooledStddev calculates the pooled standard deviation of two samples.
// This is used for Cohen's d calculation.
func pooledStddev(s1, s2 []float64) float64 {
	n1 := float64(len(s1))
	n2 := float64(len(s2))

	if n1 < 2 || n2 < 2 {
		return 0
	}

	var1 := stat.Variance(s1, nil)
	var2 := stat.Variance(s2, nil)

	// Pooled variance = ((n1-1)*var1 + (n2-1)*var2) / (n1 + n2 - 2)
	pooledVar := ((n1-1)*var1 + (n2-1)*var2) / (n1 + n2 - 2)

	return math.Sqrt(pooledVar)
}

// ToGraphStats converts CorrelationResult to graph.SignalCorrelationStats for graph storage.
func (r *CorrelationResult) ToGraphStats() graph.SignalCorrelationStats {
	return graph.SignalCorrelationStats{
		TStatistic:        r.TTest.TStatistic,
		PValue:            r.TTest.PValue,
		CohensD:           r.EffectSize.CohensD,
		ThresholdExceeded: r.Threshold.Exceeded,
		MeanBefore:        r.MeanBefore,
		MeanAfter:         r.MeanAfter,
		StddevBefore:      r.StddevBefore,
		StddevAfter:       r.StddevAfter,
		SamplesBefore:     r.SamplesBefore,
		SamplesAfter:      r.SamplesAfter,
	}
}
