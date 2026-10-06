package main

import (
	"sync"
	"time"

	"github.com/dronm/elkonexport/config"
	"github.com/dronm/elkonexport/logger"
)

const responsePrefix = "ElkonProduction."

type Nil *struct{}

type ElkonProductionList []ElkonProduction //[]map[string]any

type ElkonPayloadType interface {
	int | string | LogTail | config.UnclosedProductionList | ElkonProductionList | Nil | config.Config
}

type ElkonPayload[T ElkonPayloadType] struct {
	ProductionSiteID int `json:"production_site_id"`
	Msg              T   `json:"msg"`
}

type ClientMessage[T ElkonPayloadType] struct {
	Func    string          `json:"f"`
	QueryID string          `json:"q"`
	Payload ElkonPayload[T] `json:"p"`
}

const elkonStructDBTag = "json"

type ElkonProduction struct {
	ID                int        `json:"id"`
	ProductionDtStart *time.Time `json:"production_dt_start"`
	ProductionDtEnd   *time.Time `json:"production_dt_end"`
	ConcreteQuant     float64    `json:"concrete_quant"`
	ConcreteDescr     string     `json:"concrete_descr"`
	UserDescr         string     `json:"user_descr"`
	VehicleDescr      string     `json:"vehicle_descr"`
	CorrectionID      *int       `json:"correction_id"`
	CorrectionDtEnd   *time.Time `json:"correction_dt_end"`

	Mat1Descr          *string  `json:"mat1_descr"`
	Mat1Quant          float64  `json:"mat1_quant"`
	Mat1QuantReq       float64  `json:"mat1_quant_req"`
	Mat1Cement         bool     `json:"mat1_cement"`
	Mat1QuantCorrected *float64 `json:"mat1_quant_corrected"`

	Mat2Descr          *string  `json:"mat2_descr"`
	Mat2Quant          float64  `json:"mat2_quant"`
	Mat2QuantReq       float64  `json:"mat2_quant_req"`
	Mat2Cement         bool     `json:"mat2_cement"`
	Mat2QuantCorrected *float64 `json:"mat2_quant_corrected"`

	Mat3Descr          *string  `json:"mat3_descr"`
	Mat3Quant          float64  `json:"mat3_quant"`
	Mat3QuantReq       float64  `json:"mat3_quant_req"`
	Mat3Cement         bool     `json:"mat3_cement"`
	Mat3QuantCorrected *float64 `json:"mat3_quant_corrected"`

	Mat4Descr          *string  `json:"mat4_descr"`
	Mat4Quant          float64  `json:"mat4_quant"`
	Mat4QuantReq       float64  `json:"mat4_quant_req"`
	Mat4Cement         bool     `json:"mat4_cement"`
	Mat4QuantCorrected *float64 `json:"mat4_quant_corrected"`

	Mat5Descr          *string  `json:"mat5_descr"`
	Mat5Quant          float64  `json:"mat5_quant"`
	Mat5QuantReq       float64  `json:"mat5_quant_req"`
	Mat5Cement         bool     `json:"mat5_cement"`
	Mat5QuantCorrected *float64 `json:"mat5_quant_corrected"`

	Mat6Descr          *string  `json:"mat6_descr"`
	Mat6Quant          float64  `json:"mat6_quant"`
	Mat6QuantReq       float64  `json:"mat6_quant_req"`
	Mat6Cement         bool     `json:"mat6_cement"`
	Mat6QuantCorrected *float64 `json:"mat6_quant_corrected"`

	Mat7Descr          *string  `json:"mat7_descr"`
	Mat7Quant          float64  `json:"mat7_quant"`
	Mat7QuantReq       float64  `json:"mat7_quant_req"`
	Mat7Cement         bool     `json:"mat7_cement"`
	Mat7QuantCorrected *float64 `json:"mat7_quant_corrected"`

	Mat8Descr          *string  `json:"mat8_descr"`
	Mat8Quant          float64  `json:"mat8_quant"`
	Mat8QuantReq       float64  `json:"mat8_quant_req"`
	Mat8Cement         bool     `json:"mat8_cement"`
	Mat8QuantCorrected *float64 `json:"mat8_quant_corrected"`

	Mat9Descr          *string  `json:"mat9_descr"`
	Mat9Quant          float64  `json:"mat9_quant"`
	Mat9QuantReq       float64  `json:"mat9_quant_req"`
	Mat9Cement         bool     `json:"mat9_cement"`
	Mat9QuantCorrected *float64 `json:"mat9_quant_corrected"`

	Mat10Descr          *string  `json:"mat10_descr"`
	Mat10Quant          float64  `json:"mat10_quant"`
	Mat10QuantReq       float64  `json:"mat10_quant_req"`
	Mat10Cement         bool     `json:"mat10_cement"`
	Mat10QuantCorrected *float64 `json:"mat10_quant_corrected"`

	Mat11Descr          *string  `json:"mat11_descr"`
	Mat11Quant          float64  `json:"mat11_quant"`
	Mat11QuantReq       float64  `json:"mat11_quant_req"`
	Mat11Cement         bool     `json:"mat11_cement"`
	Mat11QuantCorrected *float64 `json:"mat11_quant_corrected"`

	Mat12Descr          *string  `json:"mat12_descr"`
	Mat12Quant          float64  `json:"mat12_quant"`
	Mat12QuantReq       float64  `json:"mat12_quant_req"`
	Mat12Cement         bool     `json:"mat12_cement"`
	Mat12QuantCorrected *float64 `json:"mat12_quant_corrected"`

	Mat13Descr          *string  `json:"mat13_descr"`
	Mat13Quant          float64  `json:"mat13_quant"`
	Mat13QuantReq       float64  `json:"mat13_quant_req"`
	Mat13Cement         bool     `json:"mat13_cement"`
	Mat13QuantCorrected *float64 `json:"mat13_quant_corrected"`

	Mat14Descr          *string  `json:"mat14_descr"`
	Mat14Quant          float64  `json:"mat14_quant"`
	Mat14QuantReq       float64  `json:"mat14_quant_req"`
	Mat14Cement         bool     `json:"mat14_cement"`
	Mat14QuantCorrected *float64 `json:"mat14_quant_corrected"`

	Mat15Descr          *string  `json:"mat15_descr"`
	Mat15Quant          float64  `json:"mat15_quant"`
	Mat15QuantReq       float64  `json:"mat15_quant_req"`
	Mat15Cement         bool     `json:"mat15_cement"`
	Mat15QuantCorrected *float64 `json:"mat15_quant_corrected"`

	Mat16Descr          *string  `json:"mat16_descr"`
	Mat16Quant          float64  `json:"mat16_quant"`
	Mat16QuantReq       float64  `json:"mat16_quant_req"`
	Mat16Cement         bool     `json:"mat16_cement"`
	Mat16QuantCorrected *float64 `json:"mat16_quant_corrected"`
}

func sendPayload[T ElkonPayloadType](a *App, meth string, payload T, queryID string) {
	a.sendResultCh <- ClientMessage[T]{
		Func:    responsePrefix + meth,
		Payload: ElkonPayload[T]{ProductionSiteID: a.productionSiteID, Msg: payload},
		QueryID: queryID,
	}
}

type LastError struct {
	mx        sync.Mutex
	errorMsg  string
	errorTime time.Time
}

var lastError LastError

func sendError(a *App, err error, queryID string) {
	lastError.mx.Lock()
	defer lastError.mx.Unlock()

	msg := err.Error()

	if lastError.errorMsg == msg && time.Since(lastError.errorTime) < a.cfgMgr.ProductionErrorDebounce() {
		logger.Logger.Debugf("sendError debounced error: %s", msg)
		return
	}
	lastError.errorMsg = msg
	lastError.errorTime = time.Now()

	a.sendResultCh <- ClientMessage[string]{
		Func:    responsePrefix + "Error",
		Payload: ElkonPayload[string]{ProductionSiteID: a.productionSiteID, Msg: msg},
		QueryID: queryID,
	}
}
