package twilio

import "fmt"

const (
	StatusDelivered   = "delivered"
	StatusUndelivered = "undelivered"
	StatusFailed      = "failed"
)

// IsFinalStatus reports whether Twilio will send no further receipts for the
// message, so a late, out-of-order receipt must not replace it.
func IsFinalStatus(status string) bool {
	switch status {
	case StatusDelivered, StatusUndelivered, StatusFailed, "read", "canceled":
		return true
	}
	return false
}

func IsFailureStatus(status string) bool {
	return status == StatusUndelivered || status == StatusFailed
}

// Receipts carry only the error code, so the common carrier-side ones are
// spelled out here.
var deliveryErrorDescriptions = map[string]string{
	"30001": "queue overflow",
	"30002": "account suspended",
	"30003": "unreachable destination handset",
	"30004": "message blocked by the recipient or carrier",
	"30005": "unknown destination handset",
	"30006": "landline or unreachable carrier",
	"30007": "message filtered by the carrier",
	"30008": "unknown carrier error",
	"30032": "toll-free number is not verified",
	"30034": "sent from a number not registered to an approved A2P 10DLC campaign",
	"21610": "recipient has unsubscribed (replied STOP)",
}

func DeliveryErrorMessage(status string, errorCode string) string {
	if errorCode == "" || errorCode == "0" {
		return "twilio reported the message " + status
	}
	description, ok := deliveryErrorDescriptions[errorCode]
	if !ok {
		description = "see https://www.twilio.com/docs/api/errors/" + errorCode
	}
	return fmt.Sprintf("twilio reported the message %s: %s: %s", status, errorCode, description)
}
