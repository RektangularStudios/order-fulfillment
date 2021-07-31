package now_payments

import (
	"fmt"
	"context"
	"net/http"
	"encoding/json"
	"net/url"
	"bytes"
	"io/ioutil"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"

	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/config"
)

const (
	baseURL = "https://api.nowpayments.io/v1/"
	sandboxBaseURL = "https://api.sandbox.nowpayments.io/v1/"
	ipnCallbackURL = "https://api.rektangularstudios.com/order-fulfillment/ipn"
)

type ServiceImpl struct {
	apiKey string
	ipnSecretKey string
	isSandbox bool
	ipnCallbackURL string
}

// creates a new ServiceImpl
func New(apiKey string, ipnSecretKey string, isSandbox bool) (*ServiceImpl, error) {
	config, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get config from env: %v", err)
	}

	return &ServiceImpl {
		apiKey: apiKey,
		ipnSecretKey: ipnSecretKey,
		isSandbox: isSandbox,
		ipnCallbackURL: config.NowPayments.IPNCallbackURL,
	}, nil
}

type GetStatusResponse struct {
	Message string `json:"message"`
}

type CreatePaymentRequest struct {
	PriceAmount float64 `json:"price_amount"`
	PriceCurrency string `json:"price_currency"`
	PayCurrency string `json:"pay_currency"`
	IPNCallbackURL string `json:"ipn_callback_url"`
	OrderID string `json:"order_id"`
	OrderDescription string `json:"order_description"`
	// Sandbox case
	Case string `json:"case,omitempty"`
}

type CreatePaymentResponse struct {
	PaymentID string `json:"payment_id"`
	PaymentStatus string `json:"payment_status"`
	PayAddress string `json:"pay_address"`
	PriceAmount int `json:"price_amount"`
	PriceCurrency string `json:"price_currency"`
	PayAmount string `json:"pay_amount"`
	PayCurrency string `json:"pay_currency"`
	OrderID string `json:"order_id"`
	OrderDescription string `json:"order_description"`
	PayInExtraID string `json:"payin_extra_id"`
	IPNCallbackURL string `json:"ipn_callback_url"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	PurchaseID string `json:"purchase_id"`
}

type GetPaymentStatusResponse struct {
	PaymentID json.Number `json:"payment_id"`
	PaymentStatus string `json:"payment_status"`
	PayAddress string `json:"pay_address"`
	PriceAmount int `json:"price_amount"`
	PriceCurrency string `json:"price_currency"`
	PayAmount float64 `json:"pay_amount"`
	ActuallyPaid float64 `json:"actually_paid"`
	PayCurrency string `json:"pay_currency"`
	OrderID string `json:"order_id"`
	OrderDescription string `json:"order_description"`
	PurchaseID string `json:"purchase_id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	OutcomeAmount float64 `json:"outcome_amount"`
	OutcomeCurrency string `json:"outcome_currency"`
	Case string `json:"case,omitempty"`
}

