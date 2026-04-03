package anomaly

import (
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/analysis"
)

// isCertManagerCertificate checks if a Certificate node is from cert-manager
func (d *AnomalyDetector) isCertManagerCertificate(certNode *analysis.GraphNode) bool {
	certData := latestSnapshotData(certNode)
	if certData == nil {
		return false
	}

	apiVersion, ok := certData["apiVersion"].(string)
	if !ok {
		return false
	}

	switch apiVersion {
	case "cert-manager.io/v1", "cert-manager.io/v1alpha2", "cert-manager.io/v1alpha3", "cert-manager.io/v1beta1":
		return true
	default:
		return false
	}
}

// detectCertificateExpiredAnomalies checks if a cert-manager Certificate has expired
func (d *AnomalyDetector) detectCertificateExpiredAnomalies(
	certNode *analysis.GraphNode,
	timeWindow TimeWindow,
) []Anomaly {
	var anomalies []Anomaly

	certData := latestSnapshotData(certNode)
	if certData == nil {
		return anomalies
	}

	status, ok := certData["status"].(map[string]interface{})
	if !ok {
		return anomalies
	}

	notAfterStr, ok := status["notAfter"].(string)
	if !ok {
		return anomalies
	}

	notAfter, err := time.Parse(time.RFC3339, notAfterStr)
	if err != nil {
		d.logger.Debug("Failed to parse Certificate notAfter time: %v", err)
		return anomalies
	}

	now := time.Now()
	if notAfter.Before(now) {
		anomalies = append(anomalies, Anomaly{
			Node:      NodeFromGraphNode(certNode),
			Category:  CategoryState,
			Type:      "CertExpired",
			Severity:  SeverityCritical,
			Timestamp: timeWindow.End,
			Summary:   fmt.Sprintf("Certificate expired on %s", notAfter.Format(time.RFC3339)),
			Details: map[string]interface{}{
				"not_after":   notAfterStr,
				"expired_for": now.Sub(notAfter).String(),
			},
		})
	}

	return anomalies
}
