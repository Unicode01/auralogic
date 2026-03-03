package service

import (
	"net/http"
	"sync"
)

var (
	paymentHTTPClientOnce sync.Once
	paymentHTTPClient     *http.Client
)

func getPaymentHTTPClient() *http.Client {
	paymentHTTPClientOnce.Do(func() {
		paymentHTTPClient = newPaymentHTTPClient()
	})
	return paymentHTTPClient
}
