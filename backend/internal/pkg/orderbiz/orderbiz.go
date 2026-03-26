package orderbiz

import (
	"auralogic/internal/models"
	"auralogic/internal/pkg/bizerr"
)

func InvalidOrderID() *bizerr.Error {
	return bizerr.New("order.invalidOrderID", "Invalid order ID format")
}

func InvalidRequestParameters() *bizerr.Error {
	return bizerr.New("order.invalidRequestParameters", "Invalid request parameters")
}

func TrackingNumberLengthInvalid(min, max int) *bizerr.Error {
	return bizerr.Newf("order.trackingNumberLengthInvalid", "Tracking number length must be between %d-%d characters", min, max).
		WithParams(map[string]interface{}{"min": min, "max": max})
}

func AdminRemarkTooLong(max int) *bizerr.Error {
	return bizerr.Newf("order.adminRemarkTooLong", "Admin remark length cannot exceed %d characters", max).
		WithParams(map[string]interface{}{"max": max})
}

func CancellationReasonTooLong(max int) *bizerr.Error {
	return bizerr.Newf("order.cancellationReasonTooLong", "Cancellation reason length cannot exceed %d characters", max).
		WithParams(map[string]interface{}{"max": max})
}

func RefundReasonTooLong(max int) *bizerr.Error {
	return bizerr.Newf("order.refundReasonTooLong", "Refund reason length cannot exceed %d characters", max).
		WithParams(map[string]interface{}{"max": max})
}

func RefundStatusInvalid(status models.OrderStatus) *bizerr.Error {
	return bizerr.Newf("order.refundStatusInvalid", "Current order status does not support refund (current status: %s)", status).
		WithParams(map[string]interface{}{"status": status})
}

func OrderPaymentMethodNotFound() *bizerr.Error {
	return bizerr.New("order.orderPaymentMethodNotFound", "Order payment method not found")
}

func PaymentMethodNotFound() *bizerr.Error {
	return bizerr.New("order.paymentMethodNotFound", "Payment method not found")
}

func ShippingInfoStatusInvalid(status models.OrderStatus) *bizerr.Error {
	return bizerr.Newf("order.shippingInfoStatusInvalid", "Order status does not allow shipping information modification (current status: %s)", status).
		WithParams(map[string]interface{}{"status": status})
}

func ReceiverNameLengthInvalid(min, max int) *bizerr.Error {
	return bizerr.Newf("order.receiverNameLengthInvalid", "Receiver name length must be between %d-%d characters", min, max).
		WithParams(map[string]interface{}{"min": min, "max": max})
}

func PhoneCodeInvalid() *bizerr.Error {
	return bizerr.New("order.phoneCodeInvalid", "Invalid phone code format")
}

func ReceiverPhoneInvalid() *bizerr.Error {
	return bizerr.New("order.receiverPhoneInvalid", "Invalid phone number format or length")
}

func EmailTooLong(max int) *bizerr.Error {
	return bizerr.Newf("order.emailTooLong", "Email length cannot exceed %d characters", max).
		WithParams(map[string]interface{}{"max": max})
}

func CountryCodeInvalid() *bizerr.Error {
	return bizerr.New("order.countryCodeInvalid", "Invalid country code format")
}

func ProvinceTooLong(max int) *bizerr.Error {
	return bizerr.Newf("order.provinceTooLong", "Province length cannot exceed %d characters", max).
		WithParams(map[string]interface{}{"max": max})
}

func CityTooLong(max int) *bizerr.Error {
	return bizerr.Newf("order.cityTooLong", "City length cannot exceed %d characters", max).
		WithParams(map[string]interface{}{"max": max})
}

func DistrictTooLong(max int) *bizerr.Error {
	return bizerr.Newf("order.districtTooLong", "District length cannot exceed %d characters", max).
		WithParams(map[string]interface{}{"max": max})
}

func AddressLengthInvalid(min, max int) *bizerr.Error {
	return bizerr.Newf("order.addressLengthInvalid", "Detailed address length must be between %d-%d characters", min, max).
		WithParams(map[string]interface{}{"min": min, "max": max})
}

func PostcodeInvalid() *bizerr.Error {
	return bizerr.New("order.postcodeInvalid", "Invalid postal code format or length")
}

func ResubmitReasonLengthInvalid(min, max int) *bizerr.Error {
	return bizerr.Newf("order.resubmitReasonLengthInvalid", "Resubmit reason length must be between %d-%d characters", min, max).
		WithParams(map[string]interface{}{"min": min, "max": max})
}

func ResubmitStatusInvalid(status models.OrderStatus) *bizerr.Error {
	return bizerr.Newf("order.resubmitStatusInvalid", "Only orders in pending status can request resubmission (current status: %s)", status).
		WithParams(map[string]interface{}{"status": status})
}

func UpdatePriceStatusInvalid(status models.OrderStatus) *bizerr.Error {
	return bizerr.Newf("order.updatePriceStatusInvalid", "Only pending payment orders can have price modified (current status: %s)", status).
		WithParams(map[string]interface{}{"status": status})
}

func BatchLimitExceeded(max int) *bizerr.Error {
	return bizerr.Newf("order.batchLimitExceeded", "Cannot process more than %d orders at once", max).
		WithParams(map[string]interface{}{"max": max})
}

func ItemsEmpty() *bizerr.Error {
	return bizerr.New("order.itemsEmpty", "Order items cannot be empty")
}

func TotalAmountNegative() *bizerr.Error {
	return bizerr.New("order.totalAmountNegative", "Total amount cannot be negative")
}

func ExternalUserIDLengthInvalid(min, max int) *bizerr.Error {
	return bizerr.Newf("order.externalUserIDLengthInvalid", "External user ID length must be between %d-%d characters", min, max).
		WithParams(map[string]interface{}{"min": min, "max": max})
}

func UsernameTooLong(max int) *bizerr.Error {
	return bizerr.Newf("order.usernameTooLong", "Username length cannot exceed %d characters", max).
		WithParams(map[string]interface{}{"max": max})
}

func ExternalOrderIDTooLong(max int) *bizerr.Error {
	return bizerr.Newf("order.externalOrderIDTooLong", "External order ID length cannot exceed %d characters", max).
		WithParams(map[string]interface{}{"max": max})
}

func PlatformNameTooLong(max int) *bizerr.Error {
	return bizerr.Newf("order.platformNameTooLong", "Platform name length cannot exceed %d characters", max).
		WithParams(map[string]interface{}{"max": max})
}

func OrderRemarkTooLong(max int) *bizerr.Error {
	return bizerr.Newf("order.orderRemarkTooLong", "Order remark length cannot exceed %d characters", max).
		WithParams(map[string]interface{}{"max": max})
}