type GetPaymentStatusResponseStringOnly struct {
	PaymentID json.Number `json:"payment_id"`
	PaymentStatus string `json:"payment_status"`
	PayAddress string `json:"pay_address"`
	PriceAmount json.Number `json:"price_amount"`
	PriceCurrency string `json:"price_currency"`
	PayAmount json.Number `json:"pay_amount"`
	ActuallyPaid json.Number `json:"actually_paid"`
	PayCurrency string `json:"pay_currency"`
	OrderID string `json:"order_id"`
	OrderDescription string `json:"order_description"`
	PurchaseID string `json:"purchase_id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	OutcomeAmount json.Number `json:"outcome_amount"`
	OutcomeCurrency string `json:"outcome_currency"`
	Case string `json:"case,omitempty"`
}

func (s *ServiceImpl) fromBaseURL(route string) (*url.URL, error) {
	u, err := url.Parse(route)
	if err != nil {
		return nil, err
	}

	var base *url.URL
	if s.isSandbox {
		base, err = url.Parse(sandboxBaseURL)
	} else {
		base, err= url.Parse(baseURL)
	}
	if err != nil {
		return nil, err
	}

	return base.ResolveReference(u), nil
}

func (s *ServiceImpl) Status(ctx context.Context) (string, error) {
	// "/status"
	u, err := s.fromBaseURL("status")
	if err != nil {
		return "", err
	}

	resp, err := http.Get(u.String())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var respBody GetStatusResponse
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(bodyBytes, &respBody)
	if err != nil {
		return "", err
	}

	return respBody.Message, nil
}

func (s *ServiceImpl) CreatePayment(ctx context.Context, createPaymentRequest CreatePaymentRequest) (*CreatePaymentResponse, error) {
	config, err := config.GetConfig()
	if err != nil {
		return nil, err
	}
	// automagically set "success" for Sandbox testint
	if config.NowPayments.IsSandbox {
		fmt.Printf("Creating payment with success case (sandbox)\n")
		createPaymentRequest.Case = "success"
	}

	// set IPN callback URL
	if len(createPaymentRequest.IPNCallbackURL) == 0 {
		createPaymentRequest.IPNCallbackURL = s.ipnCallbackURL
	}

	// "/payment"
	u, err := s.fromBaseURL("payment")
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(createPaymentRequest)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", u.String(), bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	fmt.Printf("request: %+v", req)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		return nil, fmt.Errorf("create payment failed: %+v", resp)
	}

	var respBody CreatePaymentResponse
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(bodyBytes, &respBody)
	if err != nil {
		return nil, err
	}

	return &respBody, nil
}

func (s *ServiceImpl) GetPaymentStatus(ctx context.Context, paymentID string) (*GetPaymentStatusResponse, error) {
	// "/payment/<your_payment_id>"
	u, err := s.fromBaseURL(fmt.Sprintf("payment/%s", paymentID))
	if err != nil {
		return nil,err
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", s.apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("get payment status failed: %+v", resp)
	}

	var respBody GetPaymentStatusResponse
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(bodyBytes, &respBody)
	if err != nil {
		return nil, err
	}

	return &respBody, nil
}

func jsonRemarshal(bytes []byte) ([]byte, error) {
	// yes, this function is as dumb as it looks. it does two things:
	// - handles Golangs broken type conversions for JSON
	// - makes sure the fields are sorted alphabetically

	var r GetPaymentStatusResponse
	err := json.Unmarshal(bytes, &r)
	if err != nil {
			return []byte{}, err
	}
	//fmt.Printf("\n\nresponse: %+v", r)
	rStringOnly := GetPaymentStatusResponseStringOnly {
		PaymentID: r.PaymentID,
		PaymentStatus: fmt.Sprintf("%v", r.PaymentStatus),
		PayAddress: fmt.Sprintf("%v", r.PayAddress),
		PriceAmount: json.Number(fmt.Sprintf("%v", r.PriceAmount)),
		PriceCurrency: fmt.Sprintf("%v", r.PriceCurrency),
		PayAmount: json.Number(fmt.Sprintf("%v", r.PayAmount)),
		ActuallyPaid: json.Number(fmt.Sprintf("%v", r.ActuallyPaid)),
		PayCurrency: fmt.Sprintf("%v", r.PayCurrency),
		OrderID: fmt.Sprintf("%v", r.OrderID),
		OrderDescription: fmt.Sprintf("%v", r.OrderDescription),
		PurchaseID: fmt.Sprintf("%v", r.PurchaseID),
		CreatedAt: fmt.Sprintf("%v", r.CreatedAt),
		UpdatedAt: fmt.Sprintf("%v", r.UpdatedAt),
		OutcomeAmount: json.Number(fmt.Sprintf("%v", r.OutcomeAmount)),
		OutcomeCurrency: fmt.Sprintf("%v", r.OutcomeCurrency),
		Case: fmt.Sprintf("%v", r.Case),
	}
	rStringOnlyBytes, err := json.Marshal(rStringOnly)
	if err != nil {
			return []byte{}, err
	}
	//fmt.Printf("\n\nstring only bytes: %s", string(rStringOnlyBytes))
	var iface interface{}
	err = json.Unmarshal(rStringOnlyBytes, &iface)
	if err != nil {
			return []byte{}, err
	}
	//fmt.Printf("\n\niface: %+v", iface)
	// abuse fact that marshal for interface{} will automatically sort fields
	output, err := json.Marshal(iface)
	if err != nil {
			return []byte{}, err
	}
	//fmt.Printf("\n\noutput: %s", string(output))
	return output, nil
}


func (s *ServiceImpl) IPNWebhookValidate(r *http.Request) (*GetPaymentStatusResponse, error) {
	// remarshal callback JSON to be sorted alphabetically
	//fmt.Printf("\n\nrequest: %+v,", r)
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	//fmt.Printf("Received webhook: %+v, %s", r, string(bodyBytes))

	//fmt.Printf("\n\nincoming body bytes: %s", string(bodyBytes))
	sortedJSON, err := jsonRemarshal(bodyBytes)
	if err != nil {
		return nil, err
	}

	// hash body
	h := hmac.New(sha512.New, []byte(s.ipnSecretKey))
	h.Write(sortedJSON)
	sha := hex.EncodeToString(h.Sum(nil))

	// verify signature
	sigValues := r.Header.Values("X-Nowpayments-Sig")
	if len(sigValues) == 0 {
		return nil, fmt.Errorf("IPN callback missing signature header")
	}
	sig := sigValues[0]
	//fmt.Printf("\n\n sig(%s) =?= sha(%s)", sig, sha)

	if sig != sha {
		return nil, fmt.Errorf("IPN callback signature did not match")
	}

	// return properly typed struct
	var bodyStruct GetPaymentStatusResponse
	err = json.Unmarshal(bodyBytes, &bodyStruct)
	if err != nil {
		return nil, err
	}

	return &bodyStruct, nil
}
