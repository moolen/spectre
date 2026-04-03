package embeddedstore

import "time"

func (qe *QueryExecutor) recordQueryMetrics(queryFamily string, stats queryPlanStats, start time.Time, err error) {
	if qe == nil || qe.metrics == nil {
		return
	}

	qe.metrics.RecordQuery(queryFamily, stats.storeMix(), time.Since(start), err)
	qe.metrics.RecordSegmentScans(queryFamily, stats.scannedSegments)
	qe.metrics.RecordHotScans(queryFamily, stats.hotScans)
	qe.metrics.RecordUIDDiskLookups(queryFamily, stats.uidDiskLookups)
}
