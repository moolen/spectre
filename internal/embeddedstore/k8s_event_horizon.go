package embeddedstore

import "time"

// DefaultK8sEventSummaryHorizon is how long core/v1 Event summaries stay in memory. Older
// ones are read from the segments' associated-UID index on demand: over a 30-day window
// the summaries would otherwise be the largest part of the heap.
const DefaultK8sEventSummaryHorizon = 48 * time.Hour

// SetK8sEventInfoHorizon bounds how far back core/v1 Event summaries are kept in memory
// (0 keeps them for the whole retention window) and trims the current ones.
func (p *Projection) SetK8sEventInfoHorizon(horizon time.Duration) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.k8sEventInfoHorizonNs = int64(max(horizon, 0))
	p.trimK8sEventInfosToHorizonLocked(p.maxTimestampNs, true)
}

// trimK8sEventInfosToHorizonLocked drops event summaries older than the horizon. Sweeping
// visits every involved UID, so unless forced it only runs once the cutoff has moved by
// an eighth of the horizon; summaries are therefore kept for up to 9/8 of it.
func (p *Projection) trimK8sEventInfosToHorizonLocked(nowNs int64, force bool) {
	horizon := p.k8sEventInfoHorizonNs
	if horizon <= 0 || nowNs <= 0 {
		return
	}
	cutoff := nowNs - horizon
	if cutoff <= p.k8sEventInfosTrimmedThrough+1 {
		return
	}
	if !force && cutoff <= p.k8sEventInfosTrimmedThrough+horizon/8 {
		return
	}
	trimK8sEventInfosBefore(p.k8sEventsByInvolvedUID, cutoff)
	p.k8sEventInfosTrimmedThrough = cutoff - 1
	for uid, truncated := range p.k8sEventsTruncatedThrough {
		if truncated <= p.k8sEventInfosTrimmedThrough {
			delete(p.k8sEventsTruncatedThrough, uid)
		}
	}
}
