package googlecloud

import "testing"

func TestIsGoogleQuotaExceededResponse_DetectsQuotaFailure(t *testing.T) {
	raw := []byte(`{
		"error": {
			"code": 429,
			"message": "You exceeded your current quota",
			"status": "RESOURCE_EXHAUSTED",
			"details": [
				{
					"@type": "type.googleapis.com/google.rpc.QuotaFailure",
					"violations": [
						{
							"quotaMetric": "generativelanguage.googleapis.com/generate_requests_per_model_per_day",
							"quotaId": "GenerateRequestsPerDayPerProjectPerModel"
						}
					]
				}
			]
		}
	}`)
	if !isGoogleQuotaExceededResponse(raw) {
		t.Fatalf("expected quota failure response to be detected")
	}
}

func TestIsGoogleQuotaExceededResponse_IgnoresGeneric429(t *testing.T) {
	raw := []byte(`{
		"error": {
			"code": 429,
			"message": "Too many requests",
			"status": "RESOURCE_EXHAUSTED",
			"details": [
				{
					"@type": "type.googleapis.com/google.rpc.RetryInfo",
					"retryDelay": "10s"
				}
			]
		}
	}`)
	if isGoogleQuotaExceededResponse(raw) {
		t.Fatalf("expected generic 429 response not to be treated as quota exhaustion")
	}
}
