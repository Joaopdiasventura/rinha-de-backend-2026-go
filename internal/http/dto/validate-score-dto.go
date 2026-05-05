package dto

import "time"

type ValidateTransactionDTO struct {
	ID              string              `json:"id"`
	Transaction     TransactionDTO      `json:"transaction"`
	Customer        CustomerDTO         `json:"customer"`
	Merchant        MerchantDTO         `json:"merchant"`
	Terminal        TerminalDTO         `json:"terminal"`
	LastTransaction *LastTransactionDTO `json:"last_transaction"`
}

type TransactionDTO struct {
	Amount       float64   `json:"amount"`
	Installments float64   `json:"installments"`
	RequestedAt  time.Time `json:"requested_at"`
}

type CustomerDTO struct {
	AvgAmount      float64  `json:"avg_amount"`
	TxCount24h     int      `json:"tx_count_24h"`
	KnownMerchants []string `json:"known_merchants"`
}

type MerchantDTO struct {
	ID        string  `json:"id"`
	MCC       string  `json:"mcc"`
	AvgAmount float64 `json:"avg_amount"`
}

type TerminalDTO struct {
	IsOnline    bool    `json:"is_online"`
	CardPresent bool    `json:"card_present"`
	KmFromHome  float64 `json:"km_from_home"`
}

type LastTransactionDTO struct {
	Timestamp     time.Time `json:"timestamp"`
	KmFromCurrent float64   `json:"km_from_current"`
}
